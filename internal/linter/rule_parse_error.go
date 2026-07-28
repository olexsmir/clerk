package linter

import "olexsmir.xyz/clerk/internal/analyzer"

// ParseError wraps parser errors into lint findings.
type ParseError struct{}

func (ParseError) ID() RuleID         { return "parse-error" }
func (ParseError) Severity() Severity { return SeverityError }
func (e *ParseError) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, pf := range an.Files {
		for _, err := range pf.Errors {
			finds = append(finds, Find{
				Code:     e.ID(),
				Severity: e.Severity(),
				Message:  err.Message,
				Span:     err.Span,
			})
		}
	}
	return finds
}
