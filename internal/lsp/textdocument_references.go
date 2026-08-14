package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"olexsmir.xyz/clerk/internal/analyzer"
)

func (s *server) References(_ context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	state, ok := s.getDocState(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	an := s.analysisFor(params.TextDocument.URI)
	if an == nil {
		return nil, nil
	}
	cursor := state.lineIdx.Offset(int(params.Position.Line), int(params.Position.Character))
	ref := findSymbolUnderCursor(an, params.TextDocument.URI.Path(), state.text, cursor)
	if ref == nil {
		return nil, nil
	}
	return findReferences(an, ref, params.Context.IncludeDeclaration), nil
}

func findReferences(an *analyzer.Analysis, ref *symbolRef, includeDeclaration bool) []protocol.Location {
	switch ref.kind {
	case symbolAccount:
		return findAccountReferences(an, ref.name, includeDeclaration)
	case symbolCommodity:
		return findCommodityReferences(an, ref.name, includeDeclaration)
	case symbolPayee:
		return findPayeeReferences(an, ref.name, includeDeclaration)
	case symbolTag:
		return findTagReferences(an, ref.name, includeDeclaration)
	}
	return nil
}

func findAccountReferences(an *analyzer.Analysis, name string, includeDeclaration bool) []protocol.Location {
	var locations []protocol.Location
	for _, candidate := range an.AccountNames {
		if !accountMatches(candidate, name) {
			continue
		}
		info := an.Accounts[candidate]
		for _, u := range info.Usages {
			appendLocation(&locations, locationFor(an, u.FileIndex, u.Posting.Account.Span))
		}
		if includeDeclaration {
			for _, d := range info.Directives {
				appendLocation(&locations, locationForDirective(an, d, d.Account.Span))
			}
		}
	}
	return locations
}

func findCommodityReferences(an *analyzer.Analysis, symbol string, includeDeclaration bool) []protocol.Location {
	info := an.Commodities[symbol]
	if info == nil {
		return nil
	}
	var locations []protocol.Location
	for _, u := range info.Usages {
		appendLocation(&locations, locationFor(an, u.FileIndex, u.Amount.CommoditySpan))
	}
	if includeDeclaration {
		for _, d := range info.Directives {
			appendLocation(&locations, locationForDirective(an, d, d.CommoditySpan))
		}
	}
	return locations
}

func findPayeeReferences(an *analyzer.Analysis, name string, includeDeclaration bool) []protocol.Location {
	info := an.Payees[name]
	if info == nil {
		return nil
	}
	var locations []protocol.Location
	for _, u := range info.Usage {
		appendLocation(&locations, locationFor(an, u.FileIndex, u.Payee.Span))
	}
	if includeDeclaration {
		for _, d := range info.Directives {
			if d.Name != nil {
				appendLocation(&locations, locationForDirective(an, d, d.Name.Span))
			}
		}
	}
	return locations
}

func findTagReferences(an *analyzer.Analysis, key string, includeDeclaration bool) []protocol.Location {
	info := an.Tags[key]
	if info == nil {
		return nil
	}
	var locations []protocol.Location
	for _, u := range info.Usage {
		span := tagKeySpan(string(an.Files[u.FileIndex].Src), u.Tag)
		appendLocation(&locations, locationFor(an, u.FileIndex, span))
	}
	if includeDeclaration {
		for _, d := range info.Directives {
			fileIdx := fileIndexForEntry(an, d)
			if fileIdx < 0 {
				continue
			}
			if span, ok := tagDirectiveSpan(string(an.Files[fileIdx].Src), d); ok {
				appendLocation(&locations, locationFor(an, fileIdx, span))
			}
		}
	}
	return locations
}

func appendLocation(locations *[]protocol.Location, loc *protocol.Location) {
	if loc != nil {
		*locations = append(*locations, *loc)
	}
}
