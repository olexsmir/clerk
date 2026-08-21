package linter

import "olexsmir.xyz/clerk/internal/analyzer"

const ParseErrorID RuleID = "parse-error"

// ParseError wraps parser errors into lint findings.
type ParseError struct{}

func (ParseError) ID() RuleID { return ParseErrorID }
func (p *ParseError) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, pf := range an.Files {
		for _, err := range pf.Errors {
			finds = append(finds, Find{
				Code:    p.ID(),
				Message: err.Message,
				Span:    err.Span,
			})
		}
	}
	return finds
}
