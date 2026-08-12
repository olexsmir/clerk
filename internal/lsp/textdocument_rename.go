package lsp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/lsp/lsputil"
	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/token"
)

func (s *server) PrepareRename(_ context.Context, params *protocol.PrepareRenameParams) (protocol.PrepareRenameResult, error) {
	state, ok := s.getDocState(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	an := s.analysis()
	cursor := lsputil.Offset(state.text, int(params.Position.Line), int(params.Position.Character))
	ref := findSymbolUnderCursor(an, params.TextDocument.URI.Path(), state.text, cursor)
	if ref == nil {
		return nil, nil
	}

	return &protocol.PrepareRenamePlaceholder{
		Range:       spanToProtocolRange(state.text, ref.span),
		Placeholder: ref.name,
	}, nil
}

func (s *server) Rename(_ context.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	state, ok := s.getDocState(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	an := s.analysis()
	cursor := lsputil.Offset(state.text, int(params.Position.Line), int(params.Position.Character))
	ref := findSymbolUnderCursor(an, params.TextDocument.URI.Path(), state.text, cursor)
	if ref == nil {
		return nil, nil
	}

	switch ref.kind {
	case symbolAccount:
		if err := validateAccountName(params.NewName); err != nil {
			return nil, err
		}
	case symbolCommodity:
		if err := validateCommodityName(params.NewName); err != nil {
			return nil, err
		}
	case symbolPayee:
		if err := validatePayeeName(params.NewName); err != nil {
			return nil, err
		}
	case symbolTag:
		if err := validateTagName(params.NewName); err != nil {
			return nil, err
		}
	default:
		return nil, nil
	}

	changes := renameChanges(an, ref, params.NewName)
	if len(changes) == 0 {
		return nil, nil
	}
	return &protocol.WorkspaceEdit{Changes: changes}, nil
}

func findSymbolUnderCursor(an *analyzer.Analysis, docPath, content string, cursor int) *symbolRef {
	for _, pf := range an.Files {
		if pf.Path != docPath {
			continue
		}
		for _, entry := range pf.Ast.Entries {
			if ref := symbolInEntry(content, entry, cursor); ref != nil {
				return ref
			}
		}
		return nil
	}
	return nil
}

func symbolInEntry(content string, e ast.Entry, cursor int) *symbolRef {
	switch e := e.(type) {
	case *ast.Transaction:
		if e.Payee != nil && spanContains(content, e.Payee.Span, cursor) {
			return &symbolRef{symbolPayee, e.Payee.Name, e.Payee.Span}
		}
		if ref := tagRefInComment(content, e.Comment, cursor); ref != nil {
			return ref
		}
		for _, c := range e.HeaderComments {
			if ref := tagRefInComment(content, c, cursor); ref != nil {
				return ref
			}
		}
		return symbolInPostings(content, e.Postings, cursor)
	case *ast.PeriodicTransaction:
		if ref := tagRefInComment(content, e.Comment, cursor); ref != nil {
			return ref
		}
		for _, c := range e.HeaderComments {
			if ref := tagRefInComment(content, c, cursor); ref != nil {
				return ref
			}
		}
		return symbolInPostings(content, e.Postings, cursor)
	case *ast.AutomatedTransaction:
		if ref := tagRefInComment(content, e.Comment, cursor); ref != nil {
			return ref
		}
		for _, c := range e.HeaderComments {
			if ref := tagRefInComment(content, c, cursor); ref != nil {
				return ref
			}
		}
		return symbolInPostings(content, e.Postings, cursor)
	case *ast.Comment:
		return tagRefInComment(content, e, cursor)
	case *ast.AccountDirective:
		if spanContains(content, e.Account.Span, cursor) {
			return &symbolRef{symbolAccount, e.Account.String(), e.Account.Span}
		}
		for _, sd := range e.Subdirectives {
			if sd.Name == "alias" && spanContains(content, sd.ValueSpan, cursor) {
				return &symbolRef{symbolAccount, sd.Value, sd.ValueSpan}
			}
		}
	case *ast.AliasDirective:
		if spanContains(content, e.From.Span, cursor) {
			return &symbolRef{symbolAccount, e.From.String(), e.From.Span}
		}
		if spanContains(content, e.To.Span, cursor) {
			return &symbolRef{symbolAccount, e.To.String(), e.To.Span}
		}
	case *ast.CommodityDirective:
		if spanContains(content, e.CommoditySpan, cursor) {
			return &symbolRef{symbolCommodity, e.Commodity, e.CommoditySpan}
		}
	case *ast.PayeeDirective:
		if e.Name != nil && spanContains(content, e.Name.Span, cursor) {
			return &symbolRef{symbolPayee, e.Name.Name, e.Name.Span}
		}
	}
	return nil
}

// symbolKind is the kind of symbol under the cursor.
type symbolKind int

const (
	symbolAccount symbolKind = iota
	symbolCommodity
	symbolPayee
	symbolTag
)

// symbolRef is a symbol under the cursor, ready to be resolved or renamed.
type symbolRef struct {
	kind symbolKind
	name string
	span token.Span
}

// renameTo returns the replacement rext for an occurrence of the nodeKind with the given name.
func (ref *symbolRef) renameTo(nodeKind symbolKind, name, newName string) (text string, renamed bool) {
	if ref.kind != nodeKind {
		return "", false
	}
	if ref.kind == symbolAccount {
		if !accountMatches(name, ref.name) {
			return "", false
		}
		return newName + strings.TrimPrefix(name, ref.name), true
	}
	if name != ref.name {
		return "", false
	}
	return newName, true
}

func accountMatches(name, old string) bool {
	return name == old || strings.HasPrefix(name, old+":")
}

func tagRefInComment(content string, c *ast.Comment, cursor int) *symbolRef {
	if c == nil {
		return nil
	}
	for i := range c.Tags {
		t := &c.Tags[i]
		if span := tagKeySpan(content, t); spanContains(content, span, cursor) {
			return &symbolRef{symbolTag, t.Key, span}
		}
	}
	return nil
}

func commodityRef(content string, am *ast.Amount, cursor int) *symbolRef {
	if am == nil || am.Commodity == "" || !spanContains(content, am.CommoditySpan, cursor) {
		return nil
	}
	return &symbolRef{symbolCommodity, am.Commodity, am.CommoditySpan}
}

func symbolInPostings(content string, postings []*ast.Posting, cursor int) *symbolRef {
	for _, p := range postings {
		if spanContains(content, p.Account.Span, cursor) {
			return &symbolRef{symbolAccount, p.Account.String(), p.Account.Span}
		}
		if ref := commodityRef(content, p.Amount, cursor); ref != nil {
			return ref
		}
		if p.Cost != nil {
			if ref := commodityRef(content, &p.Cost.Amount, cursor); ref != nil {
				return ref
			}
		}
		if p.Balance != nil {
			if ref := commodityRef(content, &p.Balance.Amount, cursor); ref != nil {
				return ref
			}
		}
		if ref := tagRefInComment(content, p.Comment, cursor); ref != nil {
			return ref
		}
		for i := range p.Comments {
			if ref := tagRefInComment(content, &p.Comments[i], cursor); ref != nil {
				return ref
			}
		}
	}
	return nil
}

// renameChanges collects the workspace edits renaming ref to newName
func renameChanges(an *analyzer.Analysis, ref *symbolRef, newName string) map[uri.URI][]protocol.TextEdit {
	changes := make(map[uri.URI][]protocol.TextEdit)
	for _, pf := range an.Files {
		content := string(pf.Src)
		var edits []protocol.TextEdit
		add := func(span token.Span, text string) {
			edits = append(edits, protocol.TextEdit{
				Range:   spanToProtocolRange(content, span),
				NewText: text,
			})
		}
		for _, e := range pf.Ast.Entries {
			renameEntry(add, ref, newName, content, e)
		}
		if len(edits) > 0 {
			changes[uri.File(pf.Path)] = edits
		}
	}
	sortAndDedup(changes)
	return changes
}

func renameEntry(add func(token.Span, string), ref *symbolRef, newName, content string, e ast.Entry) {
	switch e := e.(type) {
	case *ast.Transaction:
		renamePayee(add, ref, newName, e.Payee)
		renameCommentTags(add, ref, newName, content, e.Comment)
		for _, c := range e.HeaderComments {
			renameCommentTags(add, ref, newName, content, c)
		}
		renamePostings(add, ref, newName, content, e.Postings)
	case *ast.PeriodicTransaction:
		renameCommentTags(add, ref, newName, content, e.Comment)
		for _, c := range e.HeaderComments {
			renameCommentTags(add, ref, newName, content, c)
		}
		renamePostings(add, ref, newName, content, e.Postings)
	case *ast.AutomatedTransaction:
		renameCommentTags(add, ref, newName, content, e.Comment)
		for _, c := range e.HeaderComments {
			renameCommentTags(add, ref, newName, content, c)
		}
		renamePostings(add, ref, newName, content, e.Postings)
	case *ast.Comment:
		renameCommentTags(add, ref, newName, content, e)
	case *ast.AccountDirective:
		if text, ok := ref.renameTo(symbolAccount, e.Account.String(), newName); ok {
			add(e.Account.Span, text)
		}
		for _, sd := range e.Subdirectives {
			if sd.Name == "alias" {
				if text, ok := ref.renameTo(symbolAccount, sd.Value, newName); ok {
					add(sd.ValueSpan, text)
				}
			}
		}
	case *ast.AliasDirective:
		if text, ok := ref.renameTo(symbolAccount, e.From.String(), newName); ok {
			add(e.From.Span, text)
		}
		if text, ok := ref.renameTo(symbolAccount, e.To.String(), newName); ok {
			add(e.To.Span, text)
		}
	case *ast.CommodityDirective:
		if text, ok := ref.renameTo(symbolCommodity, e.Commodity, newName); ok {
			add(e.CommoditySpan, text)
		}
	case *ast.PayeeDirective:
		renamePayee(add, ref, newName, e.Name)
	case *ast.TagDirective:
		if text, ok := ref.renameTo(symbolTag, e.Name, newName); ok {
			if span, ok := tagDirectiveSpan(content, e); ok {
				add(span, text)
			}
		}
	}
}

func renamePostings(add func(token.Span, string), ref *symbolRef, newName, content string, postings []*ast.Posting) {
	for _, p := range postings {
		if text, ok := ref.renameTo(symbolAccount, p.Account.String(), newName); ok {
			add(p.Account.Span, text)
		}
		renameCommodity(add, ref, newName, p.Amount)
		if p.Cost != nil {
			renameCommodity(add, ref, newName, &p.Cost.Amount)
		}
		if p.Balance != nil {
			renameCommodity(add, ref, newName, &p.Balance.Amount)
		}
		renameCommentTags(add, ref, newName, content, p.Comment)
		for i := range p.Comments {
			renameCommentTags(add, ref, newName, content, &p.Comments[i])
		}
	}
}

func renameCommodity(add func(token.Span, string), ref *symbolRef, newName string, am *ast.Amount) {
	if am == nil {
		return
	}
	if text, ok := ref.renameTo(symbolCommodity, am.Commodity, newName); ok {
		add(am.CommoditySpan, text)
	}
}

func renamePayee(add func(token.Span, string), ref *symbolRef, newName string, p *ast.Payee) {
	if p == nil {
		return
	}
	if text, ok := ref.renameTo(symbolPayee, p.Name, newName); ok {
		add(p.Span, text)
	}
}

func renameCommentTags(add func(token.Span, string), ref *symbolRef, newName, content string, c *ast.Comment) {
	if c == nil {
		return
	}
	for i := range c.Tags {
		t := &c.Tags[i]
		if text, ok := ref.renameTo(symbolTag, t.Key, newName); ok {
			add(tagKeySpan(content, t), text)
		}
	}
}

func tagDirectiveSpan(content string, d *ast.TagDirective) (token.Span, bool) {
	end := d.Span.End.Offset
	if d.Comment != nil {
		end = d.Comment.Span.Start.Offset
	}
	return betweenSpan(content, d.Span.Start.File, d.Span.Start.Offset+len("tag"), end)
}

func tagKeySpan(content string, t *ast.Tag) token.Span {
	end := t.Span.End.Offset
	for off := t.Span.Start.Offset; off < end; off++ {
		if content[off] == ':' || content[off] == ',' {
			end = off
			break
		}
	}
	for end > t.Span.Start.Offset && (content[end-1] == ' ' || content[end-1] == '\t') {
		end--
	}
	return token.Span{Start: t.Span.Start, End: offsetPos(t.Span.Start.File, end)}
}

func sortAndDedup(changes map[uri.URI][]protocol.TextEdit) {
	for u, edits := range changes {
		sort.Slice(edits, func(i, j int) bool {
			ri, rj := edits[i].Range, edits[j].Range
			if ri.Start.Line != rj.Start.Line {
				return ri.Start.Line < rj.Start.Line
			}
			return ri.Start.Character < rj.Start.Character
		})
		dedup := edits[:0]
		for _, e := range edits {
			if len(dedup) == 0 || dedup[len(dedup)-1] != e {
				dedup = append(dedup, e)
			}
		}
		changes[u] = dedup
	}
}

// Validation

func validateAccountName(name string) error   { return validateRenameName(name, "account", ";") }
func validateCommodityName(name string) error { return validateRenameName(name, "commodity", ";") }
func validatePayeeName(name string) error     { return validateRenameName(name, "payee", ";|") }
func validateTagName(name string) error       { return validateRenameName(name, "tag", ":,; \t") }
func validateRenameName(name, what, forbidden string) error {
	if name == "" {
		return fmt.Errorf("%s name must not be empty", what)
	}
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("%s name must not have leading or trailing whitespace", what)
	}
	for _, r := range name {
		if strings.ContainsRune(forbidden, r) || r == '\n' || r == '\r' {
			return fmt.Errorf("%s name contains illegal character %q", what, r)
		}
	}
	return nil
}
