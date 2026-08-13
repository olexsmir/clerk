package lsp

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"go.lsp.dev/protocol"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/lsp/fuzzy"
	"olexsmir.xyz/clerk/internal/lsp/lsputil"
	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/lexer"
	"olexsmir.xyz/clerk/journal/token"
)

func (s *server) Completion(ctx context.Context, params *protocol.CompletionParams) (protocol.CompletionResult, error) {
	state, ok := s.getDocState(params.TextDocument.URI)
	if !ok {
		return &protocol.CompletionList{}, nil
	}
	cursor := state.lineIdx.Offset(int(params.Position.Line), int(params.Position.Character))
	if cursor > len(state.text) {
		return &protocol.CompletionList{}, nil
	}
	detectedCtx, start := detectCompletionCtx(state.text, cursor)
	if detectedCtx == cmplNone {
		return &protocol.CompletionList{}, nil
	}
	an := s.analysisFor(params.TextDocument.URI)
	if an == nil {
		return &protocol.CompletionList{}, nil
	}
	return &protocol.CompletionList{
		IsIncomplete: true,
		Items:        cmplItems(an, detectedCtx, state.text, state.lineIdx, start, cursor),
	}, nil
}

const maxCompletionItems = 50

type cmplCtx int

const (
	cmplNone cmplCtx = iota
	cmplAccount
	cmplPayee
	cmplCommodity
	cmplTagName
	cmplTagValue
	cmplDirective
)

var directiveKeywords = []string{
	"account", "include", "commodity", "payee", "decimal-mark", "alias",
	"apply", "end", "tag", "year", "D", "P", "N", "C", "Y",
}

func detectCompletionCtx(content string, cursor int) (cmplCtx, int) {
	toks := lexLine(content, cursor)
	lineStart, _ := lineBounds(content, cursor)

	if m := commentMarker(toks, cursor); m != -1 {
		return cmplTagContext(content, toks[m].Span.End.Offset, cursor)
	}
	if len(toks) == 0 {
		return cmplDirective, lineStart
	}

	switch toks[0].Type {
	case token.INDENT:
		return cmplPostingCtx(content, cursor, toks)
	case token.DATE:
		return cmplHeaderCtx(content, cursor, toks)
	case token.ACCOUNT, token.COMMODITY, token.PAYEE, token.TAG:
		return cmplDirectiveContext(cursor, lineStart, toks)
	case token.TEXT:
		return cmplDirective, lineStart // half-typed keyword or unparseable line
	}
	return cmplNone, cursor
}

func cmplPostingCtx(content string, cursor int, toks []token.Token) (cmplCtx, int) {
	if inDirectiveBody(content, cursor) {
		return cmplNone, cursor
	}
	fieldStart := toks[0].Span.End.Offset
	i := 1
	for i < len(toks) {
		switch toks[i].Type {
		case token.STAR, token.BANG, token.LPAREN, token.LBRACKET, token.WHITESPACE:
			fieldStart = toks[i].Span.End.Offset
			i++
		default:
			goto run
		}
	}
run:
	// account run: consecutive account-name segments and colons
	fieldEnd := fieldStart
	for ; i < len(toks); i++ {
		if toks[i].Type != token.TEXT && toks[i].Type != token.COLON {
			break
		}
		fieldEnd = toks[i].Span.End.Offset
	}
	if cursor <= fieldEnd && cursor >= fieldStart {
		return cmplAccount, fieldStart
	}
	if cursor > fieldEnd {
		if t := tokenUnder(toks, cursor); t != nil && (t.Type == token.COMMODITYMARK || t.Type == token.STRING) {
			start := t.Span.Start.Offset
			if t.Type == token.STRING {
				start++ // skip opening quote
			}
			return cmplCommodity, start
		}
		if strings.TrimSpace(content[fieldEnd:cursor]) == "" {
			return cmplCommodity, cursor
		}
	}
	return cmplNone, cursor
}

func cmplHeaderCtx(content string, cursor int, toks []token.Token) (cmplCtx, int) {
	// skip date, status, code, and whitespace - where the payee beginds
	fieldStart := toks[0].Span.End.Offset
	fieldEnd := fieldStart
	seen := false
	for i := 1; i < len(toks); i++ {
		t := toks[i]
		switch t.Type {
		case token.WHITESPACE, token.STAR, token.BANG, token.DATE, token.TIME,
			token.EQ, token.EQEQ, token.EQEQEQ:
			fieldStart = t.Span.End.Offset
			fieldEnd = t.Span.End.Offset
		case token.TEXT:
			lit := content[t.Span.Start.Offset:t.Span.End.Offset]
			if !seen && len(lit) >= 2 && lit[0] == '(' && lit[len(lit)-1] == ')' {
				fieldStart = t.Span.End.Offset // parenthesized code
				fieldEnd = t.Span.End.Offset
				continue
			}
			if !seen {
				fieldStart = t.Span.Start.Offset
				seen = true
			}
			fieldEnd = t.Span.End.Offset
			if p := strings.IndexByte(lit, '|'); p >= 0 {
				fieldEnd = t.Span.Start.Offset + p // "payee|note" keeps the pipe in the token
				return payeeAt(cursor, fieldStart, fieldEnd)
			}
		case token.STRING:
			if !seen {
				fieldStart = t.Span.Start.Offset + 1 // skip opening quote
				seen = true
			}
			fieldEnd = t.Span.End.Offset
		default:
			// PIPE, SEMICOLON, ...
			return payeeAt(cursor, fieldStart, fieldEnd)
		}
	}
	if seen {
		return payeeAt(cursor, fieldStart, fieldEnd)
	}
	// no payee yet: the payee field is the whitespace after the header meta
	if cursor >= fieldStart && strings.TrimSpace(content[fieldStart:cursor]) == "" {
		return cmplPayee, cursor
	}
	return cmplNone, cursor
}

