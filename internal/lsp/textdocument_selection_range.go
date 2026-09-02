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
		return transactionSelection(content, t, li, cursor, parent)
	case *ast.PeriodicTransaction:
		return periodicSelection(content, t, li, cursor, parent)
	case *ast.AutomatedTransaction:
		return automatedSelection(content, t, li, cursor, parent)
	case *ast.AccountDirective:
		return accountDirectiveSelection(content, t, li, cursor, parent)
	case *ast.CommodityDirective:
		return commodityDirectiveSelection(content, t, li, cursor, parent)
	case *ast.PayeeDirective:
		if t.Name != nil {
			if sel, ok := selectionForSpan(content, li, t.Name.Span, cursor, parent); ok {
				return sel
			}
		}
		return commentOrParent(content, t.Comment, li, cursor, parent)
	case *ast.TagDirective:
		if sp, ok := tagDirectiveSpan(content, t); ok {
			if sel, ok := selectionForSpan(content, li, sp, cursor, parent); ok {
				return sel
			}
		}
		return commentOrParent(content, t.Comment, li, cursor, parent)
	case *ast.AliasDirective:
		if sel, ok := accountSelection(content, &t.From, li, cursor, parent); ok {
			return sel
		}
		if sel, ok := accountSelection(content, &t.To, li, cursor, parent); ok {
			return sel
		}
		return commentOrParent(content, t.Comment, li, cursor, parent)
	case *ast.DefaultCommodityDirective:
		if sel, ok := amountSelection(content, &t.Amount, li, cursor, parent); ok {
			return sel
		}
		return commentOrParent(content, t.Comment, li, cursor, parent)
	case *ast.MarketPriceDirective:
		if sel, ok := selectionForSpan(content, li, t.DateTime.Date.Span, cursor, parent); ok {
			return sel
		}
		if t.DateTime.Time != nil {
			if sel, ok := selectionForSpan(content, li, t.DateTime.Time.Span, cursor, parent); ok {
				return sel
			}
		}
		if sel, ok := amountSelection(content, &t.Amount, li, cursor, parent); ok {
			return sel
		}
		return commentOrParent(content, t.Comment, li, cursor, parent)
	case *ast.ConversionDirective:
		if sel, ok := amountSelection(content, &t.From, li, cursor, parent); ok {
			return sel
		}
		if sel, ok := amountSelection(content, &t.To, li, cursor, parent); ok {
			return sel
		}
		return commentOrParent(content, t.Comment, li, cursor, parent)
	}
	return parent
}

func transactionSelection(content string, t *ast.Transaction, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	var header [6]token.Span // date, second date, status, code, payee, note
	sps := append(header[:0], t.Date.Span)
	if t.SecondDate != nil {
		sps = append(sps, t.SecondDate.Span)
	}
	if t.Status.Value != ast.StatusNone {
		sps = append(sps, t.Status.Span)
	}
	if t.Code != nil {
		sps = append(sps, t.Code.Span)
	}
	if t.Payee != nil {
		sps = append(sps, t.Payee.Span)
	}
	if t.Note != nil {
		sps = append(sps, t.Note.Span)
	}
	for _, sp := range sps {
		if sel, ok := selectionForSpan(content, li, sp, cursor, parent); ok {
			return sel
		}
	}
	return commentsAndPostingsSelection(content, t.Comment, t.HeaderComments, t.Postings, li, cursor, parent)
}

// commentsAndPostingsSelection descends into an entry's inline comment, header
// comments, then postings; returns parent when none contains the cursor.
func commentsAndPostingsSelection(content string, inline *ast.Comment, headers []*ast.Comment, postings []ast.Posting, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	if sel, ok := commentSelection(content, inline, li, cursor, parent); ok {
		return sel
	}
	for _, c := range headers {
		if sel, ok := commentSelection(content, c, li, cursor, parent); ok {
			return sel
		}
	}
	if sel, ok := postingsSelection(content, postings, li, cursor, parent); ok {
		return sel
	}
	return parent
}

func periodicSelection(content string, pt *ast.PeriodicTransaction, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	if sel, ok := selectionForSpan(content, li, pt.Period.Span, cursor, parent); ok {
		if d := pt.Period.From; d != nil {
			if sub, ok := selectionForSpan(content, li, d.Span, cursor, sel); ok {
				return sub
			}
		}
		if d := pt.Period.To; d != nil {
			if sub, ok := selectionForSpan(content, li, d.Span, cursor, sel); ok {
				return sub
			}
		}
		return sel
	}
	if pt.Description != nil {
		if sel, ok := selectionForSpan(content, li, pt.Description.Span, cursor, parent); ok {
			return sel
		}
	}
	return commentsAndPostingsSelection(content, pt.Comment, pt.HeaderComments, pt.Postings, li, cursor, parent)
}

func automatedSelection(content string, at *ast.AutomatedTransaction, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	if sel, ok := selectionForSpan(content, li, at.Expr.Span, cursor, parent); ok {
		return sel
	}
	return commentsAndPostingsSelection(content, at.Comment, at.HeaderComments, at.Postings, li, cursor, parent)
}

