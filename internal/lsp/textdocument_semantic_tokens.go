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
	return s.semanticTokensFullResult(params.TextDocument.URI), nil
}

func (s *server) SemanticTokensFullDelta(ctx context.Context, params *protocol.SemanticTokensDeltaParams) (protocol.SemanticTokensDeltaResult, error) {
	if !s.semanticHighlightingEnabled() {
		return &protocol.SemanticTokens{}, nil
	}

	u := params.TextDocument.URI
	s.mu.RLock()
	st, ok := s.openDocs[u]
	s.mu.RUnlock()
	if !ok || st.semGen == 0 || params.PreviousResultID != st.resultID() {
		return s.semanticTokensFullResult(u), nil
	}

	data, ok := s.semanticTokensData(u)
	if !ok {
		return &protocol.SemanticTokens{}, nil
	}
	edits := semanticTokensEdits(st.semBaseline, data)
	if len(edits) == 0 {
		return &protocol.SemanticTokensDelta{ResultID: new(st.resultID()), Edits: []protocol.SemanticTokensEdit{}}, nil
	}

	rid, ok := s.storeSemResult(u, data)
	if !ok {
		return &protocol.SemanticTokens{Data: data}, nil
	}
	return &protocol.SemanticTokensDelta{ResultID: &rid, Edits: edits}, nil
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

func (s *server) semanticTokensFullResult(u uri.URI) *protocol.SemanticTokens {
	data, ok := s.semanticTokensData(u)
	if !ok {
		return &protocol.SemanticTokens{}
	}
	res := &protocol.SemanticTokens{Data: data}
	if rid, ok := s.storeSemResult(u, data); ok {
		res.ResultID = &rid
	}
	return res
}

func (s *server) semanticTokensData(u uri.URI) ([]uint32, bool) {
	tokens, ok := s.tokensForDoc(u)
	if !ok {
		return nil, false
	}
	return encodeSemTokens(tokens), true
}

func (s *server) storeSemResult(u uri.URI, data []uint32) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.openDocs[u]
	if !ok {
		return "", false
	}
	rid := st.nextResultID()
	st.semBaseline = data
	s.openDocs[u] = st
	return rid, true
}

func (s *server) tokensForDoc(doc uri.URI) ([]semanticToken, bool) {
	s.mu.RLock()
	st, ok := s.openDocs[doc]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if st.semTokens != nil {
		return st.semTokens, true
	}
	// Tokenize outside the lock: a full tokenization of a large journal is
	// milliseconds, during which didChange/didOpen would otherwise stall.
	rj := s.loader.ResolveBytes(doc.Path(), []byte(st.text))
	tokens := tokenizeForSemantics(st.text, rj.Occurrences[0].Ast)
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.openDocs[doc]
	if !ok {
		return nil, false
	}
	if cur.text == st.text { // unchanged during tokenization
		cur.semTokens = tokens
		s.openDocs[doc] = cur
	}
	return tokens, true
}

