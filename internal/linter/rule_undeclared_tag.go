package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/internal/analyzer"
)

const UndeclaredTagID RuleID = "undeclared-tag"

// UndeclaredTag flags used tag that's not declared via `tag` directive.
type UndeclaredTag struct{}

func (UndeclaredTag) ID() RuleID { return UndeclaredTagID }
func (u *UndeclaredTag) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for name, info := range an.Tags {
		if len(info.Directives) > 0 {
			continue
		}
		// default tags
		if name == "date" || name == "date2" || name == "type" {
			continue
		}
		for _, usage := range info.Usage {
			finds = append(finds, Find{
				Code:    u.ID(),
				Span:    usage.Tag.Span,
				Message: fmt.Sprintf("undeclared tag: %s", name),
			})
		}
	}
	return finds
}
