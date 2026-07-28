package tags

import (
	"io"
	"path/filepath"
	"sort"

	"olexsmir.xyz/clerk/internal/tags"
	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/token"
)

const (
	KindAccount     = 'a'
	KindAccountDesc = "account"

	KindCommodity     = 'c'
	KindCommodityDesc = "commodity"

	KindPayee     = 'p'
	KindPayeeDesc = "payee"

	KindTag     = 't'
	KindTagDesc = "tag"
)

// priority constants for source types - lower wins.
const (
	prioTagDirective = 0

	prioAccountDirective = 0
	prioAliasDirective   = 1
	prioAccountPosting   = 2

	prioCommodityDirective = 0
	prioMarketPrice        = 1
	prioAmountCommodity    = 2

	prioPayeeDirective   = 0
	prioPayeeTransaction = 1
)

// candidate is a potential tag before sorting and dedup.
type candidate struct {
	name     string
	kind     byte
	file     string
	line     int
	priority int
	src      []byte
	offset   int
}

type Tagger struct {
	rj     *journal.ResolvedJournal
	relDir string

	candidates []candidate
}

// New creates a [Tagger] from a resolved journal. If relDir is not empty,
// file paths in the tag output are made relative to relDir.
func New(rj *journal.ResolvedJournal, relDir string) *Tagger {
	return &Tagger{rj: rj, relDir: relDir}
}

// Write generate tags file and write it in tags format.
func (t *Tagger) Write(w io.Writer) error {
	tw := tags.NewWriter()
	tw.DescribeKind(KindAccount, KindAccountDesc)
	tw.DescribeKind(KindCommodity, KindCommodityDesc)
	tw.DescribeKind(KindPayee, KindPayeeDesc)
	tw.DescribeKind(KindTag, KindTagDesc)
	for _, e := range t.collect() {
		tw.Add(e)
	}
	return tw.Write(w)
}

func (t *Tagger) collect() []tags.Entry {
	t.candidates = nil

	for _, item := range t.rj.Items {
		if item.IsInclude {
			continue
		}
		pf := item.Occurrence
		filePath := pf.Path
		if t.relDir != "" {
			filePath = relativePath(filePath, t.relDir)
		}
		t.collectFromEntry(pf, filePath, pf.Ast.Entries[item.EntryIndex])
	}

	// sort by priority
	sort.Slice(t.candidates, func(i, j int) bool {
		if t.candidates[i].name != t.candidates[j].name {
			return t.candidates[i].name < t.candidates[j].name
		}
		if t.candidates[i].kind != t.candidates[j].kind {
			return t.candidates[i].kind < t.candidates[j].kind
		}
		if t.candidates[i].priority != t.candidates[j].priority {
			return t.candidates[i].priority < t.candidates[j].priority
		}
		if t.candidates[i].file != t.candidates[j].file {
			return t.candidates[i].file < t.candidates[j].file
		}
		return t.candidates[i].line < t.candidates[j].line
	})

	// deduplicate
	var entries []tags.Entry
	for i, c := range t.candidates {
		if i > 0 && t.candidates[i-1].name == c.name && t.candidates[i-1].kind == c.kind {
			continue
		}
		entries = append(entries, tags.Entry{
			Name:     c.name,
			File:     c.file,
			Line:     c.line,
			Pattern:  extractLine(c.src, c.offset),
			Kind:     c.kind,
			Language: "hledger",
		})
	}

	return entries
}