func payeeAt(cursor, start, end int) (cmplCtx, int) {
	if cursor >= start && cursor <= end {
		return cmplPayee, start
	}
	return cmplNone, cursor
}

// cmplDirectiveContext classifies a directive line. keyword completion before the keyword ends, symbol completion in the value field after
func cmplDirectiveContext(cursor, lineStart int, toks []token.Token) (cmplCtx, int) {
	kwEnd := toks[0].Span.End.Offset
	if cursor <= kwEnd {
		return cmplDirective, lineStart
	}
	start := kwEnd
	for i := 1; i < len(toks); i++ {
		if toks[i].Type == token.WHITESPACE || toks[i].Span.End.Offset <= kwEnd {
			continue
		}
		start = toks[i].Span.Start.Offset
		if toks[i].Type == token.STRING {
			start++ // skip opening quote
		}
		break
	}
	if start > cursor {
		start = cursor
	}
	switch toks[0].Type {
	case token.ACCOUNT:
		return cmplAccount, start
	case token.COMMODITY:
		return cmplCommodity, start
	case token.PAYEE:
		return cmplPayee, start
	case token.TAG:
		return cmplTagName, start
	}
	return cmplNone, cursor
}

// commentStart completes tag names before ':' of the current tag and tag values after it
func cmplTagContext(content string, commentStart, cursor int) (cmplCtx, int) {
	prefix := content[commentStart:cursor]
	segStart := commentStart
	seg := prefix
	if comma := strings.LastIndexByte(prefix, ','); comma >= 0 {
		segStart = commentStart + comma + 1
		seg = prefix[comma+1:]
	}
	if colon := strings.IndexByte(seg, ':'); colon >= 0 {
		start := segStart + colon + 1
		for start < cursor && (content[start] == ' ' || content[start] == '\t') {
			start++
		}
		return cmplTagValue, start
	}
	keyStart := commentStart + lastSeparator(prefix) + 1
	return cmplTagName, keyStart
}

// tagKeyAt returns the key of tag whose value region starts at start
func tagKeyAt(content string, start int) (string, bool) {
	lineStart, _ := lineBounds(content, start)
	segStart := lineStart
	for i := start - 1; i >= lineStart; i-- {
		switch content[i] {
		case ',', ';', '#', '%':
			segStart = i + 1
			i = lineStart - 1 // stop at the separator closest to start
		}
	}
	colon := strings.IndexByte(content[segStart:start], ':')
	if colon < 0 {
		return "", false
	}
	colon += segStart
	keyStart := lastSeparator(content[segStart:colon]) + 1
	key := content[segStart+keyStart : colon]
	if key == "" {
		return "", false
	}
	return key, true
}

// commentMarker returns index of the first comment marker token at or before the cursor, or -1.
func commentMarker(toks []token.Token, cursor int) int {
	for i, t := range toks {
		switch t.Type {
		case token.SEMICOLON, token.HASH, token.PERCENT:
			if t.Span.Start.Offset <= cursor {
				return i
			}
		case token.STAR:
			if i == 0 && t.Span.Start.Offset <= cursor {
				return i
			}
		}
	}
	return -1
}

func inDirectiveBody(content string, cursor int) bool {
	lineStart, _ := lineBounds(content, cursor)
	if toks := lexLine(content, lineStart); len(toks) == 0 || toks[0].Type != token.INDENT {
		return false
	}
	for lineStart > 0 {
		lineStart, _ = lineBounds(content, lineStart-1)
		toks := lexLine(content, lineStart)
		if len(toks) == 0 || toks[0].Type != token.INDENT {
			return len(toks) > 0 && (toks[0].Type == token.ACCOUNT || toks[0].Type == token.COMMODITY)
		}
	}
	return false
}

type cmplCand struct {
	label        string
	score        float64
	count        int
	lastUsedDays int64 // days since 1970-01-01; 0 when unset
}

