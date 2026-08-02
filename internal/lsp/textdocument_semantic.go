package lsp

import (
	"context"
	"slices"
	"unicode/utf8"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/lsp/lsputil"
	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/lexer"
	"olexsmir.xyz/clerk/journal/token"
)

func (s *server) SemanticTokensFull(ctx context.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	if !s.semanticHighlightingEnabled() {
		return &protocol.SemanticTokens{}, nil
	}

	tokens, ok := s.tokensForDoc(params.TextDocument.URI)
	if !ok {
		return &protocol.SemanticTokens{}, nil
	}
	return &protocol.SemanticTokens{Data: encodeSemTokens(tokens)}, nil
}

func (s *server) SemanticTokensRange(ctx context.Context, params *protocol.SemanticTokensRangeParams) (*protocol.SemanticTokens, error) {
	if !s.semanticHighlightingEnabled() {
		return &protocol.SemanticTokens{}, nil
	}

	tokens, ok := s.tokensForDoc(params.TextDocument.URI)
	if !ok {
		return &protocol.SemanticTokens{}, nil
	}
	start := int(params.Range.Start.Line)
	end := int(params.Range.End.Line)
	var filtered []semanticToken
	for _, t := range tokens {
		if t.line >= uint32(start) && t.line <= uint32(end) {
			filtered = append(filtered, t)
		}
	}
	return &protocol.SemanticTokens{Data: encodeSemTokens(filtered)}, nil
}

func (s *server) tokensForDoc(doc uri.URI) ([]semanticToken, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.openDocs[doc]
	if !ok {
		return nil, false
	}
	if st.semTokens != nil {
		return st.semTokens, true
	}
	st.semTokens = tokenizeForSemantics(st.text, parseJournalStr(st.text))
	s.openDocs[doc] = st
	return st.semTokens, true
}

// Implementation

const (
	SemanticDirective = iota
	SemanticDate
	SemanticAccount
	SemanticCommodity
	SemanticAmount
	SemanticStatus
	SemanticComment
	SemString
	SemOperator
	SemProperty

	semTypeCount // used to ensure we dont go out of bound
)

var tokenTypeStrings = []string{
	string(protocol.SemanticTokenTypesKeyword),   // directive
	string(protocol.SemanticTokenTypesClass),     // date
	string(protocol.SemanticTokenTypesNamespace), // account
	string(protocol.SemanticTokenTypesType),      // commodity
	string(protocol.SemanticTokenTypesNumber),    // amount
	string(protocol.SemanticTokenTypesOperator),  // status
	string(protocol.SemanticTokenTypesComment),   // comment
	string(protocol.SemanticTokenTypesString),    // string
	string(protocol.SemanticTokenTypesOperator),  // operator
	string(protocol.SemanticTokenTypesProperty),  // property
}

const (
	modifierAbstract = 1 << 0 // virtual account
	modifierNegative = 1 << 1 // negative amount
)

var modifierStrings = []string{
	"abstract", // bit 0
	"negative", // bit 1
}

func getSemanticTokensLegend() protocol.SemanticTokensLegend {
	return protocol.SemanticTokensLegend{
		TokenTypes:     tokenTypeStrings,
		TokenModifiers: modifierStrings,
	}
}

type semanticToken struct {
	line      uint32 // 0-based
	col       uint32 // 0-based UTF-16
	length    uint32 // UTF-16 code units
	tokenType uint32
	modifiers uint32
}

func tokenizeForSemantics(content string, j *ast.Journal) []semanticToken {
	var raw []rawSpan
	emit := func(s token.Span, tokType, mods uint32) {
		if s.Start.Offset >= s.End.Offset {
			return
		}
		raw = append(raw, rawSpan{s, tokType, mods})
	}
	if j == nil || len(j.Errors) > 0 {
		semLexerFallback(content, emit)
	} else {
		for _, e := range j.Entries {
			visitEntry(content, e, emit)
		}
	}
	return rawToSemanticTokens(content, raw)
}

