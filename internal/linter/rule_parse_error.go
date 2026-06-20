package linter

import "olexsmir.xyz/clerk/journal/ast"

// ParseError wraps parser errors into lint findings.
type ParseError struct{}

func (ParseError) ID() RuleID         { return "parse-error" }
func (ParseError) Severity() Severity { return SeverityError }
func (e *ParseError) CheckJournal(j *ast.Journal) []Find {
	var finds []Find
	for _, err := range j.Errors {
		finds = append(finds, Find{
			Code:     e.ID(),
			Severity: e.Severity(),
			Message:  err.Message,
			Span:     err.Span,
		})
	}
	return finds
}