const (
	semDirective uint32 = iota
	semDate
	semAccount
	semCommodity
	semAmount
	semStatus
	semComment
	semString
	semOperator
	semProperty
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
	line, col uint32 // 0-based
	length    uint32
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
	for _, e := range j.Entries {
		visitEntry(content, e, emit)
	}
	if len(j.Errors) > 0 {
		// parser recovers per line; lexer fills the unparsed regions, keeping ast tokens where the parser succeeded
		semLexerFallback(content, raw, emit)
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

func visitEntry(content string, e ast.Entry, emit semEmitFunc) {
	switch e := e.(type) {
	case *ast.Transaction:
		visitTransaction(content, e, emit)
	case *ast.PeriodicTransaction:
		visitPeriodicTransaction(content, e, emit)
	case *ast.AutomatedTransaction:
		visitAutomatedTransaction(content, e, emit)
	case *ast.AccountDirective:
		emit(directiveKeyword(e.Span, "account"), semDirective, 0)
		emit(e.Account.Span, semAccount, 0)
		for _, sd := range e.Subdirectives {
			if sd.Kind == ast.SubdirectiveComment {
				emitComment(sd.Comment, emit)
				continue
			}
			emit(sd.NameSpan, semDirective, 0)
			switch sd.Kind {
			case ast.SubdirectiveAlias:
				emit(sd.ValueSpan, semAccount, 0)
			case ast.SubdirectiveType, ast.SubdirectiveNote:
				emit(sd.ValueSpan, semProperty, 0)
			}
			emitComment(sd.Comment, emit)
		}
		emitComment(e.Comment, emit)
	case *ast.CommodityDirective:
		emit(directiveKeyword(e.Span, "commodity"), semDirective, 0)
		if e.FormatSub != nil {
			if e.FormatSub.KeywordSpan.End.Offset > 0 {
				emit(e.FormatSub.KeywordSpan, semDirective, 0)
			}
			semEmitAmount(content, &e.FormatSub.Amount, emit)
			emitComment(e.FormatSub.Comment, emit)
		} else if e.CommoditySpan.Start.Offset > 0 && e.CommoditySpan.End.Offset > 0 {
			emit(e.CommoditySpan, semCommodity, 0)
		}
		emitBlockComments(e.BlockComments, emit)
		emitComment(e.Comment, emit)
	case *ast.IncludeDirective:
		emitDirective(content, e.Span, len("include"), semString, e.Comment, emit)
	case *ast.PayeeDirective:
		emitDirective(content, e.Span, len("payee"), semProperty, e.Comment, emit)
	case *ast.TagDirective:
		emitDirective(content, e.Span, len("tag"), semProperty, e.Comment, emit)
	case *ast.AliasDirective:
		emit(directiveKeyword(e.Span, "alias"), semDirective, 0)
		emit(e.From.Span, semAccount, 0)
		if op, ok := betweenSpan(content, e.Span.File, e.From.Span.End.Offset, e.To.Span.Start.Offset); ok {
			emit(op, semOperator, 0)
		}
		emit(e.To.Span, semAccount, 0)
		emitComment(e.Comment, emit)
	case *ast.YearDirective:
		kwLen := len("year")
		if content[e.Span.Start.Offset] == 'Y' {
			kwLen = 1
		}
		emitDirective(content, e.Span, kwLen, semProperty, e.Comment, emit)
	case *ast.DecimalMarkDirective:
		emitDirective(content, e.Span, len("decimal-mark"), semProperty, e.Comment, emit)
	case *ast.DefaultCommodityDirective:
		emit(directiveKeyword(e.Span, "D"), semDirective, 0)
		semEmitAmount(content, &e.Amount, emit)
		emitComment(e.Comment, emit)
	case *ast.MarketPriceDirective:
		emit(directiveKeyword(e.Span, "P"), semDirective, 0)
		emit(e.DateTime.Date.Span, semDate, 0)
		if e.DateTime.Time != nil {
			emit(e.DateTime.Time.Span, semDate, 0)
		}
		// commodity: text between the date (or time) and the amount
		commStart := e.DateTime.Date.Span.End.Offset
		if e.DateTime.Time != nil {
			commStart = e.DateTime.Time.Span.End.Offset
		}
		if comm, ok := betweenSpan(content, e.Span.File, commStart, e.Amount.Span.Start.Offset); ok {
			emit(comm, semCommodity, 0)
		}
		semEmitAmount(content, &e.Amount, emit)
		emitComment(e.Comment, emit)
	case *ast.ConversionDirective:
		emit(directiveKeyword(e.Span, "C"), semDirective, 0)
		semEmitAmount(content, &e.From, emit)
		// = operator: text between the two amounts
		if op, ok := betweenSpan(content, e.Span.File, e.From.Span.End.Offset, e.To.Span.Start.Offset); ok {
			emit(op, semOperator, 0)
		}
		semEmitAmount(content, &e.To, emit)
		emitComment(e.Comment, emit)
	case *ast.Comment:
		emitComment(e, emit)
	case *ast.CommentBlockDirective:
		emit(e.Span, semComment, 0)
	case *ast.IgnoredDirective:
		emitDirective(content, e.Span, len("N"), semProperty, e.Comment, emit)
	case *ast.ApplyDirective:
		emitDirective(content, e.Span, len("apply"), semProperty, e.Comment, emit)
	case *ast.EndDirective:
		emitDirective(content, e.Span, len("end"), semProperty, e.Comment, emit)
	case *ast.BlankLine:
	}
}

func visitTransaction(content string, t *ast.Transaction, emit semEmitFunc) {
	emit(t.Date.Span, semDate, 0)
	if t.SecondDate != nil {
		emit(t.SecondDate.Span, semDate, 0)
	}
	if t.Status.Value != ast.StatusNone {
		emit(t.Status.Span, semStatus, 0)
	}
	if t.Code != nil {
		emit(t.Code.Span, semString, 0)
	}
	if t.Payee != nil {
		emit(t.Payee.Span, semProperty, 0)
	}
	if t.Note != nil {
		emit(t.Note.Span, semProperty, 0)
	}
	emitComment(t.Comment, emit)
	for i := range t.HeaderComments {
		emitComment(t.HeaderComments[i], emit)
	}
	for _, p := range t.Postings {
		visitPosting(content, p, emit)
	}
}

func visitPeriodicTransaction(content string, pt *ast.PeriodicTransaction, emit semEmitFunc) {
	// ~ operator is at the start of the period span
	emit(offsetSpan(pt.Span.File, pt.Span.Start.Offset, pt.Span.Start.Offset+1), semOperator, 0)

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
				emit(offsetSpan(pt.Period.Span.File, pos, d.Span.Start.Offset), semProperty, 0)
			}
			emit(d.Span, semDate, 0)
			pos = d.Span.End.Offset
		}
		if pos < pt.Period.Span.End.Offset {
			emit(offsetSpan(pt.Period.Span.File, pos, pt.Period.Span.End.Offset), semProperty, 0)
		}
	}
	if pt.Description != nil {
		emit(pt.Description.Span, semProperty, 0)
	}
	emitComment(pt.Comment, emit)
	for i := range pt.HeaderComments {
		emitComment(pt.HeaderComments[i], emit)
	}
	for _, p := range pt.Postings {
		visitPosting(content, p, emit)
	}
}