// rawSpan is a source span tagged with semantic token
type rawSpan struct {
	span      token.Span
	tok, mods uint32
}

func rawToSemanticTokens(content string, raw []rawSpan) []semanticToken {
	if len(raw) == 0 {
		return nil
	}
	slices.SortFunc(raw, func(a, b rawSpan) int { return a.span.Start.Offset - b.span.Start.Offset })
	out := make([]semanticToken, len(raw))
	line, col, cursor := 0, 0, 0
	advance := func(end int) {
		for cursor < end {
			r, size := utf8.DecodeRuneInString(content[cursor:])
			if r == utf8.RuneError && size <= 1 {
				break
			}
			if r == '\r' {
				cursor += size
				if cursor < len(content) && content[cursor] == '\n' {
					cursor++
				}
				line++
				col = 0
				continue
			}
			if r == '\n' {
				cursor += size
				line++
				col = 0
				continue
			}
			cursor += size
			col += utf16Units(r)
		}
	}
	for i, t := range raw {
		if cursor < t.span.Start.Offset {
			advance(t.span.Start.Offset)
		}
		out[i] = semanticToken{
			line:      uint32(line),
			col:       uint32(col),
			length:    uint32(lsputil.Utf16Len(content, t.span.Start.Offset, t.span.End.Offset)),
			tokenType: t.tok,
			modifiers: t.mods,
		}
		advance(t.span.End.Offset)
	}
	return out
}

func utf16Units(r rune) int {
	if r >= 0x10000 && r <= 0x10FFFF {
		return 2
	}
	return 1
}

