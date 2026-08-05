package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/internal/analyzer"
)

// UnusedTag flags declared tags that are not used.
type UnusedTag struct{}

func (UnusedTag) ID() RuleID         { return "unused-tag" }
func (UnusedTag) Severity() Severity { return SeverityWarning }
func (u *UnusedTag) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for name, info := range an.Tags {
		if len(info.Directives) == 0 {
			continue
		}
		if len(info.Usage) > 0 {
			continue
		}
		for _, d := range info.Directives {
			finds = append(finds, Find{
				Code:     u.ID(),
				Severity: u.Severity(),
				Span:     d.Span,
				Message:  fmt.Sprintf("unused tag: %s", name),
			})
		}
	}
	return finds
}
