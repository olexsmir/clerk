package lsp

import (
	"context"
	"fmt"
	"strings"

	"go.lsp.dev/protocol"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/token"
)

func (s *server) Hover(_ context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	state, ok := s.getDocState(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	an := s.analysisFor(params.TextDocument.URI)
	cursor := state.lineIdx.Offset(int(params.Position.Line), int(params.Position.Character))
	el := hoverAt(an, params.TextDocument.URI.Path(), state.text, cursor)
	if el == nil {
		return nil, nil
	}

	return &protocol.Hover{
		Contents: &protocol.MarkupContent{
			Kind:  protocol.MarkupKindMarkdown,
			Value: buildHoverContent(an, state.text, el),
		},
		Range: new(state.lineIdx.SpanRange(el.span)),
	}, nil
}

type hoverKind int

const (
	hoverAccount hoverKind = iota
	hoverCommodity
	hoverPayee
	hoverTagKey
	hoverTagValue
	hoverAmount
	hoverDate
)

type hoverElement struct {
	kind     hoverKind
	span     token.Span
	name     string // account, commodity or payee name
	tagKey   string
	tagValue string
	amount   *ast.Amount
	cost     *ast.Cost
	tx       *ast.Transaction // date hover
}

// hoverAt finds the hover target under the cursor in the document matching
// docPath. Only the entry containing the cursor can match; entries are in
// file order, so [entryAt] finds the containing entry in O(log n).
func hoverAt(an *analyzer.Analysis, docPath, content string, cursor int) *hoverElement {
	for _, pf := range an.Files {
		if pf.Path != docPath {
			continue
		}
		if entry := entryAt(pf.Ast.Entries, cursor); entry != nil {
			return hoverInEntry(content, entry, cursor)
		}
		return nil
	}
	return nil
}

func hoverInEntry(content string, e ast.Entry, cursor int) *hoverElement {
	switch e := e.(type) {
	case *ast.Transaction:
		if e.Payee != nil && spanContains(content, e.Payee.Span, cursor) {
			return &hoverElement{kind: hoverPayee, span: e.Payee.Span, name: e.Payee.Name}
		}
		if spanContains(content, e.Date.Span, cursor) {
			return &hoverElement{kind: hoverDate, span: e.Date.Span, tx: e}
		}
		if el := hoverTagInComment(content, e.Comment, cursor); el != nil {
			return el
		}
		for _, c := range e.HeaderComments {
			if el := hoverTagInComment(content, c, cursor); el != nil {
				return el
			}
		}
		return hoverInPostings(content, e.Postings, cursor)
	case *ast.PeriodicTransaction:
		if el := hoverTagInComment(content, e.Comment, cursor); el != nil {
			return el
		}
		for _, c := range e.HeaderComments {
			if el := hoverTagInComment(content, c, cursor); el != nil {
				return el
			}
		}
		return hoverInPostings(content, e.Postings, cursor)
	case *ast.AutomatedTransaction:
		if el := hoverTagInComment(content, e.Comment, cursor); el != nil {
			return el
		}
		for _, c := range e.HeaderComments {
			if el := hoverTagInComment(content, c, cursor); el != nil {
				return el
			}
		}
		return hoverInPostings(content, e.Postings, cursor)
	case *ast.Comment:
		return hoverTagInComment(content, e, cursor)
	case *ast.AccountDirective:
		if spanContains(content, e.Account.Span, cursor) {
			return &hoverElement{kind: hoverAccount, span: e.Account.Span, name: e.Account.String()}
		}
		for _, sd := range e.Subdirectives {
			if sd.Kind == ast.SubdirectiveAlias && spanContains(content, sd.ValueSpan, cursor) {
				return &hoverElement{kind: hoverAccount, span: sd.ValueSpan, name: sd.Value}
			}
		}
	case *ast.AliasDirective:
		if spanContains(content, e.From.Span, cursor) {
			return &hoverElement{kind: hoverAccount, span: e.From.Span, name: e.From.String()}
		}
		if spanContains(content, e.To.Span, cursor) {
			return &hoverElement{kind: hoverAccount, span: e.To.Span, name: e.To.String()}
		}
	case *ast.CommodityDirective:
		if spanContains(content, e.CommoditySpan, cursor) {
			return &hoverElement{kind: hoverCommodity, span: e.CommoditySpan, name: e.Commodity}
		}
	case *ast.PayeeDirective:
		if e.Name != nil && spanContains(content, e.Name.Span, cursor) {
			return &hoverElement{kind: hoverPayee, span: e.Name.Span, name: e.Name.Name}
		}
	case *ast.TagDirective:
		if e.Name != "" {
			if span, ok := tagDirectiveSpan(content, e); ok && spanContains(content, span, cursor) {
				return &hoverElement{kind: hoverTagKey, span: span, tagKey: e.Name}
			}
		}
	}
	return nil
}

func hoverInPostings(content string, postings []*ast.Posting, cursor int) *hoverElement {
	for _, p := range postings {
		if spanContains(content, p.Account.Span, cursor) {
			return &hoverElement{kind: hoverAccount, span: p.Account.Span, name: p.Account.String()}
		}
		if p.Amount != nil {
			if spanContains(content, p.Amount.Span, cursor) {
				return &hoverElement{kind: hoverAmount, span: p.Amount.Span, amount: p.Amount, cost: p.Cost}
			}
			if ref := commodityRef(content, p.Amount, cursor); ref != nil {
				return &hoverElement{kind: hoverCommodity, span: ref.span, name: ref.name}
			}
		}
		if p.Cost != nil {
			if ref := commodityRef(content, &p.Cost.Amount, cursor); ref != nil {
				return &hoverElement{kind: hoverCommodity, span: ref.span, name: ref.name}
			}
		}
		if p.Balance != nil {
			if ref := commodityRef(content, &p.Balance.Amount, cursor); ref != nil {
				return &hoverElement{kind: hoverCommodity, span: ref.span, name: ref.name}
			}
			if p.Balance.Cost != nil {
				if ref := commodityRef(content, &p.Balance.Cost.Amount, cursor); ref != nil {
					return &hoverElement{kind: hoverCommodity, span: ref.span, name: ref.name}
				}
			}
		}
		if el := hoverTagInComment(content, p.Comment, cursor); el != nil {
			return el
		}
		for i := range p.Comments {
			if el := hoverTagInComment(content, &p.Comments[i], cursor); el != nil {
				return el
			}
		}
	}
	return nil
}

func hoverTagInComment(content string, c *ast.Comment, cursor int) *hoverElement {
	if c == nil {
		return nil
	}
	for i := range c.Tags {
		t := &c.Tags[i]
		if !spanContains(content, t.Span, cursor) {
			continue
		}
		keySpan := tagKeySpan(content, t)
		if spanContains(content, keySpan, cursor) {
			return &hoverElement{kind: hoverTagKey, span: keySpan, tagKey: t.Key}
		}
		return &hoverElement{kind: hoverTagValue, span: tagValueSpan(content, t), tagKey: t.Key, tagValue: t.Value}
	}
	return nil
}

func buildHoverContent(an *analyzer.Analysis, content string, el *hoverElement) string {
	switch el.kind {
	case hoverAccount:
		return buildAccountHover(an, el.name)
	case hoverCommodity:
		return buildCommodityHover(an, el.name)
	case hoverPayee:
		return buildPayeeHover(an, el.name)
	case hoverTagKey:
		return buildTagHover(an, el.tagKey)
	case hoverTagValue:
		return buildTagValueHover(an, el.tagKey, el.tagValue)
	case hoverAmount:
		return buildAmountHover(content, el.amount, el.cost)
	case hoverDate:
		return buildDateHover(an, el.tx)
	default:
		return ""
	}
}

func buildAccountHover(an *analyzer.Analysis, name string) string {
	if canon, ok := an.AccountAliases[name]; ok {
		name = canon
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "**Account:** `%s`", name)
	if info := an.Accounts[name]; info != nil {
		fmt.Fprintf(&sb, "\n**Postings:** %d", info.UsedCount)
		writeDateSection(&sb, "**Last used:**", info.LastUsed)
	}
	return sb.String()
}

func buildCommodityHover(an *analyzer.Analysis, symbol string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "**Commodity:** `%s`", symbol)
	if info := an.Commodities[symbol]; info != nil {
		fmt.Fprintf(&sb, "\n**Usage:** %d", info.UsedCount)
		writeDateSection(&sb, "**Last used:**", info.LastUsed)
	}
	return sb.String()
}