func visitEntry(content string, e ast.Entry, emit semEmitFn) {
	switch e := e.(type) {
	case *ast.Transaction:
		visitTransaction(content, e, emit)
	case *ast.PeriodicTransaction:
		visitPeriodicTransaction(content, e, emit)
	case *ast.AutomatedTransaction:
		visitAutomatedTransaction(content, e, emit)
	case *ast.AccountDirective:
		emit(directiveKeyword(e.Span, "account"), SemanticDirective, 0)
		emit(e.Account.Span, SemanticAccount, 0)
		if e.Comment != nil {
			emit(e.Comment.Span, SemanticComment, 0)
		}
	case *ast.CommodityDirective:
		emit(directiveKeyword(e.Span, "commodity"), SemanticDirective, 0)
		if e.Format.Span.End.Offset > 0 {
			semEmitAmount(content, &e.Format, emit)
		} else if e.CommoditySpan.Start.Offset > 0 && e.CommoditySpan.End.Offset > 0 {
			emit(e.CommoditySpan, SemanticCommodity, 0)
		}
		if e.Comment != nil {
			emit(e.Comment.Span, SemanticComment, 0)
		}
	case *ast.IncludeDirective:
		emitDirective(content, e.Span, len("include"), SemString, e.Comment, emit)
	case *ast.PayeeDirective:
		emitDirective(content, e.Span, len("payee"), SemProperty, e.Comment, emit)
	case *ast.TagDirective:
		emitDirective(content, e.Span, len("tag"), SemProperty, e.Comment, emit)
	case *ast.AliasDirective:
		emit(directiveKeyword(e.Span, "alias"), SemanticDirective, 0)
		emit(e.From.Span, SemanticAccount, 0)
		if op, ok := betweenSpan(content, e.Span.Start.File, e.From.Span.End.Offset, e.To.Span.Start.Offset); ok {
			emit(op, SemOperator, 0)
		}
		emit(e.To.Span, SemanticAccount, 0)
		if e.Comment != nil {
			emit(e.Comment.Span, SemanticComment, 0)
		}
	case *ast.YearDirective:
		kwLen := len("year")
		if content[e.Span.Start.Offset] == 'Y' {
			kwLen = 1
		}
		emitDirective(content, e.Span, kwLen, SemProperty, e.Comment, emit)
	case *ast.DecimalMarkDirective:
		emitDirective(content, e.Span, len("decimal-mark"), SemProperty, e.Comment, emit)
	case *ast.DefaultCommodityDirective:
		emit(directiveKeyword(e.Span, "D"), SemanticDirective, 0)
		semEmitAmount(content, &e.Amount, emit)
		if e.Comment != nil {
			emit(e.Comment.Span, SemanticComment, 0)
		}
	case *ast.MarketPriceDirective:
		emit(directiveKeyword(e.Span, "P"), SemanticDirective, 0)
		emit(e.DateTime.Date.Span, SemanticDate, 0)
		if e.DateTime.Time != nil {
			emit(e.DateTime.Time.Span, SemanticDate, 0)
		}
		// commodity: text between the date (or time) and the amount
		commStart := e.DateTime.Date.Span.End.Offset
		if e.DateTime.Time != nil {
			commStart = e.DateTime.Time.Span.End.Offset
		}
		if comm, ok := betweenSpan(content, e.Span.Start.File, commStart, e.Amount.Span.Start.Offset); ok {
			emit(comm, SemanticCommodity, 0)
		}
		semEmitAmount(content, &e.Amount, emit)
		if e.Comment != nil {
			emit(e.Comment.Span, SemanticComment, 0)
		}
	case *ast.ConversionDirective:
		emit(directiveKeyword(e.Span, "C"), SemanticDirective, 0)
		semEmitAmount(content, &e.From, emit)
		// = operator: text between the two amounts
		if op, ok := betweenSpan(content, e.Span.Start.File, e.From.Span.End.Offset, e.To.Span.Start.Offset); ok {
			emit(op, SemOperator, 0)
		}
		semEmitAmount(content, &e.To, emit)
		if e.Comment != nil {
			emit(e.Comment.Span, SemanticComment, 0)
		}
	case *ast.Comment:
		emit(e.Span, SemanticComment, 0)
	case *ast.CommentBlockDirective:
		emit(e.Span, SemanticComment, 0)
	case *ast.IgnoredDirective:
		emitDirective(content, e.Span, len("N"), SemProperty, e.Comment, emit)
	case *ast.ApplyDirective:
		emitDirective(content, e.Span, len("apply"), SemProperty, e.Comment, emit)
	case *ast.EndDirective:
		emitDirective(content, e.Span, len("end"), SemProperty, e.Comment, emit)
	case *ast.BlankLine:
	}
}

func visitTransaction(content string, t *ast.Transaction, emit semEmitFn) {
	emit(t.Date.Span, SemanticDate, 0)
	if t.SecondDate != nil {
		emit(t.SecondDate.Span, SemanticDate, 0)
	}
	if t.Status.Value != ast.StatusNone {
		emit(t.Status.Span, SemanticStatus, 0)
	}
	if t.Code != nil {
		emit(t.Code.Span, SemString, 0)
	}
	if t.Payee != nil {
		emit(t.Payee.Span, SemProperty, 0)
	}
	if t.Note != nil {
		emit(t.Note.Span, SemProperty, 0)
	}
	if t.Comment != nil {
		emit(t.Comment.Span, SemanticComment, 0)
	}
	for i := range t.HeaderComments {
		emit(t.HeaderComments[i].Span, SemanticComment, 0)
	}
	for _, p := range t.Postings {
		visitPosting(content, p, emit)
	}
}

