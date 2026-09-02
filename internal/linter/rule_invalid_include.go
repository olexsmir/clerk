package linter

import (
	"fmt"
	"path/filepath"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/ast"
)

const InvalidIncludeID RuleID = "invalid-include"

// InvalidInclude flags include directives that don't point to an existing journal file.
type InvalidInclude struct{}

func (InvalidInclude) ID() RuleID { return InvalidIncludeID }
func (i *InvalidInclude) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, pf := range an.Files {
		for _, entry := range pf.Ast.Entries {
			inc, ok := entry.(*ast.IncludeDirective)
			if !ok {
				continue
			}
			target := filepath.Clean(filepath.Join(filepath.Dir(pf.Path), inc.Path))

			if !i.resolved(target, an.Files) {
				finds = append(finds, Find{
					Code:    i.ID(),
					Message: fmt.Sprintf("include not found: %s", inc.Path),
					Span:    inc.Span,
				})
				continue
			}
			if !journal.IsJournalFile(target) {
				finds = append(finds, Find{
					Code:    i.ID(),
					Message: fmt.Sprintf("include is not a journal file: %s", inc.Path),
					Span:    inc.Span,
				})
			}
		}
	}
	return finds
}

func (i *InvalidInclude) resolved(target string, files []*journal.ParsedFile) bool {
	for _, pf := range files {
		if ok, _ := filepath.Match(target, pf.Path); ok {
			return true
		}
	}
	return false
}