func visitAutomatedTransaction(content string, at *ast.AutomatedTransaction, emit semEmitFunc) {
	// = operator is at the start of the expression span
	emit(offsetSpan(at.Span.File, at.Span.Start.Offset, at.Span.Start.Offset+1), semOperator, 0)

	if at.Expr.Value != "" {
		emit(at.Expr.Span, semString, 0)
	}
	emitComment(at.Comment, emit)
	for i := range at.HeaderComments {
		emitComment(at.HeaderComments[i], emit)
	}
	for _, p := range at.Postings {
		visitPosting(content, p, emit)
	}
}

func visitPosting(content string, p ast.Posting, emit semEmitFunc) {
	if p.Status.Value != ast.StatusNone {
		emit(p.Status.Span, semStatus, 0)
	}

	// virtual brackets
	if p.Type == ast.PostingVirtualUnbalanced || p.Type == ast.PostingVirtualBalanced {
		// opening bracket
		for off := p.Span.Start.Offset; off < p.Account.Span.Start.Offset && off < p.Span.End.Offset; off++ {
			if content[off] == '(' || content[off] == '[' {
				emit(offsetSpan(p.Span.File, off, off+1), semOperator, modifierAbstract)
				break
			}
		}
		// closing bracket
		for off := p.Account.Span.End.Offset; off < p.Span.End.Offset; off++ {
			if content[off] == ')' || content[off] == ']' {
				emit(offsetSpan(p.Span.File, off, off+1), semOperator, modifierAbstract)
				break
			}
		}
	}

	emit(p.Account.Span, semAccount, 0)

	if p.Amount != nil {
		semEmitAmount(content, p.Amount, emit)
	}
	if p.Cost != nil {
		semEmitCost(content, p.Cost, emit)
	}
	if p.Balance != nil {
		semEmitBalanceAssertion(content, p.Balance, emit)
	}
	emitComment(p.Comment, emit)
	for i := range p.Comments {
		emitComment(&p.Comments[i], emit)
	}
}

// directiveKeyword returns the span of the leading keyword on a directive line.
func directiveKeyword(e token.Span, kw string) token.Span {
	return token.Span{File: e.File, Start: e.Start, End: token.Pos{Offset: e.Start.Offset + len(kw)}}
}