func visitPeriodicTransaction(content string, pt *ast.PeriodicTransaction, emit semEmitFn) {
	// ~ operator is at the start of the period span
	emit(offsetSpan(pt.Span.Start.File, pt.Span.Start.Offset, pt.Span.Start.Offset+1), SemOperator, 0)

	// The period span covers the whole expr, including any "from ... to ..." dates
	if pt.Period.Span.End.Offset > pt.Period.Span.Start.Offset {
		var dates []*ast.Date
		if pt.Period.From != nil {
			dates = append(dates, pt.Period.From)
		}
		if pt.Period.To != nil {
			dates = append(dates, pt.Period.To)
		}
		pos := pt.Period.Span.Start.Offset
		for _, d := range dates {
			if d.Span.Start.Offset > pos {
				emit(offsetSpan(pt.Period.Span.Start.File, pos, d.Span.Start.Offset), SemProperty, 0)
			}
			emit(d.Span, SemanticDate, 0)
			pos = d.Span.End.Offset
		}
		if pos < pt.Period.Span.End.Offset {
			emit(offsetSpan(pt.Period.Span.Start.File, pos, pt.Period.Span.End.Offset), SemProperty, 0)
		}
	}
	if pt.Description != nil {
		emit(pt.Description.Span, SemProperty, 0)
	}
	if pt.Comment != nil {
		emit(pt.Comment.Span, SemanticComment, 0)
	}
	for i := range pt.HeaderComments {
		emit(pt.HeaderComments[i].Span, SemanticComment, 0)
	}
	for _, p := range pt.Postings {
		visitPosting(content, p, emit)
	}
}

func visitAutomatedTransaction(content string, at *ast.AutomatedTransaction, emit semEmitFn) {
	// = operator is at the start of the expression span
	emit(offsetSpan(at.Span.Start.File, at.Span.Start.Offset, at.Span.Start.Offset+1), SemOperator, 0)

	if at.Expr.Value != "" {
		emit(at.Expr.Span, SemString, 0)
	}
	if at.Comment != nil {
		emit(at.Comment.Span, SemanticComment, 0)
	}
	for i := range at.HeaderComments {
		emit(at.HeaderComments[i].Span, SemanticComment, 0)
	}
	for _, p := range at.Postings {
		visitPosting(content, p, emit)
	}
}

func visitPosting(content string, p *ast.Posting, emit semEmitFn) {
	if p.Status.Value != ast.StatusNone {
		emit(p.Status.Span, SemanticStatus, 0)
	}

	// virtual brackets
	if p.Type == ast.PostingVirtualUnbalanced || p.Type == ast.PostingVirtualBalanced {
		// opening bracket
		for off := p.Span.Start.Offset; off < p.Account.Span.Start.Offset && off < p.Span.End.Offset; off++ {
			if content[off] == '(' || content[off] == '[' {
				brSpan := token.Span{Start: offsetPos(p.Span.Start.File, off), End: offsetPos(p.Span.Start.File, off+1)}
				emit(brSpan, SemOperator, modifierAbstract)
				break
			}
		}
		// closing bracket
		for off := p.Account.Span.End.Offset; off < p.Span.End.Offset; off++ {
			if content[off] == ')' || content[off] == ']' {
				brSpan := token.Span{Start: offsetPos(p.Span.Start.File, off), End: offsetPos(p.Span.Start.File, off+1)}
				emit(brSpan, SemOperator, modifierAbstract)
				break
			}
		}
	}

	emit(p.Account.Span, SemanticAccount, 0)

	if p.Amount != nil {
		semEmitAmount(content, p.Amount, emit)
	}
	if p.Cost != nil {
		semEmitCost(content, p.Cost, emit)
	}
	if p.Balance != nil {
		semEmitBalanceAssertion(content, p.Balance, emit)
	}
	if p.Comment != nil {
		emit(p.Comment.Span, SemanticComment, 0)
	}
	for i := range p.Comments {
		emit(p.Comments[i].Span, SemanticComment, 0)
	}
}

// directiveKeyword returns the span of the leading keyword on a directive line.
func directiveKeyword(e token.Span, kw string) token.Span {
	return token.Span{Start: e.Start, End: offsetPos(e.Start.File, e.Start.Offset+len(kw))}
}

// directiveValue returns the trimmed span of the text after the keyword end
// offset, up to the inline comment or the end of the line.
func directiveValue(content string, e token.Span, comment *ast.Comment, kwEnd int) (token.Span, bool) {
	end := e.End.Offset
	if comment != nil {
		end = comment.Span.Start.Offset
	}
	return betweenSpan(content, e.Start.File, kwEnd, end)
}