func buildPayeeHover(an *analyzer.Analysis, name string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "**Payee:** %s", name)
	if info := an.Payees[name]; info != nil {
		fmt.Fprintf(&sb, "\n**Transactions:** %d", info.UsedCount)
	}
	return sb.String()
}

func buildTagHover(an *analyzer.Analysis, key string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "**Tag:** `%s`", key)
	if info := an.Tags[key]; info != nil {
		fmt.Fprintf(&sb, "\n**Usage:** %d", info.UsedCount)
		if len(info.Values) > 0 {
			sb.WriteString("\n**Values:**\n")
			for i, v := range info.Values {
				if i > 0 {
					sb.WriteByte('\n')
				}
				if v == "" {
					sb.WriteString("- *(empty)*")
				} else {
					fmt.Fprintf(&sb, "- `%s`", v)
				}
			}
		}
	}
	return sb.String()
}

func buildTagValueHover(an *analyzer.Analysis, key, value string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "**Tag:** `%s`", key)
	if value == "" {
		sb.WriteString("\n**Value:** *(empty)*")
	} else {
		fmt.Fprintf(&sb, "\n**Value:** `%s`", value)
	}
	if info := an.Tags[key]; info != nil {
		count := 0
		for _, u := range info.Usage {
			if u.Tag.Value == value {
				count++
			}
		}
		fmt.Fprintf(&sb, "\n**Usage:** %d", count)
	}
	return sb.String()
}