func accountDirectiveSelection(content string, d *ast.AccountDirective, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	if sel, ok := accountSelection(content, &d.Account, li, cursor, parent); ok {
		return sel
	}
	for i := range d.Subdirectives {
		sd := &d.Subdirectives[i]
		if sd.Kind == ast.SubdirectiveComment {
			if sel, ok := commentSelection(content, sd.Comment, li, cursor, parent); ok {
				return sel
			}
			continue
		}
		if sel, ok := selectionForSpan(content, li, sd.ValueSpan, cursor, parent); ok {
			return sel
		}
	}
	return commentOrParent(content, d.Comment, li, cursor, parent)
}

func commodityDirectiveSelection(content string, d *ast.CommodityDirective, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	if sel, ok := selectionForSpan(content, li, d.CommoditySpan, cursor, parent); ok {
		return sel
	}
	if d.FormatSub != nil {
		if sel, ok := amountSelection(content, &d.FormatSub.Amount, li, cursor, parent); ok {
			return sel
		}
	}
	return commentOrParent(content, d.Comment, li, cursor, parent)
}

// postingsSelection returns the selection inside the posting containing cursor.
func postingsSelection(content string, postings []ast.Posting, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) (protocol.SelectionRange, bool) {
	for i := range postings {
		p := &postings[i]
		postingSel, ok := selectionForSpan(content, li, p.Span, cursor, parent)
		if !ok {
			continue
		}
		return postingSelection(content, p, li, cursor, postingSel), true
	}
	return protocol.SelectionRange{}, false
}

func postingSelection(content string, p *ast.Posting, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	if p.Status.Value != ast.StatusNone {
		if sel, ok := selectionForSpan(content, li, p.Status.Span, cursor, parent); ok {
			return sel
		}
	}
	if sel, ok := accountSelection(content, &p.Account, li, cursor, parent); ok {
		return sel
	}
	if sel, ok := amountSelection(content, p.Amount, li, cursor, parent); ok {
		return sel
	}
	if p.Cost != nil {
		if sel, ok := selectionForSpan(content, li, p.Cost.Span, cursor, parent); ok {
			return sel
		}
	}
	if p.Balance != nil {
		if sel, ok := selectionForSpan(content, li, p.Balance.Span, cursor, parent); ok {
			return sel
		}
	}
	if sel, ok := commentSelection(content, p.Comment, li, cursor, parent); ok {
		return sel
	}
	for i := range p.Comments {
		if sel, ok := commentSelection(content, &p.Comments[i], li, cursor, parent); ok {
			return sel
		}
	}
	return parent
}

func accountSelection(content string, a *ast.Account, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) (protocol.SelectionRange, bool) {
	accountSel, ok := selectionForSpan(content, li, a.Span, cursor, parent)
	if !ok {
		return protocol.SelectionRange{}, false
	}
	if len(a.Name) <= 1 {
		return accountSel, true
	}
	for i := range a.Name {
		if sel, ok := selectionForSpan(content, li, a.Name[i].Span, cursor, accountSel); ok {
			return sel, true
		}
	}
	return accountSel, true
}

func amountSelection(content string, am *ast.Amount, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) (protocol.SelectionRange, bool) {
	if am == nil {
		return protocol.SelectionRange{}, false
	}
	amountSel, ok := selectionForSpan(content, li, am.Span, cursor, parent)
	if !ok {
		return protocol.SelectionRange{}, false
	}
	if sel, ok := selectionForSpan(content, li, am.CommoditySpan, cursor, amountSel); ok {
		return sel, true
	}
	qStart, qEnd := quantitySpan(content, am)
	if qEnd > qStart {
		if sel, ok := selectionForSpan(content, li, token.Span{Start: token.Pos{Offset: qStart}, End: token.Pos{Offset: qEnd}}, cursor, amountSel); ok {
			return sel, true
		}
	}
	return amountSel, true
}

// selectionForSpan selects span nested in parent when span contains cursor.
func selectionForSpan(content string, li *lsputil.LineIndex, span token.Span, cursor int, parent protocol.SelectionRange) (protocol.SelectionRange, bool) {
	if !spanContains(content, span, cursor) {
		return protocol.SelectionRange{}, false
	}
	return protocol.SelectionRange{Range: li.SpanRange(span), Parent: &parent}, true
}

// commentSelection returns a selection inside comment when the cursor is on it:
// the tag key when the cursor is on a tag, else the whole comment line.
func commentSelection(content string, c *ast.Comment, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) (protocol.SelectionRange, bool) {
	if c == nil {
		return protocol.SelectionRange{}, false
	}
	commentSel, ok := selectionForSpan(content, li, c.Span, cursor, parent)
	if !ok {
		return protocol.SelectionRange{}, false
	}
	if ref := tagRefInComment(content, c, cursor); ref != nil {
		return protocol.SelectionRange{Range: li.SpanRange(ref.span), Parent: &commentSel}, true
	}
	return commentSel, true
}

// commentOrParent is commentSelection with parent as the fallback.
func commentOrParent(content string, c *ast.Comment, li *lsputil.LineIndex, cursor int, parent protocol.SelectionRange) protocol.SelectionRange {
	if sel, ok := commentSelection(content, c, li, cursor, parent); ok {
		return sel
	}
	return parent
}