func semEmitAmount(content string, a *ast.Amount, emit semEmitFn) {
	if a == nil {
		return
	}
	hasCommodity := a.CommoditySpan.Start.Offset > 0 && a.CommoditySpan.End.Offset > 0
	if hasCommodity && a.CommodityPos == ast.CommodityBefore {
		emit(a.CommoditySpan, SemanticCommodity, 0)
		semEmitQuantity(content, a, emit)
		return
	}
	semEmitQuantity(content, a, emit)
	if hasCommodity {
		emit(a.CommoditySpan, SemanticCommodity, 0)
	}
}

func semEmitQuantity(content string, a *ast.Amount, emit semEmitFn) {
	qStart, qEnd := quantitySpan(content, a)
	if qEnd <= qStart {
		return
	}
	mods := uint32(0)
	if a.IsNegative {
		mods |= modifierNegative
	}
	emit(offsetSpan(a.Span.Start.File, qStart, qEnd), SemanticAmount, mods)
}

func quantitySpan(content string, a *ast.Amount) (int, int) {
	start, end := a.Span.Start.Offset, a.Span.End.Offset
	switch {
	case a.Commodity == "":
		// bare quantity
	case a.CommodityPos == ast.CommodityBefore:
		// "$50.00" or "$   50.00": quantity follows the commodity span
		start = a.CommoditySpan.End.Offset
	default: // CommodityAfter
		// "50.00 USD" or "50.00     USD": quantity precedes the commodity
		end = a.CommoditySpan.Start.Offset
	}
	for start < end && (content[start] == ' ' || content[start] == '\t') {
		start++
	}
	for end > start && (content[end-1] == ' ' || content[end-1] == '\t') {
		end--
	}
	return start, end
}

type semEmitFn func(token.Span, uint32, uint32)

func emitDirective(content string, e token.Span, kwLen int, valType uint32, comment *ast.Comment, emit semEmitFn) {
	kwEnd := e.Start.Offset + kwLen
	emit(token.Span{Start: e.Start, End: offsetPos(e.Start.File, kwEnd)}, SemanticDirective, 0)
	if v, ok := directiveValue(content, e, comment, kwEnd); ok {
		emit(v, valType, 0)
	}
	if comment != nil {
		emit(comment.Span, SemanticComment, 0)
	}
}

func semEmitCost(content string, c *ast.Cost, emit semEmitFn) {
	if c.IsTotal {
		emit(token.Span{Start: c.Span.Start, End: offsetPos(c.Span.Start.File, c.Span.Start.Offset+2)}, SemOperator, 0)
	} else {
		emit(token.Span{Start: c.Span.Start, End: offsetPos(c.Span.Start.File, c.Span.Start.Offset+1)}, SemOperator, 0)
	}
	semEmitAmount(content, &c.Amount, emit)
}

func semEmitBalanceAssertion(content string, ba *ast.BalanceAssertion, emit semEmitFn) {
	// The operator is the run of '=', ':', '*' chars from the span start.
	// The ':' of ':=' precedes the '=' token, so back up one offset.
	opStart := ba.Span.Start.Offset
	if ba.IsAssignment && opStart > 0 {
		opStart--
	}
	opEnd := opStart
	for opEnd < ba.Span.End.Offset && (content[opEnd] == '=' || content[opEnd] == ':' || content[opEnd] == '*') {
		opEnd++
	}
	emit(token.Span{Start: offsetPos(ba.Span.Start.File, opStart), End: offsetPos(ba.Span.Start.File, opEnd)}, SemOperator, 0)
	semEmitAmount(content, &ba.Amount, emit)
	if ba.Cost != nil {
		semEmitCost(content, ba.Cost, emit)
	}
}