// directiveValue returns the trimmed span of the text after the keyword end
// offset, up to the inline comment or the end of the line.
func directiveValue(content string, e token.Span, comment *ast.Comment, kwEnd int) (token.Span, bool) {
	end := e.End.Offset
	if comment != nil {
		end = comment.Span.Start.Offset
	}
	return betweenSpan(content, e.File, kwEnd, end)
}

func semEmitAmount(content string, a *ast.Amount, emit semEmitFunc) {
	if a == nil {
		return
	}
	hasCommodity := a.CommoditySpan.Start.Offset > 0 && a.CommoditySpan.End.Offset > 0
	if hasCommodity && a.CommodityPos == ast.CommodityBefore {
		emit(a.CommoditySpan, semCommodity, 0)
		semEmitQuantity(content, a, emit)
		return
	}
	semEmitQuantity(content, a, emit)
	if hasCommodity {
		emit(a.CommoditySpan, semCommodity, 0)
	}
}

func semEmitQuantity(content string, a *ast.Amount, emit semEmitFunc) {
	qStart, qEnd := quantitySpan(content, a)
	if qEnd <= qStart {
		return
	}
	mods := uint32(0)
	if a.IsNegative {
		mods |= modifierNegative
	}
	emit(offsetSpan(a.Span.File, qStart, qEnd), semAmount, mods)
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

type semEmitFunc func(tok token.Span, tokKind, modifier uint32)

func emitBlockComments(cs []*ast.Comment, emit semEmitFunc) {
	for _, c := range cs {
		emitComment(c, emit)
	}
}

func emitComment(c *ast.Comment, emit semEmitFunc) {
	if c == nil {
		return
	}
	if len(c.Tags) == 0 {
		emit(c.Span, semComment, 0)
		return
	}
	pos := c.Span.Start.Offset
	for _, t := range c.Tags {
		if t.Span.Start.Offset > pos {
			emit(offsetSpan(c.Span.File, pos, t.Span.Start.Offset), semComment, 0)
		}
		emit(t.Span, semProperty, 0)
		pos = t.Span.End.Offset
	}
	if pos < c.Span.End.Offset {
		emit(offsetSpan(c.Span.File, pos, c.Span.End.Offset), semComment, 0)
	}
}

func emitDirective(content string, e token.Span, kwLen int, valType uint32, comment *ast.Comment, emit semEmitFunc) {
	kwEnd := e.Start.Offset + kwLen
	emit(token.Span{File: e.File, Start: e.Start, End: token.Pos{Offset: kwEnd}}, semDirective, 0)
	if v, ok := directiveValue(content, e, comment, kwEnd); ok {
		emit(v, valType, 0)
	}
	emitComment(comment, emit)
}

func semEmitCost(content string, c *ast.Cost, emit semEmitFunc) {
	if c.IsTotal {
		emit(token.Span{File: c.Span.File, Start: c.Span.Start, End: token.Pos{Offset: c.Span.Start.Offset + 2}}, semOperator, 0)
	} else {
		emit(token.Span{File: c.Span.File, Start: c.Span.Start, End: token.Pos{Offset: c.Span.Start.Offset + 1}}, semOperator, 0)
	}
	semEmitAmount(content, &c.Amount, emit)
}

func semEmitBalanceAssertion(content string, ba *ast.BalanceAssertion, emit semEmitFunc) {
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
	emit(token.Span{File: ba.Span.File, Start: token.Pos{Offset: opStart}, End: token.Pos{Offset: opEnd}}, semOperator, 0)
	semEmitAmount(content, &ba.Amount, emit)
	if ba.Cost != nil {
		semEmitCost(content, ba.Cost, emit)
	}
}

func semLexerFallback(content string, base []rawSpan, emit semEmitFunc) {
	l := lexer.New("", []byte(content))

	var commentStart, commentEnd int // 0 = not inside a comment line
	lineStart := true                // the next significant token starts a line
	skipLine := false                // the line starts with an unclassifiable token; emit nothing
	i := 0                           // next base span to compare against

	take := func(span token.Span, tokType uint32, mods uint32) {
		for i < len(base) && base[i].span.End.Offset <= span.Start.Offset {
			i++
		}
		if i < len(base) && base[i].span.Start.Offset < span.End.Offset {
			return // overlaps an AST token; AST wins
		}
		emit(span, tokType, mods)
	}

	for {
		tok := l.Next()
		if tok.Type == token.EOF {
			if commentStart > 0 {
				take(token.Span{Start: token.Pos{Offset: commentStart}, End: token.Pos{Offset: commentEnd}}, semComment, 0)
			}
			break
		}
		if tok.Type == token.NEWLINE {
			if commentStart > 0 {
				take(token.Span{Start: token.Pos{Offset: commentStart}, End: token.Pos{Offset: commentEnd}}, semComment, 0)
				commentStart, commentEnd = 0, 0
			}
			lineStart, skipLine = true, false
			continue
		}
		if tok.Type == token.WHITESPACE || tok.Type == token.INDENT {
			continue
		}
		if lineStart {
			lineStart = false
			if !isLineStartToken(tok.Type) {
				skipLine = true
			}
		}
		if skipLine {
			continue
		}
		tokType := semProperty
		if commentStart > 0 {
			tokType = semComment
			if tok.Span.End.Offset > commentEnd {
				commentEnd = tok.Span.End.Offset
			}
			continue
		}

		switch tok.Type {
		case token.SEMICOLON, token.HASH, token.PERCENT, token.STAR:
			tokType = semComment
			commentStart = tok.Span.Start.Offset
			commentEnd = tok.Span.End.Offset
			continue
		case token.STRING:
			tokType = semString
		case token.DATE:
			tokType = semDate
		case token.INT, token.DECIMAL:
			tokType = semAmount
		case token.COMMODITYMARK:
			tokType = semCommodity
		case token.BANG:
			tokType = semStatus
		case token.AT, token.ATAT, token.EQ, token.EQEQ, token.EQEQEQ, token.EQSTAR:
			tokType = semOperator
		case token.COMMENTKW, token.ACCOUNT, token.COMMODITY, token.INCLUDE,
			token.ALIAS, token.PAYEE, token.TAG, token.APPLY, token.END,
			token.YEAR, token.DECIMALMARK, token.D, token.P, token.N, token.C:
			tokType = semDirective
		}
		take(tok.Span, tokType, 0)
	}
}

func isLineStartToken(t token.Type) bool {
	switch t {
	case token.DATE, token.TILDE, token.EQ, token.BANG, token.AT,
		token.SEMICOLON, token.HASH, token.PERCENT, token.STAR,
		token.COMMENTKW, token.ACCOUNT, token.COMMODITY, token.INCLUDE,
		token.ALIAS, token.PAYEE, token.TAG, token.APPLY, token.END,
		token.YEAR, token.DECIMALMARK, token.D, token.P, token.N, token.C:
		return true
	}
	return false
}

// semanticTokensEdits returns the single edit turning old into new, or nil when
// identical. Relative delta encoding keeps the common prefix and suffix unchanged.
func semanticTokensEdits(old, new []uint32) []protocol.SemanticTokensEdit {
	p := 0
	for p < len(old) && p < len(new) && old[p] == new[p] {
		p++
	}
	s := 0
	for s < len(old)-p && s < len(new)-p && old[len(old)-1-s] == new[len(new)-1-s] {
		s++
	}
	delCount := len(old) - p - s
	ins := new[p : len(new)-s]
	if delCount == 0 && len(ins) == 0 {
		return nil
	}
	return []protocol.SemanticTokensEdit{{
		Start:       uint32(p),
		DeleteCount: uint32(delCount),
		Data:        ins,
	}}
}

// encodeSemTokens encodes tokens into LSP delta form. Input must be sorted by
// line and column; [rawToSemanticTokens] produces such order.
func encodeSemTokens(tokens []semanticToken) []uint32 {
	if len(tokens) == 0 {
		return nil
	}
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
	return offsetSpan(file, start, end), true
}

func offsetSpan(file string, start, end int) token.Span {
	return token.Span{File: file, Start: token.Pos{Offset: start}, End: token.Pos{Offset: end}}
}
