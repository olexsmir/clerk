package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/token"
)

func (s *server) DocumentSymbol(_ context.Context, params *protocol.DocumentSymbolParams) (protocol.DocumentSymbolResult, error) {
	an := s.analysisFor(params.TextDocument.URI)
	if an == nil {
		return nil, nil
	}
	pf := parsedFileFor(an, params.TextDocument.URI.Path())
	if pf == nil {
		return nil, nil
	}
	symbols := make([]protocol.DocumentSymbol, 0, len(pf.Ast.Entries)/2)
	for _, entry := range pf.Ast.Entries {
		if symbol, ok := entryDocumentSymbol(entry, pf.Src); ok {
			symbols = append(symbols, symbol)
		}
	}
	return protocol.DocumentSymbolSlice(symbols), nil
}

func entryDocumentSymbol(e ast.Entry, src []byte) (protocol.DocumentSymbol, bool) {
	var kind symbolKind
	var name string
	var sel, whole token.Span
	switch e := e.(type) {
	case *ast.Transaction:
		kind, name, sel, whole = symbolTransaction, transactionName(e), e.Date.Span, e.Span
	case *ast.PeriodicTransaction:
		kind, name, sel, whole = symbolTransaction, transactionName(e), e.Period.Span, e.Span
	case *ast.AutomatedTransaction:
		kind, name, sel, whole = symbolTransaction, transactionName(e), e.Expr.Span, e.Span
	case *ast.AccountDirective:
		kind, name, sel, whole = symbolAccount, e.Account.String(), e.Account.Span, e.Span
	case *ast.CommodityDirective:
		kind, name, sel, whole = symbolCommodity, e.Commodity, e.CommoditySpan, e.Span
	case *ast.PayeeDirective:
		if e.Name == nil {
			return protocol.DocumentSymbol{}, false
		}
		kind, name, sel, whole = symbolPayee, e.Name.Name, e.Name.Span, e.Span
	case *ast.TagDirective:
		sp, ok := tagDirectiveSpan(string(src), e)
		if !ok {
			return protocol.DocumentSymbol{}, false
		}
		kind, name, sel, whole = symbolTag, e.Name, sp, e.Span
	default:
		return protocol.DocumentSymbol{}, false
	}
	return protocol.DocumentSymbol{
		Name:           name,
		Kind:           kind.ToProtocol(),
		Range:          spanRangeFromSrc(src, whole),
		SelectionRange: spanRangeFromSrc(src, sel),
	}, true
}
