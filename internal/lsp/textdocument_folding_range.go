package lsp

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"go.lsp.dev/protocol"

	"olexsmir.xyz/clerk/journal/ast"
)

func (s *server) FoldingRange(_ context.Context, params *protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) {
	an := s.analysisFor(params.TextDocument.URI)
	if an == nil {
		return nil, nil
	}
	for _, pf := range an.Files {
		if pf.Path != params.TextDocument.URI.Path() {
			continue
		}
		ranges := foldingRangesFor(pf.Ast.Entries)
		slices.SortFunc(ranges, func(a, b protocol.FoldingRange) int {
			return cmp.Compare(a.StartLine, b.StartLine)
		})
		return ranges, nil
	}
	return nil, nil
}

// foldingRangesFor folds every foldable structure in a journal's top-level entries:
// posting blocks, directive sub-blocks, comment runs, comment blocks, and apply/end pairs.
func foldingRangesFor(entries []ast.Entry) []protocol.FoldingRange {
	ranges := make([]protocol.FoldingRange, 0, len(entries)) // each entry folds to at most one range
	var (
		comments []*ast.Comment // consecutive comment lines, flushed at the next non-comment entry
		applies  []*ast.ApplyDirective
	)

	flush := func() {
		ranges = appendFold(ranges, commentRunFold(comments))
		comments = nil
	}

	for _, entry := range entries {
		if c, ok := entry.(*ast.Comment); ok {
			comments = append(comments, c)
			continue
		}
		flush()

		switch e := entry.(type) {
		case *ast.Transaction:
			ranges = appendFold(ranges, postingsFold(e.Postings))
		case *ast.PeriodicTransaction:
			ranges = appendFold(ranges, postingsFold(e.Postings))
		case *ast.AutomatedTransaction:
			ranges = appendFold(ranges, postingsFold(e.Postings))
		case *ast.AccountDirective:
			ranges = appendFold(ranges, accountDirectiveFold(e))
		case *ast.CommentBlockDirective:
			ranges = appendFold(ranges, commentBlockDirectiveFold(e))
		case *ast.ApplyDirective:
			applies = append(applies, e)
		case *ast.EndDirective:
			if n := len(applies); n > 0 && applyKeyword(applies[n-1].Expr) == applyKeyword(e.Expr) {
				a := applies[n-1]
				applies = applies[:n-1]
				ranges = appendFold(ranges, applyFold(a, e))
			}
		}
	}
	flush()
	return ranges
}

// appendFold appends r, skipping nil.
func appendFold(ranges []protocol.FoldingRange, r *protocol.FoldingRange) []protocol.FoldingRange {
	if r != nil {
		ranges = append(ranges, *r)
	}
	return ranges
}

// postingsFold folds the posting block of a transaction, leaving the header visible.
func postingsFold(postings []*ast.Posting) *protocol.FoldingRange {
	if len(postings) < 2 {
		return nil
	}
	first, last := postings[0], postings[len(postings)-1]
	return foldRange(
		uint32(first.Span.Start.Line-1),
		uint32(last.Span.Start.Line-1),
		protocol.FoldingRangeKindRegion,
	)
}

// accountDirectiveFold folds the indented subdirective block after an "account" line.
func accountDirectiveFold(ad *ast.AccountDirective) *protocol.FoldingRange {
	if len(ad.Subdirectives) < 2 {
		return nil
	}
	first, last := ad.Subdirectives[0], ad.Subdirectives[len(ad.Subdirectives)-1]
	return foldRange(
		uint32(first.NameSpan.Start.Line-1),
		uint32(last.NameSpan.Start.Line-1),
		protocol.FoldingRangeKindRegion,
	)
}

// commentBlockDirectiveFold folds a "comment" ... "end comment" block, leaving the header visible.
// The directive span extends past the block, so the end line is derived from the content.
func commentBlockDirectiveFold(cb *ast.CommentBlockDirective) *protocol.FoldingRange {
	start := uint32(cb.Span.Start.Line - 1)
	end := start + uint32(strings.Count(cb.Content, "\n")) + 1 // +1 for the "end comment" line
	return foldRange(start, end, protocol.FoldingRangeKindComment)
}

// commentRunFold folds consecutive comment lines into a single range.
func commentRunFold(comments []*ast.Comment) *protocol.FoldingRange {
	if len(comments) < 2 {
		return nil
	}
	return foldRange(
		uint32(comments[0].Span.Start.Line-1),
		uint32(comments[len(comments)-1].Span.Start.Line-1),
		protocol.FoldingRangeKindComment,
	)
}

// applyKeyword returns the first word of an apply/end directive expr ("apply tag work" → "tag").
func applyKeyword(expr string) string {
	if i := strings.IndexByte(expr, ' '); i >= 0 {
		return expr[:i]
	}
	return expr
}

// applyFold folds an "apply" ... "end" block.
func applyFold(a *ast.ApplyDirective, end *ast.EndDirective) *protocol.FoldingRange {
	return foldRange(
		uint32(a.Span.Start.Line-1),
		uint32(end.Span.Start.Line-1),
		protocol.FoldingRangeKindRegion,
	)
}

// foldRange returns nil when the range is not foldable (single line).
func foldRange(startLine, endLine uint32, kind protocol.FoldingRangeKind) *protocol.FoldingRange {
	if endLine <= startLine {
		return nil
	}
	return &protocol.FoldingRange{StartLine: startLine, EndLine: endLine, Kind: kind}
}
