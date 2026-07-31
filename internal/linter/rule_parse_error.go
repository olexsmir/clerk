package linter

import "olexsmir.xyz/clerk/internal/analyzer"

// ParseError wraps parser errors into lint findings.
type ParseError struct{}

func (ParseError) ID() RuleID         { return "parse-error" }
func (ParseError) Severity() Severity { return SeverityError }
func (p *ParseError) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, pf := range an.Files {
		for _, err := range pf.Errors {
			finds = append(finds, Find{
				Code:     p.ID(),
				Severity: p.Severity(),
				Message:  err.Message,
				Span:     err.Span,
			})
		}
	}
	return finds
}
