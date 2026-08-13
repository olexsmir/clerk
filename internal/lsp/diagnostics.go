package lsp

import (
	"context"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/linter"
	"olexsmir.xyz/clerk/journal/token"
)

const diagDebounce = 200 * time.Millisecond

func (s *server) scheduleDiagnostics(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.diagCancel != nil {
		s.diagCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.diagCancel = cancel

	time.AfterFunc(diagDebounce, func() {
		if ctx.Err() != nil {
			return
		}
		s.publishDiagnostics(jsonrpc2.DetachContext(ctx))
	})
}

func (s *server) publishDiagnostics(ctx context.Context) {
	s.log.Debug("publishing diagnostics")

	if ctx.Err() != nil {
		return
	}

	a := s.buildAnalysis()
	if a == nil {
		s.log.Debug("no files in workspace")
		return
	}

	activePaths := make(map[string]bool, len(a.Files))
	for _, pf := range a.Files {
		activePaths[pf.Path] = true
	}

	if ctx.Err() != nil {
		return
	}

	finds := dedupFinds(s.linter.Run(a))

	s.mu.Lock()
	s.current = a
	s.mu.Unlock()

	if ctx.Err() != nil {
		return
	}

	diagsByFile := s.groupFindsByFile(finds)

	for fpath := range activePaths {
		if ctx.Err() != nil {
			return
		}
		if err := s.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
			URI:         uri.File(fpath),
			Diagnostics: diagsByFile[fpath],
		}); err != nil {
			s.log.Warn("publish diagnostics failed", "uri", uri.File(fpath), "err", err)
		}
	}

	s.log.Debug("diagnostics published", "files", len(a.Files), "findings", len(finds))
}

func (s *server) groupFindsByFile(finds []linter.Find) map[string][]protocol.Diagnostic {
	// count per file to pre-size the diagnostic slices: append growth on ~10k
	// findings is the dominant allocation in the diagnostics path
	counts := make(map[string]int, len(finds))
	for _, find := range finds {
		if find.Span.Start.File != "" {
			counts[find.Span.Start.File]++
		}
	}
	diags := make(map[string][]protocol.Diagnostic, len(counts))
	for fpath, n := range counts {
		diags[fpath] = make([]protocol.Diagnostic, 0, n)
	}
	for _, find := range finds {
		file := find.Span.Start.File
		if file == "" {
			continue
		}
		diags[file] = append(diags[file], s.findToDiagnostic(find))
	}
	return diags
}

func (s *server) findToDiagnostic(find linter.Find) protocol.Diagnostic {
	return protocol.Diagnostic{
		Range:    spanToRange(find.Span),
		Severity: severityToLSP(find.Severity),
		Message:  protocol.String(find.Message),
		Source:   protocol.NewOptional(s.name),
		Code:     protocol.String(string(find.Code)),
	}
}

func dedupFinds(finds []linter.Find) []linter.Find {
	seen := make(map[findKey]bool, len(finds))
	dedup := make([]linter.Find, 0, len(finds))
	for _, f := range finds {
		k := findKey{f.Span.Start.File, f.Span.Start.Line, f.Span.Start.Col, f.Code}
		if seen[k] {
			continue
		}
		seen[k] = true
		dedup = append(dedup, f)
	}
	return dedup
}

// findKey identifies a find by its position and rule; a struct key avoids a
// per-find fmt.Sprintf.
type findKey struct {
	file      string
	line, col int
	code      linter.RuleID
}

func spanToRange(span token.Span) protocol.Range {
	return protocol.Range{
		Start: protocol.Position{
			Line:      max(0, uint32(span.Start.Line-1)),
			Character: max(0, uint32(span.Start.Col-1)),
		},
		End: protocol.Position{
			Line:      max(0, uint32(span.End.Line-1)),
			Character: uint32(max(0, span.End.Col-1)),
		},
	}
}

func severityToLSP(s linter.Severity) protocol.DiagnosticSeverity {
	switch s {
	case linter.SeverityError:
		return protocol.DiagnosticSeverityError
	case linter.SeverityWarning:
		return protocol.DiagnosticSeverityWarning
	case linter.SeverityInfo:
		return protocol.DiagnosticSeverityInformation
	case linter.SeverityHint:
		return protocol.DiagnosticSeverityHint
	default:
		panic("impossible diagnostic severity")
	}
}
