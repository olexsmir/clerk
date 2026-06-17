package parser

import (
	"fmt"
	"strconv"
	"strings"

	"olexsmir.xyz/clerk/internal/decimal"
	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/lexer"
	"olexsmir.xyz/clerk/journal/token"
)

type Parser struct {
	lexer  *lexer.Lexer
	errors []*ast.ParseError
	cur    token.Token
	peek   token.Token
}

func New(lex *lexer.Lexer) *Parser {
	p := &Parser{lexer: lex}
	p.advance() // populate .peek
	p.advance() // populate .cur
	return p
}

func (p *Parser) ParseJournal() *ast.Journal {
	f := &ast.Journal{}
	for p.cur.Type != token.EOF {
		if e := p.parseEntry(); e != nil {
			f.Entries = append(f.Entries, e)
		}
	}
	f.Errors = p.errors
	return f
}

func isDirectiveKeyword(t token.Type) bool {
	switch t {
	case token.COMMENTKW, token.ACCOUNT, token.COMMODITY, token.INCLUDE,
		token.ALIAS, token.PAYEE, token.TAG, token.APPLY, token.END,
		token.YEAR, token.DECIMALMARK, token.D, token.P, token.N, token.C:
		return true
	}
	return false
}

func (p *Parser) parseEntry() ast.Entry {
	if p.got(token.BANG) || p.got(token.AT) {
		if isDirectiveKeyword(p.peek.Type) {
			p.advance() // consume prefix
		}
	}
	switch p.cur.Type {
	case token.ILLEGAL:
		p.errorf("illegal character %q", p.cur.Literal)
		p.advance()
		return nil
	case token.INDENT:
		p.errorf("unexpected indent")
		p.syncToNextline()
		return nil
	case token.DATE:
		return p.parseTransaction()
	case token.TILDE:
		return p.parsePeriodicTransaction()
	case token.EQ:
		return p.parseAutomatedTransaction()
	case token.NEWLINE:
		return p.parseBlankLine()
	case token.SEMICOLON, token.HASH, token.PERCENT, token.STAR:
		return p.parseComment()
	case token.ACCOUNT:
		return p.parseAccountDirective()
	case token.COMMODITY:
		return p.parseCommodityDirective()
	case token.INCLUDE:
		return p.parseIncludeDirective()
	case token.ALIAS:
		return p.parseAliasDirective()
	case token.PAYEE:
		return p.parsePayeeDirective()
	case token.TAG:
		return p.parseTagDirective()
	case token.YEAR:
		return p.parseYearDirective()
	case token.DECIMALMARK:
		return p.parseDecimalMarkDirective()
	case token.D:
		return p.parseDefaultCommodityDirective()
	case token.P:
		return p.parseMarketPriceDirective()
	case token.N:
		return p.parseIgnoredDirective()
	case token.C:
		return p.parseConversionDirective()
	case token.APPLY:
		return p.parseApplyDirective()
	case token.END:
		return p.parseEndDirective()
	case token.COMMENTKW:
		return p.parseCommentBlockDirective()
	default:
		p.errorf("unexpected token %s", p.cur.Type)
		p.sync()
		return nil
	}
}

func (p *Parser) parseTransaction() *ast.Transaction {
	s := p.cur.Span
	tx := &ast.Transaction{}

	tx.Date = p.parseDate()

	p.skipWhitespace()

	// optional secondary date
	if p.got(token.EQ) {
		p.advance()
		p.skipWhitespace()
		d := p.parseDate()
		tx.SecondDate = &d
	}

	p.skipWhitespace()

	// optional status
	tx.Status = p.parseStatus()

	// optional code
	if p.got(token.LPAREN) {
		p.advance()
		var code strings.Builder
		for p.cur.Type != token.RPAREN {
			_, _ = code.WriteString(p.cur.Literal)
			p.advance()
		}
		tx.Code = new(code.String())
		p.advance() // TODO: why?
		p.skipWhitespace()
	}

	// optional payee | note
	if p.got(token.TEXT) || p.got(token.STRING) {
		tx.Payee = p.parsePayee()

		// check for | separator
		if p.got(token.WHITESPACE) {
			p.skipWhitespace()
		}

		if p.got(token.PIPE) {
			p.advance()
			if p.got(token.TEXT) {
				n := p.cur.Literal
				p.advance()
				tx.Note = &n
			}
		}
	}

	tx.Comment = p.parseOptInlineComment()
	p.expectNewline()

	// header comments — indented ; lines before first posting
	for p.got(token.INDENT) && p.willGet(token.SEMICOLON) {
		p.advance() // consume indent
		c := p.parseComment()
		tx.HeaderComments = append(tx.HeaderComments, c)
	}

	// postings
	for p.got(token.INDENT) {
		if p := p.parsePosting(); p != nil {
			tx.Postings = append(tx.Postings, p)
		}
	}

	tx.Span = p.span(s)
	return tx
}