// semLexerFallback produces semantic tokens using only the lexer (for unparseable documents).
func semLexerFallback(content string, emit semEmitFn) {
	l := lexer.New("", []byte(content))

	var commentStart, commentEnd int // 0 = not inside a comment line
	take := func(span token.Span, tokType uint32, mods uint32) {
		emit(span, tokType, mods)
	}
	for {
		tok := l.Next()
		if tok.Type == token.EOF {
			if commentStart > 0 {
				take(token.Span{Start: offsetPos("", commentStart), End: offsetPos("", commentEnd)}, SemanticComment, 0)
			}
			break
		}
		if tok.Type == token.NEWLINE {
			if commentStart > 0 {
				take(token.Span{Start: offsetPos("", commentStart), End: offsetPos("", commentEnd)}, SemanticComment, 0)
				commentStart, commentEnd = 0, 0
			}
			continue
		}
		if tok.Type == token.WHITESPACE || tok.Type == token.INDENT {
			continue
		}
		tokType := uint32(SemString)
		if commentStart > 0 {
			tokType = SemanticComment
			if tok.Span.End.Offset > commentEnd {
				commentEnd = tok.Span.End.Offset
			}
			continue
		}
		switch tok.Type {
		case token.SEMICOLON, token.HASH, token.PERCENT:
			tokType = SemanticComment
			commentStart = tok.Span.Start.Offset
			commentEnd = tok.Span.End.Offset
			continue
		case token.STAR:
			tokType = SemanticComment // * at col 0 is comment marker
			commentStart = tok.Span.Start.Offset
			commentEnd = tok.Span.End.Offset
			continue
		case token.ACCOUNT, token.COMMODITY, token.INCLUDE, token.ALIAS,
			token.PAYEE, token.TAG, token.APPLY, token.END, token.COMMENTKW,
			token.YEAR, token.DECIMALMARK, token.D, token.P, token.N, token.C:
			tokType = SemanticDirective
		case token.DATE:
			tokType = SemanticDate
		case token.INT, token.DECIMAL:
			tokType = SemanticAmount
		case token.COMMODITYMARK:
			tokType = SemanticCommodity
		case token.BANG:
			tokType = SemanticStatus
		case token.AT, token.ATAT, token.EQ, token.EQEQ, token.EQEQEQ, token.EQSTAR:
			tokType = SemOperator
		}
		take(tok.Span, tokType, 0)
	}
}

func encodeSemTokens(tokens []semanticToken) []uint32 {
	if len(tokens) == 0 {
		return nil
	}
	slices.SortFunc(tokens, func(a, b semanticToken) int {
		if a.line != b.line {
			return int(a.line) - int(b.line)
		}
		return int(a.col) - int(b.col)
	})
	data := make([]uint32, 0, len(tokens)*5)
	var prevLine, prevCol uint32
	for _, t := range tokens {
		var deltaLine, deltaCol uint32
		if t.line == prevLine {
			deltaLine = 0
			deltaCol = t.col - prevCol
		} else {
			deltaLine = t.line - prevLine
			deltaCol = t.col
		}
		data = append(data, deltaLine, deltaCol, t.length, t.tokenType, t.modifiers)
		prevLine = t.line
		prevCol = t.col
	}
	return data
}

// betweenSpan returns the span of the text between two offsets, trimmed of surrounding whitespace.
func betweenSpan(content, file string, start, end int) (token.Span, bool) {
	for start < end && (content[start] == ' ' || content[start] == '\t') {
		start++
	}
	for end > start && (content[end-1] == ' ' || content[end-1] == '\t' || content[end-1] == '\n' || content[end-1] == '\r') {
		end--
	}
	if end <= start {
		return token.Span{}, false
	}
	return token.Span{Start: offsetPos(file, start), End: offsetPos(file, end)}, true
}

func offsetPos(file string, offset int) token.Pos { return token.Pos{File: file, Offset: offset} }
func offsetSpan(file string, start, end int) token.Span {
	return token.Span{Start: offsetPos(file, start), End: offsetPos(file, end)}
}