func (t *Tagger) collectFromEntry(pf *journal.ParsedFile, fpath string, entry ast.Entry) {
	switch e := entry.(type) {
	case *ast.AccountDirective:
		t.addCandidate(e.Account.String(), fpath, e.Account.Span, pf.Src, KindAccount, prioAccountDirective)

	case *ast.CommodityDirective:
		t.addCandidate(e.Commodity, fpath, e.Span, pf.Src, KindCommodity, prioCommodityDirective)

	case *ast.PayeeDirective:
		t.addCandidate(e.Name, fpath, e.Span, pf.Src, KindPayee, prioPayeeDirective)

	case *ast.TagDirective:
		t.addCandidate(e.Name, fpath, e.Span, pf.Src, KindTag, prioTagDirective)

	case *ast.Transaction:
		if e.Payee != nil {
			t.addCandidate(e.Payee.Name, fpath, e.Payee.Span, pf.Src, KindPayee, prioPayeeTransaction)
		}
		for _, p := range e.Postings {
			t.collectFromPosting(p, fpath, pf.Src)
		}

	case *ast.PeriodicTransaction:
		for _, p := range e.Postings {
			t.collectFromPosting(p, fpath, pf.Src)
		}

	case *ast.AutomatedTransaction:
		for _, p := range e.Postings {
			t.collectFromPosting(p, fpath, pf.Src)
		}

	case *ast.MarketPriceDirective:
		t.addCandidate(e.Commodity, fpath, e.Span, pf.Src, KindCommodity, prioMarketPrice)
		if e.Amount.Commodity != "" {
			t.addCandidate(e.Amount.Commodity, fpath, e.Amount.Span, pf.Src, KindCommodity, prioMarketPrice)
		}

	case *ast.DefaultCommodityDirective:
		if e.Amount.Commodity != "" {
			t.addCandidate(e.Amount.Commodity, fpath, e.Amount.Span, pf.Src, KindCommodity, prioAmountCommodity)
		}

	case *ast.ConversionDirective:
		if e.From.Commodity != "" {
			t.addCandidate(e.From.Commodity, fpath, e.From.Span, pf.Src, KindCommodity, prioAmountCommodity)
		}
		if e.To.Commodity != "" {
			t.addCandidate(e.To.Commodity, fpath, e.To.Span, pf.Src, KindCommodity, prioAmountCommodity)
		}

	case *ast.AliasDirective:
		t.addCandidate(e.From.String(), fpath, e.Span, pf.Src, KindAccount, prioAliasDirective)
		t.addCandidate(e.To.String(), fpath, e.Span, pf.Src, KindAccount, prioAliasDirective)
	}
}

func (t *Tagger) collectFromPosting(p *ast.Posting, filePath string, src []byte) {
	t.addCandidate(p.Account.String(), filePath, p.Account.Span, src, KindAccount, prioAccountPosting)
	if p.Amount != nil && p.Amount.Commodity != "" {
		t.addCandidate(p.Amount.Commodity, filePath, p.Amount.Span, src, KindCommodity, prioAmountCommodity)
	}
	if p.Cost != nil && p.Cost.Amount.Commodity != "" {
		t.addCandidate(p.Cost.Amount.Commodity, filePath, p.Cost.Amount.Span, src, KindCommodity, prioAmountCommodity)
	}
	if p.Balance != nil && p.Balance.Amount.Commodity != "" {
		t.addCandidate(p.Balance.Amount.Commodity, filePath, p.Balance.Amount.Span, src, KindCommodity, prioAmountCommodity)
	}
}

func (t *Tagger) addCandidate(name, fpath string, span token.Span, src []byte, kind byte, priority int) {
	if name == "" {
		return
	}
	t.candidates = append(t.candidates, candidate{
		name:     name,
		kind:     kind,
		file:     fpath,
		line:     span.Start.Line,
		priority: priority,
		src:      src,
		offset:   span.Start.Offset,
	})
}

func relativePath(target, base string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}

// extractLine returns the source line containing offset.
// Caller guarantees src is non-nil and 0 <= offset < len(src).
func extractLine(src []byte, offset int) string {
	start := offset
	for start > 0 && src[start-1] != '\n' {
		start--
	}
	end := offset
	for end < len(src) && src[end] != '\n' {
		end++
	}
	return string(src[start:end])
}
