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
	for _, pf := range an.Files {
		if pf.Path != docPath {
			continue
		}
		for _, entry := range pf.Ast.Entries {
			if loc := definitionInEntry(an, content, entry, cursor); loc != nil {
				return protocol.LocationSlice{*loc}
			}
		}
		return nil
	}
	return nil
}

func definitionInEntry(an *analyzer.Analysis, content string, e ast.Entry, cursor int) *protocol.Location {
	switch e := e.(type) {
	case *ast.Transaction:
		if e.Payee != nil && spanContains(content, e.Payee.Span, cursor) {
			return findPayeeDefinition(an, e.Payee.Name)
		}
		return definitionInPostings(an, content, e.Postings, cursor)
	case *ast.PeriodicTransaction:
		return definitionInPostings(an, content, e.Postings, cursor)
	case *ast.AutomatedTransaction:
		return definitionInPostings(an, content, e.Postings, cursor)
	case *ast.AccountDirective:
		if spanContains(content, e.Account.Span, cursor) {
			return findAccountDefinition(an, e.Account.String())
		}
	case *ast.CommodityDirective:
		if spanContains(content, e.CommoditySpan, cursor) {
			return findCommodityDefinition(an, e.Commodity)
		}
	case *ast.PayeeDirective:
		if e.Name != nil && spanContains(content, e.Name.Span, cursor) {
			return findPayeeDefinition(an, e.Name.Name)
		}
	}
	return nil
}

func definitionInPostings(an *analyzer.Analysis, content string, postings []*ast.Posting, cursor int) *protocol.Location {
	for _, p := range postings {
		if spanContains(content, p.Account.Span, cursor) {
			return findAccountDefinition(an, p.Account.String())
		}
		if loc := commodityDefinition(an, content, p.Amount, cursor); loc != nil {
			return loc
		}
		if p.Cost != nil {
			if loc := commodityDefinition(an, content, &p.Cost.Amount, cursor); loc != nil {
				return loc
			}
		}
		if p.Balance != nil {
			if loc := commodityDefinition(an, content, &p.Balance.Amount, cursor); loc != nil {
				return loc
			}
		}
	}
	return nil
}

func commodityDefinition(an *analyzer.Analysis, content string, am *ast.Amount, cursor int) *protocol.Location {
	if am == nil || am.Commodity == "" || !spanContains(content, am.CommoditySpan, cursor) {
		return nil
	}
	return findCommodityDefinition(an, am.Commodity)
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
