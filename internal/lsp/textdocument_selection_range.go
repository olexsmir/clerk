package lsp

import (
	"context"

	"go.lsp.dev/protocol"

	"olexsmir.xyz/clerk/internal/lsp/lsputil"
	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/token"
)

func (s *server) SelectionRange(_ context.Context, params *protocol.SelectionRangeParams) ([]protocol.SelectionRange, error) {
	u := params.TextDocument.URI
	state, ok := s.getDocState(u)
	if !ok {
		return nil, nil
	}
	an := s.analysisFor(u)
	if an == nil {
		return nil, nil
	}
	pf := parsedFileFor(an, u.Path())
	if pf == nil {
		return nil, nil
	}

	li := state.lineIdx
	docSel := protocol.SelectionRange{Range: li.SpanRange(fullDocSpan(pf.Src))}
	out := make([]protocol.SelectionRange, len(params.Positions))
	for i, pos := range params.Positions {
		cursor := li.Offset(int(pos.Line), int(pos.Character))
		out[i] = selectionAt(state.text, pf, li, cursor, docSel)
	}
	return out, nil
}

func fullDocSpan(src []byte) token.Span { return token.Span{End: token.Pos{Offset: len(src)}} }

func selectionAt(content string, pf *journal.ParsedFile, li *lsputil.LineIndex, cursor int, docSel protocol.SelectionRange) protocol.SelectionRange {
	e := entryAt(pf.Ast.Entries, cursor)
	if e == nil {
		return docSel
	}
	es := entrySpan(e)
	if !spanContains(content, es, cursor) {
		return docSel
	}
	switch c := e.(type) {
	case *ast.BlankLine:
		return docSel
	case *ast.Comment: // the comment span is the entry span itself: no extra level
		return commentOrParent(content, c, li, cursor, docSel)
	}
	parent := protocol.SelectionRange{Range: li.SpanRange(es), Parent: &docSel}
	return selectionInEntry(content, e, li, cursor, parent)
}

func selectionInEntry(content string, e ast.Entry, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	switch t := e.(type) {
	case *ast.Transaction:
		return selTransaction(content, t, li, cursor, parent)
	case *ast.PeriodicTransaction:
		return selPeriodicTransaction(content, t, li, cursor, parent)
	case *ast.AutomatedTransaction:
		return selAutomatedTransaction(content, t, li, cursor, parent)
	case *ast.AccountDirective:
		return selAccountDirective(content, t, li, cursor, parent)
	case *ast.CommodityDirective:
		return selCommodityDirective(content, t, li, cursor, parent)
	case *ast.PayeeDirective:
		if t.Name != "" {
			if sel, ok := selForSpan(content, li, t.NameSpan, cursor, parent); ok {
				return sel
			}
		}
		return commentOrParent(content, t.Comment, li, cursor, parent)
	case *ast.TagDirective:
		if sp, ok := tagDirectiveSpan(content, t); ok {
			if sel, ok := selForSpan(content, li, sp, cursor, parent); ok {
				return sel
			}
		}
		return commentOrParent(content, t.Comment, li, cursor, parent)
	case *ast.AliasDirective:
		if sel, ok := selAccount(content, &t.From, li, cursor, parent); ok {
			return sel
		}
		if sel, ok := selAccount(content, &t.To, li, cursor, parent); ok {
			return sel
		}
		return commentOrParent(content, t.Comment, li, cursor, parent)
	case *ast.DefaultCommodityDirective:
		if sel, ok := selAmount(content, &t.Amount, li, cursor, parent); ok {
			return sel
		}
		return commentOrParent(content, t.Comment, li, cursor, parent)
	case *ast.MarketPriceDirective:
		if sel, ok := selForSpan(content, li, t.DateTime.Date.Span, cursor, parent); ok {
			return sel
		}
		if t.DateTime.Time != nil {
			if sel, ok := selForSpan(content, li, t.DateTime.Time.Span, cursor, parent); ok {
				return sel
			}
		}
		if sel, ok := selAmount(content, &t.Amount, li, cursor, parent); ok {
			return sel
		}
		return commentOrParent(content, t.Comment, li, cursor, parent)
	case *ast.ConversionDirective:
		if sel, ok := selAmount(content, &t.From, li, cursor, parent); ok {
			return sel
		}
		if sel, ok := selAmount(content, &t.To, li, cursor, parent); ok {
			return sel
		}
		return commentOrParent(content, t.Comment, li, cursor, parent)
	}
	return parent
}

