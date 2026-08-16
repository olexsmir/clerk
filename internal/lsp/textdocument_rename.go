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

	an := s.analysisFor(params.TextDocument.URI)
	cursor := state.lineIdx.Offset(int(params.Position.Line), int(params.Position.Character))
	ref := findSymbolUnderCursor(an, params.TextDocument.URI.Path(), state.text, cursor)
	if ref == nil {
		return nil, nil
	}

	return &protocol.PrepareRenamePlaceholder{
		Range:       state.lineIdx.SpanRange(ref.span),
		Placeholder: ref.name,
	}, nil
}

func (s *server) Rename(_ context.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	state, ok := s.getDocState(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	an := s.analysisFor(params.TextDocument.URI)
	cursor := state.lineIdx.Offset(int(params.Position.Line), int(params.Position.Character))
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

	changes := renameChanges(an, ref, params.NewName, state.lineIdx)
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
		if entry := entryAt(pf.Ast.Entries, cursor); entry != nil {
			return symbolInEntry(content, entry, cursor)
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
			if sd.Kind == ast.SubdirectiveAlias && spanContains(content, sd.ValueSpan, cursor) {
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
	symbolTransaction
	symbolCommodity
	symbolPayee
	symbolTag
)

func (s symbolKind) ToProtocol() protocol.SymbolKind {
	switch s {
	case symbolAccount:
		return protocol.SymbolKindClass
	case symbolTransaction:
		return protocol.SymbolKindEvent
	case symbolCommodity:
		return protocol.SymbolKindVariable
	case symbolPayee:
		return protocol.SymbolKindObject
	case symbolTag:
		return protocol.SymbolKindProperty
	}
	return protocol.SymbolKindFile
}

// symbolRef is a symbol under the cursor, ready to be resolved or renamed.
type symbolRef struct {
	kind symbolKind
	name string
	span token.Span
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

func renameChanges(an *analyzer.Analysis, ref *symbolRef, newName string, primaryLI *lsputil.LineIndex) map[uri.URI][]protocol.TextEdit {
	type fileEdits struct {
		li    *lsputil.LineIndex
		edits []protocol.TextEdit
	}
	files := make(map[int]*fileEdits)
	add := func(fileIdx int, span token.Span, text string) {
		fe := files[fileIdx]
		if fe == nil {
			fe = &fileEdits{}
			if fileIdx == 0 {
				fe.li = primaryLI
			} else {
				fe.li = lsputil.NewLineIndex(string(an.Files[fileIdx].Src))
			}
			files[fileIdx] = fe
		}
		fe.edits = append(fe.edits, protocol.TextEdit{
			Range:   fe.li.SpanRange(span),
			NewText: text,
		})
	}

	switch ref.kind {
	case symbolAccount:
		renameAccountEdits(an, ref, newName, add)
	case symbolCommodity:
		renameCommodityEdits(an, ref, newName, add)
	case symbolPayee:
		renamePayeeEdits(an, ref, newName, add)
	case symbolTag:
		renameTagEdits(an, ref, newName, add)
	}

	changes := make(map[uri.URI][]protocol.TextEdit, len(files))
	for fileIdx, fe := range files {
		changes[uri.File(an.Files[fileIdx].Path)] = fe.edits
	}
	sortAndDedup(changes)
	return changes
}

func renameAccountEdits(an *analyzer.Analysis, ref *symbolRef, newName string, add func(int, token.Span, string)) {
	for _, name := range an.AccountNames {
		if !accountMatches(name, ref.name) {
			continue
		}
		info := an.Accounts[name]
		text := newName + strings.TrimPrefix(name, ref.name)
		for _, u := range info.Usages {
			add(u.FileIndex, u.Posting.Account.Span, text)
		}
	}
	for _, info := range an.Accounts {
		for _, d := range info.Directives {
			fileIdx := fileIndexForEntry(an, d)
			if fileIdx < 0 {
				continue
			}
			if accountMatches(d.Account.String(), ref.name) {
				add(fileIdx, d.Account.Span, newName+strings.TrimPrefix(d.Account.String(), ref.name))
			}
			for _, sd := range d.Subdirectives {
				if sd.Kind == ast.SubdirectiveAlias && accountMatches(sd.Value, ref.name) {
					add(fileIdx, sd.ValueSpan, newName+strings.TrimPrefix(sd.Value, ref.name))
				}
			}
		}
	}
	for _, ad := range an.AliasDirectives {
		fileIdx := fileIndexForEntry(an, ad)
		if fileIdx < 0 {
			continue
		}
		if accountMatches(ad.From.String(), ref.name) {
			add(fileIdx, ad.From.Span, newName+strings.TrimPrefix(ad.From.String(), ref.name))
		}
		if accountMatches(ad.To.String(), ref.name) {
			add(fileIdx, ad.To.Span, newName+strings.TrimPrefix(ad.To.String(), ref.name))
		}
	}
}

func renameCommodityEdits(an *analyzer.Analysis, ref *symbolRef, newName string, add func(int, token.Span, string)) {
	info := an.Commodities[ref.name]
	if info == nil {
		return
	}
	for _, d := range info.Directives {
		if fileIdx := fileIndexForEntry(an, d); fileIdx >= 0 {
			add(fileIdx, d.CommoditySpan, newName)
		}
	}
	for _, u := range info.Usages {
		add(u.FileIndex, u.Amount.CommoditySpan, newName)
	}
}

func renamePayeeEdits(an *analyzer.Analysis, ref *symbolRef, newName string, add func(int, token.Span, string)) {
	info := an.Payees[ref.name]
	if info == nil {
		return
	}
	for _, d := range info.Directives {
		if d.Name != nil {
			if fileIdx := fileIndexForEntry(an, d); fileIdx >= 0 {
				add(fileIdx, d.Name.Span, newName)
			}
		}
	}
	for _, u := range info.Usage {
		add(u.FileIndex, u.Payee.Span, newName)
	}
}

func renameTagEdits(an *analyzer.Analysis, ref *symbolRef, newName string, add func(int, token.Span, string)) {
	info := an.Tags[ref.name]
	if info == nil {
		return
	}
	contents := make(map[int]string) // file index → source, converted once per file
	content := func(fileIdx int) string {
		s, ok := contents[fileIdx]
		if !ok {
			s = string(an.Files[fileIdx].Src)
			contents[fileIdx] = s
		}
		return s
	}
	for _, d := range info.Directives {
		fileIdx := fileIndexForEntry(an, d)
		if fileIdx < 0 {
			continue
		}
		if span, ok := tagDirectiveSpan(content(fileIdx), d); ok {
			add(fileIdx, span, newName)
		}
	}
	for _, u := range info.Usage {
		span := tagKeySpan(content(u.FileIndex), u.Tag)
		add(u.FileIndex, span, newName)
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
