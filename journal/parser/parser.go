package parser

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

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

	defaultYear int // set by year directive, used for short date inference
}

func New(lex *lexer.Lexer) *Parser {
	p := &Parser{lexer: lex}
	p.advance() // populate .peek
	p.advance() // populate .cur
	return p
}

func NewWithYear(lex *lexer.Lexer, year int) *Parser {
	p := &Parser{lexer: lex, defaultYear: year}
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

	// optional code - the lexer emits "(CODE)" as a single TEXT token; split it here
	if p.got(token.TEXT) {
		if lit := p.cur.Literal; len(lit) >= 2 && lit[0] == '(' && lit[len(lit)-1] == ')' {
			tx.Code = &ast.Code{Value: lit[1 : len(lit)-1], Span: p.cur.Span}
			p.advance()
			p.skipWhitespace()
		}
	}

	// optional payee | note
	if p.got(token.TEXT) || p.got(token.STRING) {
		tx.Payee = p.parsePayee()

		// check for | separator
		p.skipWhitespace()

		if p.got(token.PIPE) {
			p.advance()
			if p.got(token.TEXT) {
				sn := p.cur.Span
				n := p.cur.Literal
				p.advance()
				tx.Note = &ast.Note{Value: n, Span: p.span(sn)}
			}
		}
	}

	tx.Comment = p.parseOptInlineComment()
	p.expectNewline()

	tx.HeaderComments, tx.Postings = p.parseHeaderCommentsAndPostings()

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
	for payeeWord(p.cur.Type) || (payeeWord(p.peek.Type) && p.got(token.WHITESPACE)) {
		_, _ = name.WriteString(p.cur.Literal)
		p.advance()
	}
	return &ast.Payee{Name: unquote(name.String()), Span: p.span(s)}
}

func payeeWord(t token.Type) bool {
	switch t {
	case token.TEXT, token.INT, token.DECIMAL, token.COMMODITYMARK:
		return true
	}
	return false
}

func (p *Parser) parsePeriodicTransaction() *ast.PeriodicTransaction {
	s := p.cur.Span
	p.expect(token.TILDE)
	p.skipWhitespace()

	pt := &ast.PeriodicTransaction{}

	pt.Period = p.parsePeriod()

	if desc, dspan := p.parseOptPeriodicDescription(); desc != "" {
		pt.Description = &ast.Description{Value: desc, Span: dspan}
	}

	comment := p.parseOptInlineComment()
	p.expectNewline()

	pt.HeaderComments, pt.Postings = p.parseHeaderCommentsAndPostings()

	pt.Span = p.span(s)
	pt.Comment = comment
	return pt
}

func (p *Parser) parseAutomatedTransaction() *ast.AutomatedTransaction {
	s := p.cur.Span
	p.expect(token.EQ)
	p.skipWhitespace()

	at := &ast.AutomatedTransaction{}

	// expression
	sd := p.cur.Span
	expr := p.parseDirectiveExpr()
	at.Expr = ast.Expr{Value: expr, Span: p.span(sd)}
	at.Comment = p.parseOptInlineComment()
	p.expectNewline()

	at.HeaderComments, at.Postings = p.parseHeaderCommentsAndPostings()

	at.Span = p.span(s)
	return at
}

func (p *Parser) parseHeaderCommentsAndPostings() (comments []*ast.Comment, postings []*ast.Posting) {
	for p.got(token.INDENT) && p.willGet(token.SEMICOLON) {
		p.advance() // consume indent
		comments = append(comments, p.parseComment())
	}

	for p.got(token.INDENT) {
		if posting := p.parsePosting(); posting != nil {
			postings = append(postings, posting)
		}
	}

	return comments, postings
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
			fromOff := strings.Index(str, dateStr)
			d.Span = periodDateSpan(period, str, dateStr, fromOff)
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
					d.Span = periodDateSpan(period, str, toAfter, fromOff+len(dateStr))
					period.To = &d
				}
			}
		}
	}
	return period
}

// periodDateSpan returns the source span of dateStr, which occurs in the
// period text at or after searchFrom. The period span and text cover the same
// bytes, so offsets line up 1:1.
func periodDateSpan(period ast.Period, text, dateStr string, searchFrom int) token.Span {
	off := strings.Index(text[searchFrom:], dateStr)
	abs := period.Span.Start.Offset + searchFrom + off
	return token.Span{
		Start: token.Pos{File: period.Span.Start.File, Offset: abs},
		End:   token.Pos{File: period.Span.Start.File, Offset: abs + len(dateStr)},
	}
}

