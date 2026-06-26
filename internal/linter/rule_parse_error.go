package linter

import "olexsmir.xyz/clerk/journal/semantic"

// ParseError wraps parser errors into lint findings.
type ParseError struct{}

func (ParseError) ID() RuleID         { return "parse-error" }
func (ParseError) Severity() Severity { return SeverityError }
func (e *ParseError) CheckJournal(ctx *semantic.Context) []Find {
	var finds []Find
	for _, pf := range ctx.Files {
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
