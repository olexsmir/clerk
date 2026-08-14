package lsp

import (
	"context"
	"sort"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/lsp/fuzzy"
)

func (s *server) Symbols(_ context.Context, params *protocol.WorkspaceSymbolParams) (protocol.WorkspaceSymbolResult, error) {
	if params.Query == "" {
		return nil, nil
	}

	s.mu.Lock()
	var u uri.URI
	for u = range s.openDocs {
		break
	}
	s.mu.Unlock()
	an := s.analysisFor(u)
	if an == nil {
		return nil, nil
	}

	symbols := searchSymbols(an, params.Query)
	if len(symbols) == 0 {
		return nil, nil
	}
	return protocol.WorkspaceSymbolSlice(symbols), nil
}

const maxSymbolResults = 100

type scoredSymbol struct {
	kind  symbolKind
	name  string
	score float64
}

func searchSymbols(an *analyzer.Analysis, query string) []protocol.WorkspaceSymbol {
	matcher := fuzzy.Compile(query)

	scored := make([]scoredSymbol, 0, len(an.AccountNames)+len(an.Commodities)+len(an.PayeeNames)+len(an.TagNames))
	add := func(kind symbolKind, name string) {
		if score := matcher.Score(name); score > 0 {
			scored = append(scored, scoredSymbol{kind, name, score})
		}
	}
	for _, name := range an.AccountNames {
		add(symbolAccount, name)
	}
	for name := range an.Commodities {
		add(symbolCommodity, name)
	}
	for _, name := range an.PayeeNames {
		add(symbolPayee, name)
	}
	for _, name := range an.TagNames {
		add(symbolTag, name)
	}

	sortScoredSymbols(scored)
	if len(scored) > maxSymbolResults {
		scored = scored[:maxSymbolResults]
	}

	symbols := make([]protocol.WorkspaceSymbol, 0, len(scored))
	for _, s := range scored {
		loc := definitionLocation(an, s.kind, s.name)
		if loc == nil {
			continue
		}
		symbols = append(symbols, protocol.WorkspaceSymbol{
			BaseSymbolInformation: protocol.BaseSymbolInformation{
				Name: s.name,
				Kind: s.kind.ToProtocol(),
			},
			Location: loc,
		})
	}
	return symbols
}

func definitionLocation(an *analyzer.Analysis, kind symbolKind, name string) *protocol.Location {
	switch kind {
	case symbolAccount:
		return findAccountDefinition(an, name)
	case symbolCommodity:
		return findCommodityDefinition(an, name)
	case symbolPayee:
		return findPayeeDefinition(an, name)
	case symbolTag:
		return findTagDefinition(an, name)
	}
	return nil
}

func sortScoredSymbols(scored []scoredSymbol) {
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].name < scored[j].name
	})
}