func selTransaction(content string, t *ast.Transaction, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	var header [6]token.Span // date, second date, status, code, payee, note
	sps := append(header[:0], t.Date.Span)
	if t.SecondDate != nil {
		sps = append(sps, t.SecondDate.Span)
	}
	if t.Status != ast.StatusNone {
		sps = append(sps, t.StatusSpan)
	}
	if t.Code != "" {
		sps = append(sps, t.CodeSpan)
	}
	if t.Payee != "" {
		sps = append(sps, t.PayeeSpan)
	}
	if t.Note != "" {
		sps = append(sps, t.NoteSpan)
	}
	for _, sp := range sps {
		if sel, ok := selForSpan(content, li, sp, cursor, parent); ok {
			return sel
		}
	}
	return selCommentsAndPostings(content, t.Comment, t.HeaderComments, t.Postings, li, cursor, parent)
}

func selCommentsAndPostings(content string, inline *ast.Comment, headers []*ast.Comment, postings []ast.Posting, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	if sel, ok := selComment(content, inline, li, cursor, parent); ok {
		return sel
	}
	for _, c := range headers {
		if sel, ok := selComment(content, c, li, cursor, parent); ok {
			return sel
		}
	}
	if sel, ok := selPostings(content, postings, li, cursor, parent); ok {
		return sel
	}
	return parent
}

func selPeriodicTransaction(content string, pt *ast.PeriodicTransaction, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	if sel, ok := selForSpan(content, li, pt.Period.Span, cursor, parent); ok {
		if d := pt.Period.From; d != nil {
			if sub, ok := selForSpan(content, li, d.Span, cursor, sel); ok {
				return sub
			}
		}
		if d := pt.Period.To; d != nil {
			if sub, ok := selForSpan(content, li, d.Span, cursor, sel); ok {
				return sub
			}
		}
		return sel
	}
	if pt.Description != "" {
		if sel, ok := selForSpan(content, li, pt.DescriptionSpan, cursor, parent); ok {
			return sel
		}
	}
	return selCommentsAndPostings(content, pt.Comment, pt.HeaderComments, pt.Postings, li, cursor, parent)
}

func selAutomatedTransaction(content string, at *ast.AutomatedTransaction, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	if sel, ok := selForSpan(content, li, at.ExprSpan, cursor, parent); ok {
		return sel
	}
	return selCommentsAndPostings(content, at.Comment, at.HeaderComments, at.Postings, li, cursor, parent)
}

func selAccountDirective(content string, d *ast.AccountDirective, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	if sel, ok := selAccount(content, &d.Account, li, cursor, parent); ok {
		return sel
	}
	for i := range d.Subdirectives {
		sd := &d.Subdirectives[i]
		if sd.Kind == ast.SubdirectiveComment {
			if sel, ok := selComment(content, sd.Comment, li, cursor, parent); ok {
				return sel
			}
			continue
		}
		if sel, ok := selForSpan(content, li, sd.ValueSpan, cursor, parent); ok {
			return sel
		}
	}
	return commentOrParent(content, d.Comment, li, cursor, parent)
}

func selCommodityDirective(content string, d *ast.CommodityDirective, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	if sel, ok := selForSpan(content, li, d.CommoditySpan, cursor, parent); ok {
		return sel
	}
	if d.FormatSub != nil {
		if sel, ok := selAmount(content, &d.FormatSub.Amount, li, cursor, parent); ok {
			return sel
		}
	}
	return commentOrParent(content, d.Comment, li, cursor, parent)
}

