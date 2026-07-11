package lsp

import (
	"context"
	"fmt"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"olexsmir.xyz/clerk/internal/linter"
	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/semantic"
	"olexsmir.xyz/clerk/journal/token"
)

func (s *server) rebuildWorkspace(ctx context.Context) {
	s.log.Debug("rebuilding workspace")

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.openDocs) == 0 {
		return
	}

	for uri, state := range s.openDocs {
		path := uri.Path()
		if _, err := s.loader.Reload(path, []byte(state.text)); err != nil {
			s.log.Warn("filed to reload open document", "uri", uri, "err", err)
		}
	}

	roots := s.loader.Roots()
	if len(roots) == 0 {
		s.log.Debug("no files in workspace")
		return
	}

	var allFiles []*journal.ParsedFile
	var finds []linter.Find
	for _, root := range roots {
		files := journal.CollectFiles(root)
		allFiles = append(allFiles, files...)
		c := semantic.Build(files)
		finds = append(finds, s.linter.Run(c)...)

	}

	finds = dedupFinds(finds)
	s.semanticCtx = semantic.Build(allFiles)

	diagsByFile := s.groupFindsByFile(finds)
	activeFiles := s.activeFileSet(s.openDocs, s.loader.Ordered())
	for fpath := range activeFiles {
		if err := s.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
			URI:         uri.File(fpath),
			Diagnostics: diagsByFile[fpath],
		}); err != nil {
			s.log.Warn("publish diagnostics failed", "uri", uri.File(fpath), "err", err)
		}
	}

	s.log.Debug("workspace rebuild complete",
		"files", len(s.loader.Ordered()), "findings", len(finds))
}

func (s *server) scheduleWorkspaceRebuild(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.debouncer != nil {
		s.debouncer.Stop()
	}
	s.debouncer = time.AfterFunc(200*time.Millisecond, func() {
		s.rebuildWorkspace(jsonrpc2.DetachContext(ctx))
	})
}

func (s *server) cancelWorkspaceRebuild() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.debouncer != nil {
		s.debouncer.Stop()
		s.debouncer = nil
	}
}

func (s *server) activeFileSet(docs map[uri.URI]docState, ordered []*journal.ParsedFile) map[string]bool {
	active := make(map[string]bool)
	for duri := range docs {
		active[duri.Path()] = true
	}
	visited := make(map[string]bool)
	var walk func(*journal.ParsedFile)
	walk = func(pf *journal.ParsedFile) {
		if visited[pf.Path] {
			return
		}
		active[pf.Path] = true
		visited[pf.Path] = true
		for _, inc := range pf.Includes {
			walk(inc)
		}
	}
	byPath := make(map[string]*journal.ParsedFile, len(ordered))
	for _, pf := range ordered {
		byPath[pf.Path] = pf
	}
	for duri := range docs {
		if pf, ok := byPath[duri.Path()]; ok {
			walk(pf)
		}
	}
	return active
}

func (s *server) groupFindsByFile(finds []linter.Find) map[string][]protocol.Diagnostic {
	diags := make(map[string][]protocol.Diagnostic)
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
	seen := make(map[string]bool)
	dedup := make([]linter.Find, 0, len(finds))
	for _, f := range finds {
		s := f.Span.Start
		key := fmt.Sprintf("%s:%d:%d:%s", s.File, s.Line, s.Col, f.Code) // TODO: performace
		if seen[key] {
			continue
		}
		seen[key] = true
		dedup = append(dedup, f)
	}
	return dedup
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