func unquote(s string) string {
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

func (p *Parser) parsePayee() *ast.Payee {
	s := p.cur.Span

	if p.got(token.STRING) {
		name := unquote(p.cur.Literal)
		p.advance()
		return &ast.Payee{Name: name, Span: p.span(s)}
	}

	// keep spaces/tags between text tokens; stop before trailing whitespace
	var name strings.Builder
	for p.got(token.TEXT) || p.got(token.INT) || p.got(token.DECIMAL) || (p.got(token.WHITESPACE) && (p.willGet(token.TEXT) || p.willGet(token.INT) || p.willGet(token.DECIMAL))) {
		_, _ = name.WriteString(p.cur.Literal)
		p.advance()
	}
	return &ast.Payee{Name: unquote(name.String()), Span: p.span(s)}
}

func (p *Parser) parsePeriodicTransaction() *ast.PeriodicTransaction {
	s := p.cur.Span
	p.expect(token.TILDE)
	p.skipWhitespace()

	pt := &ast.PeriodicTransaction{}

	pt.Span = p.span(s)
	pt.Period = p.parsePeriod()

	if desc := p.parseOptPeriodicDescription(); desc != "" {
		pt.Description = &desc
	}

	comment := p.parseOptInlineComment()
	p.expectNewline()

	var headerComments []*ast.Comment
	var postings []*ast.Posting
	for p.got(token.INDENT) || p.got(token.SEMICOLON) {
		if p.got(token.SEMICOLON) {
			c := p.parseComment()
			headerComments = append(headerComments, c)
			continue
		}
		posting := p.parsePosting()
		if posting != nil {
			postings = append(postings, posting)
		}
	}

	pt.HeaderComments = headerComments
	pt.Postings = postings
	pt.Comment = comment
	return pt
}

func (p *Parser) parseAutomatedTransaction() *ast.AutomatedTransaction {
	s := p.cur.Span
	p.expect(token.EQ)
	p.skipWhitespace()

	at := &ast.AutomatedTransaction{}
	at.Span = p.span(s)

	at.Expr = p.parseDirectiveExpr()
	at.Comment = p.parseOptInlineComment()
	p.expectNewline()

	// header comments
	for p.got(token.INDENT) && p.willGet(token.SEMICOLON) {
		p.advance()
		at.HeaderComments = append(at.HeaderComments, p.parseComment())
	}

	// postings
	for p.got(token.INDENT) {
		if p := p.parsePosting(); p != nil {
			at.Postings = append(at.Postings, p)
		}
	}

	return at
}

func (p *Parser) parsePeriod() ast.Period {
	s := p.cur.Span

	var periodBuf strings.Builder

	for !p.got(token.NEWLINE) && !p.got(token.EOF) &&
		!p.got(token.SEMICOLON) && !p.got(token.HASH) && !p.got(token.PERCENT) && !p.got(token.STAR) {

		if p.got(token.WHITESPACE) {
			if len(p.cur.Literal) >= 2 {
				break
			}
			if p.willGet(token.NEWLINE) || p.willGet(token.EOF) ||
				p.willGet(token.SEMICOLON) || p.willGet(token.HASH) ||
				p.willGet(token.PERCENT) || p.willGet(token.STAR) {
				p.advance()
				continue
			}
		}

		periodBuf.WriteString(p.cur.Literal)
		p.advance()
	}

	str := periodBuf.String()
	period := ast.Period{Raw: str, Span: p.span(s)}

	if _, after, ok := strings.Cut(str, " from "); ok {
		end := strings.Index(after, " ")
		dateStr := after
		if end >= 0 {
			dateStr = after[:end]
		}
		if d := parseSimpleDate(dateStr); d.Year > 0 {
			period.From = &d
			rest := after
			if end >= 0 {
				rest = after[end:]
			}
			if _, toAfter, ok := strings.Cut(rest, " to "); ok {
				if toEnd := strings.Index(toAfter, " "); toEnd >= 0 {
					toAfter = toAfter[:toEnd]
				}
				if d := parseSimpleDate(toAfter); d.Year > 0 {
					period.To = &d
				}
			}
		}
	}
	return period
}

func (p *Parser) parseComment() *ast.Comment {
	s := p.cur.Span
	marker := p.cur.Literal[0]
	p.advance()
	p.skipWhitespace()

	var text string
	if p.got(token.TEXT) {
		text = p.cur.Literal
		p.advance()
	}

	p.expectNewline()

	return &ast.Comment{
		Marker: marker,
		Text:   text,
		Span:   p.span(s),
	}
}

func (p *Parser) parseAccountDirective() *ast.AccountDirective {
	s := p.cur.Span
	p.expect(token.ACCOUNT)
	p.skipWhitespace()

	account := p.parseAccount()
	comment := p.parseOptInlineComment()
	p.expectNewline()

	for p.got(token.INDENT) {
		p.advance()
		for !p.got(token.NEWLINE) && !p.got(token.EOF) {
			p.advance()
		}
		p.expectNewline()
	}

	return &ast.AccountDirective{
		Account: account,
		Comment: comment,
		Span:    p.span(s),
	}
}

func (p *Parser) parseCommodityDirective() *ast.CommodityDirective {
	s := p.cur.Span
	p.expect(token.COMMODITY)
	p.skipWhitespace()

	var commodity string
	var format *ast.Amount

	switch p.cur.Type {
	case token.COMMODITYMARK, token.TEXT, token.STRING:
		commodity = p.cur.Literal
		if p.got(token.STRING) {
			commodity = unquote(commodity)
		}
		p.advance()
		hadSpace := p.got(token.WHITESPACE)
		p.skipWhitespace()
		if p.got(token.INT) || p.got(token.DECIMAL) || p.got(token.TEXT) {
			format = p.parseAmount()
			format.Commodity = commodity
			format.CommodityPos = ast.CommodityBefore
			format.HasSpace = hadSpace
		}
	case token.INT, token.DECIMAL:
		format = p.parseAmount()
		commodity = format.Commodity
	default:
		p.errorf("expected commodity name or amount, got %s", p.cur.Type)
	}

	if commodity == "" {
		p.errorf("expected commodity name, got %s", p.cur.Type)
	}

	comment := p.parseOptInlineComment()
	p.expectNewline()

	for p.got(token.INDENT) {
		p.advance()
		p.skipWhitespace()
		if p.got(token.COMMODITYMARK) && p.cur.Literal == "format" {
			p.advance()
			p.skipWhitespace()
			format = p.parseAmount()
		} else {
			for !p.got(token.NEWLINE) && !p.got(token.EOF) {
				p.advance()
			}
		}
		p.expectNewline()
	}

	cd := &ast.CommodityDirective{
		Commodity: commodity,
		Comment:   comment,
		Span:      p.span(s),
	}
	if format != nil {
		cd.Format = *format
	}
	return cd
}

func (p *Parser) parseIncludeDirective() *ast.IncludeDirective {
	s := p.cur.Span
	p.expect(token.INCLUDE)
	p.skipWhitespace()

	id := &ast.IncludeDirective{}

	if p.got(token.TEXT) {
		id.Path = p.cur.Literal
		p.advance()
	} else {
		p.errorf("expected file path, got %s", p.cur.Type)
	}

	p.skipWhitespace()
	id.Comment = p.parseOptInlineComment()
	p.expectNewline()
	id.Span = p.span(s)
	return id
}

func (p *Parser) parseAliasDirective() *ast.AliasDirective {
	s := p.cur.Span
	alias := &ast.AliasDirective{}
	p.expect(token.ALIAS)
	p.skipWhitespace()
	alias.From = p.parseAccount()
	p.skipWhitespace()
	p.expect(token.EQ)
	p.skipWhitespace()
	alias.To = p.parseAccount()
	p.skipWhitespace()
	alias.Comment = p.parseOptInlineComment()
	p.expectNewline()
	alias.Span = p.span(s)
	return alias
}

func (p *Parser) parsePayeeDirective() *ast.PayeeDirective {
	s := p.cur.Span
	p.expect(token.PAYEE)
	p.skipWhitespace()

	name := ""
	if p.got(token.TEXT) || p.got(token.STRING) {
		name = p.parsePayee().Name
	}

	comment := p.parseOptInlineComment()
	p.expectNewline()

	return &ast.PayeeDirective{
		Name:    name,
		Comment: comment,
		Span:    p.span(s),
	}
}

func (p *Parser) parseTagDirective() *ast.TagDirective {
	s := p.cur.Span
	p.expect(token.TAG)
	p.skipWhitespace()

	name := ""
	if p.got(token.TEXT) {
		name = p.cur.Literal
		p.advance()
	}

	comment := p.parseOptInlineComment()
	p.expectNewline()

	return &ast.TagDirective{
		Name:    name,
		Comment: comment,
		Span:    p.span(s),
	}
}

func (p *Parser) parseYearDirective() *ast.YearDirective {
	s := p.cur.Span
	year := &ast.YearDirective{}
	p.expect(token.YEAR)
	p.skipWhitespace()

	if p.got(token.INT) {
		year.Year, _ = strconv.Atoi(p.cur.Literal)
		p.advance()
	} else {
		p.errorf("expected year, got %s", p.cur.Type)
	}

	p.skipWhitespace()
	year.Comment = p.parseOptInlineComment()
	p.expectNewline()
	year.Span = p.span(s)
	return year
}

func (p *Parser) parseDecimalMarkDirective() *ast.DecimalMarkDirective {
	s := p.cur.Span
	mark := &ast.DecimalMarkDirective{}
	p.expect(token.DECIMALMARK)
	p.skipWhitespace()

	mark.Mark = byte('.')
	if p.got(token.TEXT) {
		if len(p.cur.Literal) > 0 {
			mark.Mark = p.cur.Literal[0]
		}
		p.advance()
	}

	p.skipWhitespace()
	mark.Comment = p.parseOptInlineComment()
	p.expectNewline()
	mark.Span = p.span(s)
	return mark
}

func (p *Parser) parseDefaultCommodityDirective() *ast.DefaultCommodityDirective {
	s := p.cur.Span
	com := &ast.DefaultCommodityDirective{}
	p.expect(token.D)
	p.skipWhitespace()
	com.Amount = *p.parseAmount()
	p.skipWhitespace()
	com.Comment = p.parseOptInlineComment()
	p.expectNewline()
	com.Span = p.span(s)
	return com
}

func (p *Parser) parseConversionDirective() *ast.ConversionDirective {
	s := p.cur.Span
	cd := &ast.ConversionDirective{}
	p.expect(token.C)
	p.skipWhitespace()

	if p.isAmountStart() {
		cd.From = *p.parseAmount()
	} else {
		p.errorf("expected amount, got %s", p.cur.Type)
	}

	p.skipWhitespace()
	if p.got(token.EQ) {
		p.advance()
		p.skipWhitespace()
		if p.isAmountStart() {
			cd.To = *p.parseAmount()
		} else {
			p.errorf("expected amount, got %s", p.cur.Type)
		}
	}

	p.skipWhitespace()
	cd.Comment = p.parseOptInlineComment()
	p.expectNewline()
	cd.Span = p.span(s)
	return cd
}

func (p *Parser) parseIgnoredDirective() *ast.IgnoredDirective {
	s := p.cur.Span
	p.expect(token.N)
	p.skipWhitespace()

	id := &ast.IgnoredDirective{}
	if p.got(token.TEXT) || p.got(token.COMMODITYMARK) {
		id.Text = p.cur.Literal
		p.advance()
	}
	p.skipWhitespace()
	id.Comment = p.parseOptInlineComment()

	p.expectNewline()
	id.Span = p.span(s)
	return id
}

func (p *Parser) parseMarketPriceDirective() *ast.MarketPriceDirective {
	s := p.cur.Span
	p.expect(token.P)
	p.skipWhitespace()

	mp := &ast.MarketPriceDirective{}
	mp.DateTime.Date = p.parseDate()
	p.skipWhitespace()

	if p.got(token.TIME) {
		mp.DateTime.Time = new(p.parseTime())
		p.skipWhitespace()
	}

	tok, _ := p.expect(token.COMMODITYMARK)
	mp.Commodity = tok.Literal
	p.advance()

	mp.Amount = *p.parseAmount()

	p.skipWhitespace()
	mp.Comment = p.parseOptInlineComment()

	p.expectNewline()
	mp.Span = p.span(s)
	return mp
}

func (p *Parser) parseTime() ast.Time {
	s := p.cur.Span
	tok, _ := p.expect(token.TIME)
	lit := tok.Literal

	parts := strings.Split(lit, ":")
	if len(parts) < 2 {
		p.errorf("invalid time format: %q", lit)
		return ast.Time{Span: p.span(s)}
	}

	hour, _ := strconv.Atoi(parts[0])
	minute, _ := strconv.Atoi(parts[1])
	second := 0
	if len(parts) > 2 {
		second, _ = strconv.Atoi(parts[2])
	}

	if hour < 0 || hour > 23 {
		p.errorf("invalid hour %d in time %q", hour, lit)
	}
	if minute < 0 || minute > 59 {
		p.errorf("invalid minute %d in time %q", minute, lit)
	}
	if second < 0 || second > 59 {
		p.errorf("invalid second %d in time %q", second, lit)
	}

	return ast.Time{
		Hour:   hour,
		Minute: minute,
		Second: second,
		Span:   p.span(s),
	}
}

func (p *Parser) parseApplyDirective() *ast.ApplyDirective {
	s := p.cur.Span
	p.expect(token.APPLY)
	p.skipWhitespace()

	expr := p.parseDirectiveExpr()
	comment := p.parseOptInlineComment()
	p.expectNewline()

	return &ast.ApplyDirective{
		Expr:    expr,
		Comment: comment,
		Span:    p.span(s),
	}
}

func (p *Parser) parseEndDirective() *ast.EndDirective {
	s := p.cur.Span
	p.expect(token.END)
	p.skipWhitespace()

	expr := p.parseDirectiveExpr()
	comment := p.parseOptInlineComment()
	p.expectNewline()

	return &ast.EndDirective{
		Expr:    expr,
		Comment: comment,
		Span:    p.span(s),
	}
}

func (p *Parser) parseCommentBlockDirective() *ast.CommentBlockDirective {
	start := p.cur.Span
	p.expect(token.COMMENTKW)
	p.skipWhitespace()

	header := p.parseDirectiveExpr()
	comment := p.parseOptInlineComment()
	p.expectNewline()

	var content strings.Builder
	for p.cur.Type != token.EOF {
		if p.got(token.END) {
			if p.willGet(token.NEWLINE) || p.willGet(token.EOF) {
				p.advance()
				p.expectNewline()
				break
			}
			if p.willGet(token.WHITESPACE) {
				endTok := p.cur
				p.advance()
				wsTok := p.cur
				p.advance()
				if p.got(token.TEXT) && p.cur.Literal == "comment" { // todo: this should check if it's an actual COMMENTKW token
					p.advance()
					p.parseDirectiveExpr()
					p.parseOptInlineComment()
					p.expectNewline()
					break
				}
				content.WriteString(endTok.Literal)
				content.WriteString(wsTok.Literal)
				continue
			}
		}
		content.WriteString(p.cur.Literal)
		p.advance()
	}

	return &ast.CommentBlockDirective{
		Header:  header,
		Content: content.String(),
		Comment: comment,
		Span:    p.span(start),
	}
}

func (p *Parser) parseStatus() *ast.Status {
	if p.got(token.STAR) || p.got(token.BANG) {
		status := ast.StatusPending
		if p.cur.Literal[0] == '*' {
			status = ast.StatusCleared
		}
		st := &ast.Status{Value: status, Span: p.cur.Span}
		p.advance()
		p.skipWhitespace()
		return st
	}
	return nil
}

func (p *Parser) isAmountStart() bool {
	switch p.cur.Type {
	default:
		return false
	case token.COMMODITYMARK, token.STRING, token.INT, token.DECIMAL, token.MINUS, token.PLUS, token.PARENEXPR:
		return true
	case token.TEXT:
		return len(p.cur.Literal) > 0 && p.cur.Literal[0] >= '0' && p.cur.Literal[0] <= '9'
	}
}

func (p *Parser) parseAmount() *ast.Amount {
	s := p.cur.Span
	amt := &ast.Amount{
		QuantityFmt: ast.QuantityFormat{Decimal: '.'},
		Span:        p.span(s),
	}

	// commodity before quantity: $10.00, eur 10.00
	if p.got(token.COMMODITYMARK) || p.got(token.TEXT) || p.got(token.STRING) {
		amt.Commodity = unquote(p.cur.Literal)
		amt.CommodityPos = ast.CommodityBefore
		p.advance()
		if p.got(token.WHITESPACE) {
			amt.HasSpace = true
			p.skipWhitespace()
		}
		switch p.cur.Type {
		case token.MINUS:
			amt.IsNegative = true
			p.advance()
		case token.PLUS:
			p.advance()
		}
		p.parseQuantityInto(amt)
	} else {
		// optional sign
		switch p.cur.Type {
		case token.MINUS:
			amt.IsNegative = true
			p.advance()
		case token.PLUS:
			p.advance()
		}

		// commodity before quantity: -$120, -eur 120:
		if p.got(token.COMMODITYMARK) || p.got(token.TEXT) || p.got(token.STRING) {
			amt.Commodity = unquote(p.cur.Literal)
			amt.CommodityPos = ast.CommodityBefore
			p.advance()
			if p.got(token.WHITESPACE) {
				amt.HasSpace = true
				p.skipWhitespace()
			}
		}

		p.parseQuantityInto(amt)

		// commodity after quantity: 10.00 UAH, 10.00 "EUR" (only if not set)
		if amt.Commodity == "" {
			switch p.cur.Type {
			case token.WHITESPACE:
				p.skipWhitespace()
				if p.got(token.COMMODITYMARK) || p.got(token.TEXT) || p.got(token.STRING) {
					amt.HasSpace = true
					amt.Commodity = unquote(p.cur.Literal)
					amt.CommodityPos = ast.CommodityAfter
					p.advance()
				}
			case token.COMMODITYMARK, token.TEXT, token.STRING:
				amt.Commodity = unquote(p.cur.Literal)
				amt.CommodityPos = ast.CommodityAfter
				p.advance()
			}
		}
	}

	return amt
}

func (p *Parser) parseAmountWithOptExpr() *ast.Amount {
	if p.got(token.STAR) {
		p.advance()
		p.skipWhitespace()
		amt := p.parseAmount()
		if amt != nil {
			amt.IsExpr = true
		}
		return amt
	}
	if p.got(token.PARENEXPR) {
		lit := p.cur.Literal
		amt := &ast.Amount{
			IsExpr:      true,
			QuantityFmt: ast.QuantityFormat{Decimal: '.'},
		}
		if len(lit) >= 2 && lit[0] == '(' && lit[len(lit)-1] == ')' {
			inner := lit[1 : len(lit)-1]
			i := 0
			for i < len(inner) && (inner[i] == ' ' || inner[i] == '\t') {
				i++
			}
			j := len(inner)
			for j > i && (inner[j-1] == ' ' || inner[j-1] == '\t') {
				j--
			}
			amt.Expr = inner[i:j]
		}
		amt.Span = p.cur.Span
		p.advance()
		return amt
	}
	return p.parseAmount()
}

func (p *Parser) parsePosting() *ast.Posting {
	s := p.cur.Span
	posting := &ast.Posting{}
	p.expect(token.INDENT)

	// exit if it's empty line
	if p.got(token.NEWLINE) || p.got(token.EOF) {
		p.syncToNextline()
		return nil
	}

	// optional status, outside of brackets, '! (account)'
	posting.Status = p.parseStatus()

	// detect virtual posting brackets
	switch p.cur.Type {
	case token.LPAREN:
		posting.Type = ast.PostingVirtualUnbalanced
		p.advance()
	case token.LBRACKET:
		posting.Type = ast.PostingVirtualBalanced
		p.advance()
	}

	// optional status, inside of brackets, '(* account)'
	if p.got(token.STAR) || p.got(token.BANG) {
		posting.Status = p.parseStatus()
	}

	// validate, must be account text
	if p.cur.Type != token.TEXT {
		p.errorf("expected account name, got %s", p.cur.Type)
		p.syncToNextline()
		return nil
	}

	posting.Account = p.parseAccount()

	// consume closing bracket
	switch p.cur.Type {
	case token.RPAREN:
		p.advance()
	case token.RBRACKET:
		p.advance()
	}

	// optional amount - after two spaces
	if p.got(token.WHITESPACE) {
		p.skipWhitespace()
		if p.isAmountStart() || p.got(token.STAR) {
			posting.Amount = p.parseAmountWithOptExpr()
		}
	}

	// optional cost '@' or '@@'
	if p.got(token.WHITESPACE) {
		p.skipWhitespace()
	}
	if p.got(token.AT) || p.got(token.ATAT) {
		posting.Cost = p.parseCost()
	}

	// optional balance assertion
	if p.got(token.WHITESPACE) {
		p.skipWhitespace()
	}
	if p.got(token.EQ) || p.got(token.EQEQ) || p.got(token.EQEQEQ) {
		posting.Balance = p.parseBalanceAssertion()
	}

	posting.Comment = p.parseOptInlineComment()
	p.expectNewline()

	// continuation comments
	for p.got(token.INDENT) && p.willGet(token.SEMICOLON) {
		p.advance()
		c := p.parseComment()
		posting.Comments = append(posting.Comments, *c)
	}

	posting.Span = p.span(s)
	return posting
}

func (p *Parser) parseCost() *ast.Cost {
	s := p.cur.Span
	isTotal := p.got(token.ATAT)
	p.advance() // consume '@' '@@'
	p.skipWhitespace()
	return &ast.Cost{
		IsTotal: isTotal,
		Amount:  *p.parseAmount(),
		Span:    p.span(s),
	}
}

func (p *Parser) parseBalanceAssertion() *ast.BalanceAssertion {
	s := p.cur.Span

	ba := &ast.BalanceAssertion{}
	switch p.cur.Type {
	case token.EQ: // basic assertion
	case token.EQEQ:
		ba.IsStrict = true
	case token.EQEQEQ:
		ba.IsStrict = true
		ba.IsInclusive = true
	}
	p.advance()
	p.skipWhitespace()

	ba.Amount = *p.parseAmount()
	p.skipWhitespace()
	if p.got(token.AT) || p.got(token.ATAT) {
		c := p.parseCost()
		ba.Cost = c
	}
	ba.Span = p.span(s)
	return ba
}

func (p *Parser) readAccountSegment() (ast.SubAccount, bool) {
	switch p.cur.Type {
	case token.TEXT:
		sub := ast.SubAccount{Name: p.cur.Literal, Span: p.cur.Span}
		p.advance()

		// handle multi work segment, e.g: "credit card"
		if p.got(token.WHITESPACE) && p.willGet(token.TEXT) && len(p.peek.Literal) > 0 && p.peek.Literal[0] != '(' {
			sub.Name += " "
			p.advance()
			sub.Name += p.cur.Literal
			p.advance()
		}
		return sub, true

	case token.COMMODITYMARK:
		sub := ast.SubAccount{Name: p.cur.Literal, Span: p.cur.Span}
		p.advance()
		// merge "EUR" + "-HRK" to "EUR-HRK"
		for p.got(token.TEXT) {
			sub.Name += p.cur.Literal
			p.advance()
		}
		return sub, true

	default:
		return ast.SubAccount{}, false
	}
}

func (p *Parser) parseAccount() ast.Account {
	s := p.cur.Span
	acc := ast.Account{}

	sub, ok := p.readAccountSegment()
	if !ok {
		p.errorf("expected account, got %s", p.cur.Type)
		return ast.Account{}
	}
	acc.Name = append(acc.Name, sub)

	for p.got(token.COLON) {
		p.advance()
		sub, ok := p.readAccountSegment()
		if !ok {
			break
		}
		acc.Name = append(acc.Name, sub)
	}

	acc.Span = p.span(s)
	return acc
}

func (p *Parser) parseDate() ast.Date {
	s := p.cur.Span
	tok, ok := p.expect(token.DATE)
	if !ok {
		return ast.Date{Span: p.span(s)}
	}

	sep := byte(0)
	lit := tok.Literal
	for i := 0; i < len(lit); i++ {
		if lit[i] == '/' || lit[i] == '-' || lit[i] == '.' {
			sep = lit[i]
			break
		}
	}
	if sep == 0 {
		p.errorf("invalid date format: %q", lit)
		return ast.Date{Span: p.span(s)}
	}

	parts := strings.Split(lit, string(sep))

	// M/D or MM/DD (year inferred)
	if len(parts) == 2 {
		month, err := strconv.Atoi(parts[0])
		day, err2 := strconv.Atoi(parts[1])
		if err != nil || err2 != nil {
			p.errorf("invalid date literal: %q", lit)
			return ast.Date{Span: p.span(s)}
		}
		if month < 1 || month > 12 {
			p.errorf("invalid month %d in %q", month, lit)
			return ast.Date{Span: p.span(s)}
		}
		if day < 1 || day > 31 {
			p.errorf("invalid day %d in %q", day, lit)
			return ast.Date{Span: p.span(s)}
		}
		return ast.Date{Month: month, Day: day, Sep: sep, Span: p.span(s)}
	}

	if len(parts) != 3 {
		p.errorf("invalid date format: %q", lit)
		return ast.Date{Span: p.span(s)}
	}

	year, err := strconv.Atoi(parts[0])
	month, err2 := strconv.Atoi(parts[1])
	day, err3 := strconv.Atoi(parts[2])
	if err != nil || err2 != nil || err3 != nil {
		p.errorf("invalid date literal: %q", lit)
		return ast.Date{Span: p.span(s)}
	}
	if month < 1 || month > 12 {
		p.errorf("invalid month %d in %q", month, lit)
		return ast.Date{Span: p.span(s)}
	}
	if day < 1 || day > 31 {
		p.errorf("invalid day %d in %q", day, lit)
		return ast.Date{Span: p.span(s)}
	}

	return ast.Date{
		Year:  year,
		Month: month,
		Day:   day,
		Sep:   sep,
		Span:  p.span(s),
	}
}

func (p *Parser) parseOptInlineComment() *ast.Comment {
	p.skipWhitespace() // todo:
	if p.cur.Type != token.SEMICOLON {
		return nil
	}

	s := p.cur.Span
	marker := p.cur.Literal[0]
	p.advance() // consume marker
	p.skipWhitespace()

	text := ""
	if p.got(token.TEXT) {
		text = p.cur.Literal
		p.advance()
	}

	return &ast.Comment{
		Marker: marker,
		Text:   text,
		Span:   p.span(s),
	}
}

func (p *Parser) parseOptPeriodicDescription() string {
	if p.cur.Type != token.WHITESPACE || len(p.cur.Literal) < 2 {
		return ""
	}

	p.skipWhitespace()

	if p.cur.Type != token.TEXT {
		return ""
	}

	return p.parseDescription()
}

func (p *Parser) parseDescription() string {
	var desc strings.Builder
	for p.got(token.TEXT) || (p.got(token.WHITESPACE) && p.willGet(token.TEXT)) {
		_, _ = desc.WriteString(p.cur.Literal)
		p.advance()
	}
	return desc.String()
}

func (p *Parser) parseDirectiveExpr() string {
	var b strings.Builder
	for p.cur.Type != token.NEWLINE && p.cur.Type != token.EOF && p.cur.Type != token.SEMICOLON {
		_, _ = b.WriteString(p.cur.Literal)
		p.advance()
	}
	return b.String()
}

func (p *Parser) parseQuantityInto(amt *ast.Amount) {
	if p.cur.Type != token.INT && p.cur.Type != token.DECIMAL && p.cur.Type != token.TEXT {
		p.errorf("expected quantity, got %s", p.cur.Type)
		return
	}

	lit := p.cur.Literal
	p.advance()

	// detect format metadata before normalizing
	amt.QuantityFmt = detectFormat(lit)

	// normalize for decimal.NewFromString
	// remove thousands separators, replace decimal mark with '.'
	normalized := normalizeLiteral(lit, amt.QuantityFmt.Thousands, amt.QuantityFmt.Decimal)

	q, err := decimal.FromString(normalized)
	if err != nil {
		p.errorf("invalid quantity %q: %v", lit, err)
		return
	}

	if amt.IsNegative {
		q = q.Neg()
	}
	amt.Quantity = q
}

func (p *Parser) parseBlankLine() *ast.BlankLine {
	s := p.cur.Span
	p.expectNewline()
	return &ast.BlankLine{Span: s}
}

func (p *Parser) expectNewline() {
	if p.got(token.NEWLINE) || p.got(token.EOF) {
		if p.got(token.NEWLINE) {
			p.advance()
		}
		return
	}
	p.errorf("expected %s, got %s", token.NEWLINE, p.cur.Type)
}

func (p *Parser) advance() token.Token {
	prev := p.cur
	p.cur = p.peek
	p.peek = p.lexer.Next()
	return prev
}

func (p *Parser) got(kind token.Type) bool     { return p.cur.Type == kind }
func (p *Parser) willGet(kind token.Type) bool { return p.peek.Type == kind }

func (p *Parser) expect(kind token.Type) (token.Token, bool) {
	if p.got(kind) {
		return p.advance(), true
	}
	p.errorf("expected %s, got %s", kind, p.cur.Type)
	return p.cur, false
}

func (p *Parser) errorf(format string, args ...any) {
	p.errors = append(p.errors, &ast.ParseError{
		Span:    p.cur.Span,
		Message: fmt.Sprintf(format, args...),
	})
}

func (p *Parser) sync() {
	for {
		switch p.cur.Type {
		case token.EOF:
			return
		case token.NEWLINE:
			p.advance()
			switch p.cur.Type {
			case token.DATE, token.ACCOUNT, token.COMMODITY,
				token.INCLUDE, token.ALIAS, token.PAYEE,
				token.TAG, token.YEAR, token.D, token.P,
				token.APPLY, token.END, token.COMMENTKW,
				token.DECIMALMARK, token.TILDE, token.N, token.EQ:
				return
			}
		default:
			p.advance()
		}
	}
}

func (p *Parser) syncToNextline() {
	for p.cur.Type != token.NEWLINE && p.cur.Type != token.EOF {
		p.advance()
	}
	if p.got(token.NEWLINE) {
		p.advance()
	}
}

func (p *Parser) skipWhitespace() {
	for p.got(token.WHITESPACE) {
		p.advance()
	}
}

func (p *Parser) span(s token.Span) token.Span {
	return token.Span{Start: s.Start, End: p.cur.Span.Start}
}

func normalizeLiteral(lit string, thousands, decimal byte) string {
	var b strings.Builder
	for _, ch := range []byte(lit) {
		if thousands != 0 && ch == thousands {
			continue // skip thousands separator
		}
		if ch == decimal {
			b.WriteByte('.')
		} else {
			b.WriteByte(ch)
		}
	}
	return b.String()
}

func detectFormat(lit string) ast.QuantityFormat {
	var separators []int
	for i, ch := range []byte(lit) {
		if ch == '.' || ch == ',' || ch == ' ' || ch == '_' || ch == '\'' {
			separators = append(separators, i)
		}
	}

	if len(separators) == 0 {
		return ast.QuantityFormat{Decimal: '.', Thousands: 0, Precision: 0}
	}

	var decimal byte
	thousands := byte(0)
	precision := 0

	if len(separators) == 1 {
		pos := separators[0]
		sepChar := lit[pos]
		if sepChar == ' ' || sepChar == '_' || sepChar == '\'' {
			thousands = sepChar
			decimal = '.' // default
			precision = 0
		} else {
			decimal = sepChar
			precision = len(lit) - pos - 1
		}
	} else {
		last := separators[len(separators)-1]
		decimal = lit[last]
		thousands = lit[separators[0]]
		precision = len(lit) - last - 1
	}

	return ast.QuantityFormat{
		Decimal:   decimal,
		Thousands: thousands,
		Precision: precision,
	}
}

func parseSimpleDate(s string) ast.Date {
	if len(s) < 8 {
		return ast.Date{}
	}
	sep := byte('-')
	if strings.Contains(s, "/") {
		sep = byte('/')
	} else if strings.Contains(s, ".") {
		sep = byte('.')
	}
	parts := strings.Split(s, string(sep))
	if len(parts) != 3 {
		return ast.Date{}
	}
	year, _ := strconv.Atoi(parts[0])
	month, _ := strconv.Atoi(parts[1])
	day, _ := strconv.Atoi(parts[2])
	return ast.Date{Year: year, Month: month, Day: day, Sep: sep}
}
