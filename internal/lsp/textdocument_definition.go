package lsp

import (
	"context"
	"slices"

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
	cursor := lsputil.Offset(state.text, int(params.Position.Line), int(params.Position.Character))
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
		Range: spanToProtocolRange(string(pf.Src), span),
	}
}

func spanToProtocolRange(content string, span token.Span) protocol.Range {
	return protocol.Range{
		Start: lsputil.Position(content, span.Start.Offset),
		End:   lsputil.Position(content, spanEndClamped(content, span.End.Offset)),
	}
}

func spanContains(content string, span token.Span, offset int) bool {
	if span.End.Offset <= span.Start.Offset {
		return false
	}
	end := spanEndClamped(content, span.End.Offset)
	return span.Start.Offset <= offset && offset <= end
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