// cmplItems ranks candidates for the content against typed pattern
func cmplItems(a *analyzer.Analysis, ctx cmplCtx, content string, li *lsputil.LineIndex, start, cursor int) []protocol.CompletionItem {
	pattern := content[start:cursor]

	var kind protocol.CompletionItemKind
	var cands []cmplCand
	switch ctx {
	case cmplAccount:
		kind = protocol.CompletionItemKindClass
		cands = make([]cmplCand, 0, len(a.Accounts))
		for name, info := range a.Accounts {
			cands = append(cands, cmplCand{label: name, count: info.UsedCount, lastUsedDays: dateToDays(info.LastUsed)})
		}
	case cmplPayee:
		kind = protocol.CompletionItemKindVariable
		cands = make([]cmplCand, 0, len(a.Payees))
		for name, info := range a.Payees {
			cands = append(cands, cmplCand{label: name, count: info.UsedCount, lastUsedDays: dateToDays(info.LastUsed)})
		}
	case cmplCommodity:
		kind = protocol.CompletionItemKindValue
		cands = make([]cmplCand, 0, len(a.Commodities))
		for name, info := range a.Commodities {
			cands = append(cands, cmplCand{label: name, count: info.UsedCount, lastUsedDays: dateToDays(info.LastUsed)})
		}
	case cmplTagName:
		kind = protocol.CompletionItemKindProperty
		cands = make([]cmplCand, 0, len(a.Tags))
		for name, info := range a.Tags {
			cands = append(cands, cmplCand{label: name, count: info.UsedCount, lastUsedDays: dateToDays(info.LastUsed)})
		}
	case cmplTagValue:
		kind = protocol.CompletionItemKindProperty
		if key, ok := tagKeyAt(content, start); ok {
			if info, ok := a.Tags[key]; ok {
				for _, v := range info.Values {
					cands = append(cands, cmplCand{label: v})
				}
			}
		}
	case cmplDirective:
		kind = protocol.CompletionItemKindKeyword
		for _, name := range directiveKeywords {
			cands = append(cands, cmplCand{label: name})
		}
	default:
		return nil
	}

	var newest int64 // days since epoch of the newest transaction; 0 when none
	if n := len(a.Dates); n > 0 {
		newest = dateToDays(a.Dates[n-1])
	}
	m := fuzzy.Compile(pattern)
	ranked := cands[:0]
	for i := range cands {
		sc := m.Score(cands[i].label)
		if sc != 0 {
			sc *= 1 + math.Log1p(float64(cands[i].count))
			if cands[i].count > 0 && cands[i].lastUsedDays != 0 && newest != 0 {
				days := newest - cands[i].lastUsedDays
				sc *= 1 + 0.5*max(0, 1-float64(days)/365)
			}
		}
		cands[i].score = sc
		if sc != 0 {
			ranked = append(ranked, cands[i])
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if ranked[i].lastUsedDays != ranked[j].lastUsedDays {
			return ranked[i].lastUsedDays > ranked[j].lastUsedDays
		}
		return ranked[i].label < ranked[j].label
	})
	if len(ranked) > maxCompletionItems {
		ranked = ranked[:maxCompletionItems]
	}

	replace := protocol.Range{
		Start: li.Position(start),
		End:   li.Position(cursor),
	}
	items := make([]protocol.CompletionItem, len(ranked))
	for i, r := range ranked {
		it := protocol.CompletionItem{
			Label:      r.label,
			Kind:       kind,
			SortText:   protocol.NewOptional(fmt.Sprintf("%04d", i)),
			FilterText: protocol.NewOptional(r.label),
			TextEdit: &protocol.TextEdit{
				Range:   replace,
				NewText: r.label,
			},
		}
		if r.count > 0 {
			it.Detail = protocol.NewOptional(fmt.Sprintf("%d uses", r.count))
		}
		items[i] = it
	}
	return items
}

// dateToDays converts a date to days since 1970-01-01; zero dates map to 0.
func dateToDays(d ast.Date) int64 {
	if d.Year == 0 {
		return 0
	}
	return daysFromCivil(d.Year, d.Month, d.Day)
}

// daysFromCivil converts a proleptic Gregorian date to days since 1970-01-01
// (Howard Hinnant's algorithm).
func daysFromCivil(y, m, d int) int64 {
	if m <= 2 {
		y--
	}
	era := y / 400
	yoe := y - era*400
	mp := (m + 9) % 12
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return int64(era)*146097 + int64(doe) - 719468
}

// lineBounds returns the byte offsets of the line containing cursor
func lineBounds(content string, cursor int) (start, end int) {
	start = cursor
	for start > 0 && content[start-1] != '\n' && content[start-1] != '\r' {
		start--
	}
	end = start
	for end < len(content) && content[end] != '\n' && content[end] != '\r' {
		end++
	}
	return start, end
}

func lastSeparator(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i] {
		case ' ', '\t', ',':
			return i
		}
	}
	return -1
}

func lexLine(content string, cursor int) []token.Token {
	lineStart, lineEnd := lineBounds(content, cursor)
	l := lexer.New("", []byte(content[lineStart:lineEnd]))
	var out []token.Token
	for {
		t := l.Next()
		if t.Type == token.EOF || t.Type == token.NEWLINE {
			break
		}
		t.Span.Start.Offset += lineStart
		t.Span.End.Offset += lineStart
		out = append(out, t)
	}
	return out
}

func tokenUnder(toks []token.Token, cursor int) *token.Token {
	for i := range toks {
		t := &toks[i]
		if t.Span.Start.Offset <= cursor && cursor <= t.Span.End.Offset {
			return t
		}
	}
	return nil
}
