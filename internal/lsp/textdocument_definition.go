package lsp

import (
	"context"
	"slices"
	"sort"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/lsp/lsputil"
	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/token"
)

func (s *server) Definition(_ context.Context, params *protocol.DefinitionParams) (protocol.DefinitionResult, error) {
	state, ok := s.getDocState(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	an := s.analysis()
	cursor := state.lineIdx.Offset(int(params.Position.Line), int(params.Position.Character))
	return findDefinitionUnderCursor(an, params.TextDocument.URI.Path(), state.text, cursor), nil
}

func findDefinitionUnderCursor(an *analyzer.Analysis, docPath, content string, cursor int) protocol.LocationSlice {
	ref := findSymbolUnderCursor(an, docPath, content, cursor)
	if ref == nil {
		return nil
	}
	loc := resolveSymbol(ref, an)
	if loc == nil {
		return nil
	}
	return protocol.LocationSlice{*loc}
}

func resolveSymbol(ref *symbolRef, an *analyzer.Analysis) *protocol.Location {
	switch ref.kind {
	case symbolAccount:
		return findAccountDefinition(an, ref.name)
	case symbolCommodity:
		return findCommodityDefinition(an, ref.name)
	case symbolPayee:
		return findPayeeDefinition(an, ref.name)
	}
	return nil
}

func findAccountDefinition(an *analyzer.Analysis, name string) *protocol.Location {
	if canon, ok := an.AccountAliases[name]; ok {
		name = canon
	}
	info := an.Accounts[name]
	if info == nil {
		return nil
	}
	if len(info.Directives) > 0 {
		return locationForDirective(an, info.Directives[0], info.Directives[0].Account.Span)
	}
	if len(info.Usages) > 0 {
		u := info.Usages[0]
		return locationFor(an, u.FileIndex, u.Posting.Account.Span)
	}
	return nil
}

func findCommodityDefinition(an *analyzer.Analysis, symbol string) *protocol.Location {
	info := an.Commodities[symbol]
	if info == nil {
		return nil
	}
	if len(info.Directives) > 0 {
		return locationForDirective(an, info.Directives[0], info.Directives[0].CommoditySpan)
	}
	if len(info.Usages) > 0 {
		u := info.Usages[0]
		return locationFor(an, u.FileIndex, u.Amount.CommoditySpan)
	}
	return nil
}

func findPayeeDefinition(an *analyzer.Analysis, name string) *protocol.Location {
	info := an.Payees[name]
	if info == nil {
		return nil
	}
	if len(info.Directives) > 0 {
		d := info.Directives[0]
		if d.Name == nil {
			return nil
		}
		return locationForDirective(an, d, d.Name.Span)
	}
	if len(info.Usage) > 0 {
		u := info.Usage[0]
		return locationFor(an, u.FileIndex, u.Payee.Span)
	}
	return nil
}

func locationForDirective(a *analyzer.Analysis, d ast.Entry, span token.Span) *protocol.Location {
	for i, pf := range a.Files {
		if slices.Contains(pf.Ast.Entries, d) {
			return locationFor(a, i, span)
		}
	}
	return nil
}

func locationFor(a *analyzer.Analysis, fileIdx int, span token.Span) *protocol.Location {
	pf := a.Files[fileIdx]
	return &protocol.Location{
		URI:   uri.File(pf.Path),
		Range: spanRangeFromSrc(pf.Src, span),
	}
}

// spanRangeFromSrc converts a span to a protocol range. Parsed spans carry
// 1-based Line/Col and are converted directly; spans whose end runs into
// trailing whitespace or uses the next-line-start convention (Col == 0) get a
// one-line scan back from the end offset.
func spanRangeFromSrc(src []byte, span token.Span) protocol.Range {
	if span.Start.Line == 0 || span.End.Line == 0 {
		// spans built from offsets without Line/Col: full line index
		return lsputil.NewLineIndex(string(src)).SpanRange(span)
	}
	start := protocol.Position{Line: uint32(span.Start.Line - 1), Character: uint32(span.Start.Col - 1)}
	end := span.End.Offset
	if span.End.Col > 0 && end > span.Start.Offset && !isSpanSpace(src[end-1]) {
		// the span's stored end position matches its offset
		return protocol.Range{Start: start, End: protocol.Position{Line: uint32(span.End.Line - 1), Character: uint32(span.End.Col - 1)}}
	}
	// Trim trailing whitespace back from the end offset; both scans are
	// bounded by the one line the span ends on.
	line := span.End.Line - 1 // 1-based line holding the end
	for end > span.Start.Offset && isSpanSpace(src[end-1]) {
		if src[end-1] == '\n' {
			line--
		}
		end--
	}
	lineStart := end
	for lineStart > 0 && src[lineStart-1] != '\n' {
		lineStart--
	}
	return protocol.Range{
		Start: start,
		End: protocol.Position{
			Line:      uint32(line - 1),
			Character: uint32(lsputil.Utf16ColBytes(src[lineStart:end])),
		},
	}
}

func isSpanSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n':
		return true
	}
	return false
}

func spanContains(content string, span token.Span, offset int) bool {
	if span.End.Offset <= span.Start.Offset {
		return false
	}
	end := spanEndClamped(content, span.End.Offset)
	return span.Start.Offset <= offset && offset <= end
}

// entryAt returns the entry whose start offset is at or before cursor, the
// only entry whose tokens can contain it. Entries are stored in file order,
// so a binary search replaces a linear scan for late-cursor requests.
func entryAt(entries []ast.Entry, cursor int) ast.Entry {
	idx := sort.Search(len(entries), func(i int) bool { return entryStart(entries[i]) > cursor }) - 1
	if idx < 0 {
		return nil
	}
	return entries[idx]
}

func entryStart(e ast.Entry) int {
	switch e := e.(type) {
	case *ast.BlankLine:
		return e.Span.Start.Offset
	case *ast.Transaction:
		return e.Span.Start.Offset
	case *ast.PeriodicTransaction:
		return e.Span.Start.Offset
	case *ast.AutomatedTransaction:
		return e.Span.Start.Offset
	case *ast.Comment:
		return e.Span.Start.Offset
	case *ast.AccountDirective:
		return e.Span.Start.Offset
	case *ast.CommodityDirective:
		return e.Span.Start.Offset
	case *ast.PayeeDirective:
		return e.Span.Start.Offset
	case *ast.TagDirective:
		return e.Span.Start.Offset
	case *ast.IncludeDirective:
		return e.Span.Start.Offset
	case *ast.AliasDirective:
		return e.Span.Start.Offset
	case *ast.YearDirective:
		return e.Span.Start.Offset
	case *ast.DecimalMarkDirective:
		return e.Span.Start.Offset
	case *ast.DefaultCommodityDirective:
		return e.Span.Start.Offset
	case *ast.MarketPriceDirective:
		return e.Span.Start.Offset
	case *ast.ConversionDirective:
		return e.Span.Start.Offset
	case *ast.ApplyDirective:
		return e.Span.Start.Offset
	case *ast.EndDirective:
		return e.Span.Start.Offset
	case *ast.CommentBlockDirective:
		return e.Span.Start.Offset
	case *ast.IgnoredDirective:
		return e.Span.Start.Offset
	}
	return 0
}

func spanEndClamped(content string, end int) int {
	for end > 0 {
		switch content[end-1] {
		case ' ', '\t', '\r', '\n':
			end--
		default:
			return end
		}
	}
	return end
}