func buildAmountHover(content string, am *ast.Amount, cost *ast.Cost) string {
	amountText := strings.TrimRight(content[am.Span.Start.Offset:am.Span.End.Offset], " \t")
	var sb strings.Builder
	fmt.Fprintf(&sb, "**Amount:** %s", amountText)
	if cost != nil {
		costText := strings.TrimRight(content[cost.Amount.Span.Start.Offset:cost.Amount.Span.End.Offset], " \t")
		label, marker := "**Unit cost:**", "@"
		if cost.IsTotal {
			label, marker = "**Total cost:**", "@@"
		}
		fmt.Fprintf(&sb, "\n\n%s %s %s", label, marker, costText)
	}
	return sb.String()
}

func buildDateHover(an *analyzer.Analysis, tx *ast.Transaction) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "**Date:** %04d-%02d-%02d", tx.Date.Year, tx.Date.Month, tx.Date.Day)
	if tx.Payee != nil {
		fmt.Fprintf(&sb, "\n\n**Payee:** %s", tx.Payee.Name)
	}
	fmt.Fprintf(&sb, "\n\n**Transactions:** %d", an.CountTransactionsOnDate(tx.Date))
	fmt.Fprintf(&sb, "\n\n**Postings:** %d", len(tx.Postings))
	return sb.String()
}

func writeDateSection(sb *strings.Builder, label string, d ast.Date) {
	if d.Year == 0 {
		return
	}
	fmt.Fprintf(sb, "\n%s %04d-%02d-%02d", label, d.Year, d.Month, d.Day)
}

func tagValueSpan(content string, t *ast.Tag) token.Span {
	start := tagKeySpan(content, t).End.Offset
	if start < t.Span.End.Offset && content[start] == ':' {
		start++
	}
	end := t.Span.End.Offset
	for end > start && (content[end-1] == ' ' || content[end-1] == '\t') {
		end--
	}
	return token.Span{File: t.Span.File, Start: token.Pos{Offset: start}, End: token.Pos{Offset: end}}
}
