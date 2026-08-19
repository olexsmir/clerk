package lsp

import (
	"context"
	"sort"
	"strings"

	"go.lsp.dev/protocol"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/lsp/fuzzy"
	"olexsmir.xyz/clerk/journal/ast"
)

func (s *server) Symbols(_ context.Context, params *protocol.WorkspaceSymbolParams) (protocol.WorkspaceSymbolResult, error) {
	if params.Query == "" {
		return nil, nil
	}

	s.mu.RLock()
	paths := make([]string, 0, len(s.openDocs))
	for u := range s.openDocs {
		paths = append(paths, u.Path())
	}
	s.mu.RUnlock()
	if len(paths) == 0 {
		return nil, nil
	}

	an := analyzer.Build(s.loader.ResolveFiles(paths))
	symbols := searchSymbols(an, params.Query)
	if len(symbols) == 0 {
		return nil, nil
	}
	return protocol.WorkspaceSymbolSlice(symbols), nil
}

const maxSymbolResults = 100

type scoredSymbol struct {
	kind     symbolKind
	name     string
	score    float64
	tnxEntry ast.Entry // set if kind == symbolTransaction, used for location resolution
}

func searchSymbols(an *analyzer.Analysis, query string) []protocol.WorkspaceSymbol {
	matcher := fuzzy.Compile(query)
	scored := make([]scoredSymbol, 0)
	add := func(s scoredSymbol) {
		if score := matcher.Score(s.name); score > 0 {
			s.score = score
			scored = append(scored, s)
		}
	}
	for _, name := range an.AccountNames {
		add(scoredSymbol{kind: symbolAccount, name: name})
	}
	for name := range an.Commodities {
		add(scoredSymbol{kind: symbolCommodity, name: name})
	}
	for _, name := range an.PayeeNames {
		add(scoredSymbol{kind: symbolPayee, name: name})
	}
	for _, name := range an.TagNames {
		add(scoredSymbol{kind: symbolTag, name: name})
	}
	for _, tx := range an.Transactions {
		add(scoredSymbol{kind: symbolTransaction, name: transactionName(tx), tnxEntry: tx})
	}
	for _, tx := range an.PeriodicTransactions {
		add(scoredSymbol{kind: symbolTransaction, name: transactionName(tx), tnxEntry: tx})
	}
	for _, tx := range an.AutomatedTransactions {
		add(scoredSymbol{kind: symbolTransaction, name: transactionName(tx), tnxEntry: tx})
	}

	sortScoredSymbols(scored)
	if len(scored) > maxSymbolResults {
		scored = scored[:maxSymbolResults]
	}

	symbols := make([]protocol.WorkspaceSymbol, 0, len(scored))
	for _, s := range scored {
		loc := definitionLocation(an, s)
		if loc == nil {
			continue
		}
		symbols = append(symbols, protocol.WorkspaceSymbol{
			Name:     s.name,
			Kind:     s.kind.ToProtocol(),
			Location: loc,
		})
	}
	return symbols
}

func definitionLocation(an *analyzer.Analysis, s scoredSymbol) *protocol.Location {
	switch s.kind {
	case symbolAccount:
		return findAccountDefinition(an, s.name)
	case symbolTransaction:
		return findTransactionDefinition(an, s.tnxEntry)
	case symbolCommodity:
		return findCommodityDefinition(an, s.name)
	case symbolPayee:
		return findPayeeDefinition(an, s.name)
	case symbolTag:
		return findTagDefinition(an, s.name)
	}
	return nil
}

func transactionName(e ast.Entry) string {
	var b strings.Builder
	switch e := e.(type) {
	case *ast.Transaction:
		b.Grow(24)
		b.WriteString(e.Date.String())
		if s := e.Status.Value.String(); s != "" {
			b.WriteByte(' ')
			b.WriteString(s)
		}
		if e.Payee != nil {
			b.WriteByte(' ')
			b.WriteString(e.Payee.Name)
		}
		if e.Note != nil {
			b.WriteString(" | ")
			b.WriteString(e.Note.Value)
		}
	case *ast.PeriodicTransaction:
		b.WriteByte('~')
		if s := e.Status.Value.String(); s != "" {
			b.WriteByte(' ')
			b.WriteString(s)
		}
		b.WriteByte(' ')
		b.WriteString(e.Period.Raw)
		if e.Description != nil {
			b.WriteString(" | ")
			b.WriteString(e.Description.Value)
		}
	case *ast.AutomatedTransaction:
		b.WriteByte('=')
		b.WriteByte(' ')
		b.WriteString(e.Expr.Value)
	}
	return b.String()
}

func sortScoredSymbols(scored []scoredSymbol) {
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if len(scored[i].name) != len(scored[j].name) {
			return len(scored[i].name) < len(scored[j].name)
		}
		return scored[i].name < scored[j].name
	})
}