func selPostings(content string, postings []ast.Posting, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) (protocol.SelectionRange, bool) {
	for i := range postings {
		p := &postings[i]
		postingSel, ok := selForSpan(content, li, p.Span, cursor, parent)
		if !ok {
			continue
		}
		return selPosting(content, postings[i], li, cursor, postingSel), true
	}
	return protocol.SelectionRange{}, false
}

func selPosting(content string, p ast.Posting, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	if p.Status != ast.StatusNone {
		if sel, ok := selForSpan(content, li, p.StatusSpan, cursor, parent); ok {
			return sel
		}
	}
	if sel, ok := selAccount(content, &p.Account, li, cursor, parent); ok {
		return sel
	}
	if sel, ok := selAmount(content, p.Amount, li, cursor, parent); ok {
		return sel
	}
	if p.Cost != nil {
		if sel, ok := selForSpan(content, li, p.Cost.Span, cursor, parent); ok {
			return sel
		}
	}
	if p.Balance != nil {
		if sel, ok := selForSpan(content, li, p.Balance.Span, cursor, parent); ok {
			return sel
		}
	}
	if sel, ok := selComment(content, p.Comment, li, cursor, parent); ok {
		return sel
	}
	for i := range p.Comments {
		if sel, ok := selComment(content, &p.Comments[i], li, cursor, parent); ok {
			return sel
		}
	}
	return parent
}

func selAccount(content string, a *ast.Account, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) (protocol.SelectionRange, bool) {
	accountSel, ok := selForSpan(content, li, a.Span, cursor, parent)
	if !ok {
		return protocol.SelectionRange{}, false
	}
	if len(a.Name) <= 1 {
		return accountSel, true
	}
	for i := range a.Name {
		if sel, ok := selForSpan(content, li, a.Name[i].Span, cursor, accountSel); ok {
			return sel, true
		}
	}
	return accountSel, true
}

func selAmount(content string, am *ast.Amount, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) (protocol.SelectionRange, bool) {
	if am == nil {
		return protocol.SelectionRange{}, false
	}
	amountSel, ok := selForSpan(content, li, am.Span, cursor, parent)
	if !ok {
		return protocol.SelectionRange{}, false
	}
	if sel, ok := selForSpan(content, li, am.CommoditySpan, cursor, amountSel); ok {
		return sel, true
	}
	qStart, qEnd := quantitySpan(content, am)
	if qEnd > qStart {
		if sel, ok := selForSpan(content, li, token.Span{Start: token.Pos{Offset: qStart}, End: token.Pos{Offset: qEnd}}, cursor, amountSel); ok {
			return sel, true
		}
	}
	return amountSel, true
}

func selComment(content string, c *ast.Comment, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) (protocol.SelectionRange, bool) {
	if c == nil {
		return protocol.SelectionRange{}, false
	}
	commentSel, ok := selForSpan(content, li, c.Span, cursor, parent)
	if !ok {
		return protocol.SelectionRange{}, false
	}
	if ref := tagRefInComment(content, c, cursor); ref != nil {
		return protocol.SelectionRange{Range: li.SpanRange(ref.span), Parent: &commentSel}, true
	}
	return commentSel, true
}

func selForSpan(content string, li *lsputil.LineIndex, span token.Span, cursor int, parent protocol.SelectionRange) (protocol.SelectionRange, bool) {
	if !spanContains(content, span, cursor) {
		return protocol.SelectionRange{}, false
	}
	return protocol.SelectionRange{Range: li.SpanRange(span), Parent: &parent}, true
}

func commentOrParent(content string, c *ast.Comment, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	if sel, ok := selComment(content, c, li, cursor, parent); ok {
		return sel
	}
	return parent
}