func (p *Parser) parseComment() *ast.Comment {
	s := p.cur.Span
	c := p.parseCommentRest(s)
	p.expectNewline()
	c.Span = p.span(s) // comment spans its line through the newline
	return c
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
	var commoditySpan token.Span
	var format *ast.Amount

	switch p.cur.Type {
	case token.COMMODITYMARK, token.TEXT, token.STRING:
		cs := p.cur.Span
		commodity = unquote(p.cur.Literal)
		p.advance()
		commoditySpan = token.Span{Start: cs.Start, End: p.cur.Span.Start}
		hadSpace := p.got(token.WHITESPACE)
		p.skipWhitespace()
		if p.got(token.INT) || p.got(token.DECIMAL) || p.got(token.TEXT) {
			format = p.parseAmount()
			format.Commodity = commodity
			format.CommoditySpan = commoditySpan
			format.CommodityPos = ast.CommodityBefore
			format.HasSpace = hadSpace
		}
	case token.INT, token.DECIMAL:
		format = p.parseAmount()
		commodity = format.Commodity
		commoditySpan = format.CommoditySpan
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
		if p.got(token.TEXT) && p.cur.Literal == "format" {
			p.advance()
			p.skipWhitespace()
			format = p.parseAmount()
			p.expectNewline()
			continue
		}
		for !p.got(token.NEWLINE) && !p.got(token.EOF) {
			p.advance()
		}
		p.expectNewline()
	}

	cd := &ast.CommodityDirective{
		Commodity:     commodity,
		CommoditySpan: commoditySpan,
		Comment:       comment,
		Span:          p.span(s),
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
	if p.got(token.TEXT) || p.got(token.STRING) || p.got(token.COMMODITYMARK) {
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
	if p.got(token.TEXT) || p.got(token.COMMODITYMARK) || p.got(token.STRING) {
		name = unquote(p.cur.Literal)
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
		p.defaultYear = year.Year
		p.advance()
	} else {
		p.errorf("expected year, got %s", p.cur.Type)
	}

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
	p.skipWhitespace()

	mp.Amount = *p.parseAmount()

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

func (p *Parser) parseStatus() ast.Status {
	s := p.cur.Span
	st := ast.Status{}
	switch p.cur.Type {
	case token.STAR:
		st.Value = ast.StatusCleared
	case token.BANG:
		st.Value = ast.StatusPending
	}
	if st.Value != ast.StatusNone {
		p.advance()
		p.skipWhitespace()
	}
	st.Span = p.span(s)
	return st
}

func (p *Parser) isAmountStart() bool {
	switch p.cur.Type {
	default:
		return false
	case token.COMMODITYMARK, token.STRING, token.INT, token.DECIMAL, token.MINUS, token.PLUS, token.PARENEXPR, token.STAR:
		return true
	}
}

func (p *Parser) parseAmount() *ast.Amount {
	s := p.cur.Span
	amt := &ast.Amount{
		QuantityFmt: ast.QuantityFormat{Decimal: '.'},
	}
	defer func() {
		// The span covers from the first token to the start of the next unconsumed token.
		// Since parseQuantityInto (and possible commodity consumption) advanced past the last
		// amount token, p.cur points to the next token after the amount — which is the correct end.
		amt.Span = p.span(s)
	}()

	p.parseAmountSign(amt)
	p.skipWhitespace()

	// commodity before quantity: $10.00, eur 10.00
	if p.got(token.COMMODITYMARK) || p.got(token.TEXT) || p.got(token.STRING) {
		cs := p.cur.Span
		amt.Commodity = unquote(p.cur.Literal)
		amt.CommodityPos = ast.CommodityBefore
		p.advance()
		amt.CommoditySpan = token.Span{Start: cs.Start, End: p.cur.Span.Start}
		if p.got(token.WHITESPACE) {
			amt.HasSpace = true
			p.skipWhitespace()
		}
	}

	// optional sign after commodity: $ -10
	p.parseAmountSign(amt)
	p.skipWhitespace()

	p.parseQuantityInto(amt)

	// commodity after quantity: 10.00 UAH, 10.00 "EUR" (only if not set)
	if amt.Commodity == "" {
		switch p.cur.Type {
		case token.WHITESPACE:
			p.skipWhitespace()
			if p.got(token.COMMODITYMARK) || p.got(token.TEXT) || p.got(token.STRING) {
				cs := p.cur.Span
				amt.HasSpace = true
				amt.Commodity = unquote(p.cur.Literal)
				amt.CommodityPos = ast.CommodityAfter
				p.advance()
				amt.CommoditySpan = token.Span{Start: cs.Start, End: p.cur.Span.Start}
			}
		case token.COMMODITYMARK, token.TEXT, token.STRING:
			cs := p.cur.Span
			amt.Commodity = unquote(p.cur.Literal)
			amt.CommodityPos = ast.CommodityAfter
			p.advance()
			amt.CommoditySpan = token.Span{Start: cs.Start, End: p.cur.Span.Start}
		}
	}

	return amt
}

// parseAmountSign consumes an optional leading +/- into IsNegative.
func (p *Parser) parseAmountSign(amt *ast.Amount) {
	switch p.cur.Type {
	case token.MINUS:
		amt.IsNegative = true
		p.advance()
	case token.PLUS:
		p.advance()
	}
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
			amt.Expr = strings.Trim(lit[1:len(lit)-1], " \t")
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
		if p.isAmountStart() {
			posting.Amount = p.parseAmountWithOptExpr()
		}
	}

	// optional cost '@' or '@@'
	p.skipWhitespace()
	if p.got(token.AT) || p.got(token.ATAT) {
		posting.Cost = p.parseCost()
	}

	// optional balance assertion or assignment
	p.skipWhitespace()
	if p.got(token.COLON) && p.willGet(token.EQ) {
		p.advance() // consume ':' of ':='
		posting.Balance = p.parseBalanceAssertion()
		posting.Balance.IsAssignment = true
	} else if p.got(token.EQ) || p.got(token.EQEQ) || p.got(token.EQEQEQ) || p.got(token.EQSTAR) {
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
	case token.EQSTAR: // inclusive assertion
		ba.IsInclusive = true
	case token.EQEQ: // strict assertion
		ba.IsStrict = true
	case token.EQEQEQ: // strict inclusive assertion
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

	year, month, day, sep, err := parseDateLiteral(tok.Literal)
	if err != nil {
		p.errorf("%v", err)
		return ast.Date{Span: p.span(s)}
	}
	if year == 0 {
		year = p.defaultYear
	}

	return ast.Date{Year: year, Month: month, Day: day, Sep: sep, Span: p.span(s)}
}

func (p *Parser) parseOptInlineComment() *ast.Comment {
	p.skipWhitespace()
	if !p.got(token.SEMICOLON) {
		return nil
	}
	return p.parseCommentRest(p.cur.Span)
}

// parseCommentRest consumes a comment marker at p.cur, then optional text;
// s anchors the span at the marker's start.
func (p *Parser) parseCommentRest(s token.Span) *ast.Comment {
	marker := p.cur.Literal[0]
	p.advance()
	p.skipWhitespace()

	var tags []ast.Tag
	text := ""
	if p.got(token.TEXT) {
		text = p.cur.Literal
		tags = parseCommentTags(text, p.cur.Span.Start)
		p.advance()
	}

	return &ast.Comment{
		Marker: marker,
		Tags:   tags,
		Text:   text,
		Span:   p.span(s),
	}
}

func (p *Parser) parseOptPeriodicDescription() (string, token.Span) {
	if p.cur.Type != token.WHITESPACE || len(p.cur.Literal) < 2 {
		return "", token.Span{}
	}

	p.skipWhitespace()

	if p.cur.Type != token.TEXT {
		return "", token.Span{}
	}

	s := p.cur.Span
	desc := p.parseDescription()
	return desc, p.span(s)
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

func isDirectiveKeyword(t token.Type) bool {
	switch t {
	case token.COMMENTKW, token.ACCOUNT, token.COMMODITY, token.INCLUDE,
		token.ALIAS, token.PAYEE, token.TAG, token.APPLY, token.END,
		token.YEAR, token.DECIMALMARK, token.D, token.P, token.N, token.C:
		return true
	}
	return false
}

func (p *Parser) sync() {
	for {
		switch p.cur.Type {
		case token.EOF:
			return
		case token.NEWLINE:
			p.advance()
			t := p.cur.Type
			if isDirectiveKeyword(t) || t == token.DATE || t == token.TILDE || t == token.EQ {
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
	var seps []int
	for i, ch := range []byte(lit) {
		if ch == '.' || ch == ',' || ch == ' ' || ch == '_' || ch == '\'' {
			seps = append(seps, i)
		}
	}

	if len(seps) == 0 {
		return ast.QuantityFormat{Decimal: '.', Thousands: 0, Precision: 0}
	}

	last := seps[len(seps)-1]
	dec := lit[last]
	var thou byte
	if len(seps) > 1 {
		thou = lit[seps[0]]
	} else if dec == ' ' || dec == '_' || dec == '\'' {
		// single space/underscore/apostrophe is always thousands
		thou = dec
		dec = '.'
	}

	// calculate precision when the last separator is a real decimal
	prec := 0
	if thou == 0 || len(seps) > 1 {
		prec = len(lit) - last - 1
	}

	return ast.QuantityFormat{Decimal: dec, Thousands: thou, Precision: prec}
}

// parseSimpleDate  parses full YYYY/MM/DD date literal embedded in free text.
func parseSimpleDate(s string) ast.Date {
	year, month, day, sep, err := parseDateLiteral(s)
	if err != nil {
		return ast.Date{}
	}
	return ast.Date{Year: year, Month: month, Day: day, Sep: sep}
}

// parseDateLiteral parses and validates a date literal.
func parseDateLiteral(lit string) (year, month, day int, sep byte, err error) {
	sep = dateSeparator(lit)
	if sep == 0 {
		return 0, 0, 0, 0, fmt.Errorf("invalid date format: %q", lit)
	}

	parts := strings.Split(lit, string(sep))
	if len(parts) != 2 && len(parts) != 3 {
		return 0, 0, 0, 0, fmt.Errorf("invalid date format: %q", lit)
	}

	nums := make([]int, len(parts))
	for i, part := range parts {
		if nums[i], err = strconv.Atoi(part); err != nil {
			return 0, 0, 0, 0, fmt.Errorf("invalid date literal: %q", lit)
		}
	}

	month = nums[len(parts)-2]
	if month < 1 || month > 12 {
		return 0, 0, 0, 0, fmt.Errorf("invalid month %d in %q", month, lit)
	}

	day = nums[len(parts)-1]
	if day < 1 || day > 31 {
		return 0, 0, 0, 0, fmt.Errorf("invalid day %d in %q", day, lit)
	}

	if len(parts) == 2 {
		return 0, month, day, sep, nil
	}
	return nums[0], month, day, sep, nil
}

func dateSeparator(lit string) byte {
	for i := 0; i < len(lit); i++ {
		if lit[i] == '/' || lit[i] == '-' || lit[i] == '.' {
			return lit[i]
		}
	}
	return 0
}

// parseCommentTags extacts tags from comment text.
// A tag is a word immediately followed by a ':', with an optional value that ends at a comma or the end of a line.
// https://hledger.org/1.52/hledger.html?highlight=tags#tags
func parseCommentTags(text string, base token.Pos) []ast.Tag {
	var tags []ast.Tag
	for i := 0; i < len(text); {
		colon := strings.IndexByte(text[i:], ':')
		if colon < 0 {
			break
		}
		colon += i

		keyStart := colon
		for keyStart > i {
			r, size := utf8.DecodeLastRuneInString(text[:keyStart])
			if unicode.IsSpace(r) {
				break
			}
			keyStart -= size
		}
		if keyStart == colon { // nothing before the colon = not a tag
			i = colon + 1
			continue
		}
		key := text[keyStart:colon]

		valueEnd := colon + 1
		for valueEnd < len(text) && text[valueEnd] != ',' {
			valueEnd++
		}
		value := strings.TrimSpace(text[colon+1 : valueEnd])

		tags = append(tags, ast.Tag{
			Key:   key,
			Value: value,
			Span: token.Span{
				Start: tagPos(base, text, keyStart),
				End:   tagPos(base, text, valueEnd),
			},
		})
		i = valueEnd
		if i < len(text) && text[i] == ',' {
			i++
		}
	}

	return tags
}

func tagPos(base token.Pos, text string, off int) token.Pos {
	return token.Pos{
		File:   base.File,
		Offset: base.Offset + off,
		Line:   base.Line,
		Col:    base.Col + utf8.RuneCountInString(text[:off]),
	}
}
