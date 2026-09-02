package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/internal/analyzer"
)

const DuplicatedTagID = "duplicated-tag"

// DuplicatedTag flags tag declarations that appear more than once.
type DuplicatedTag struct{}

func (DuplicatedTag) ID() RuleID { return DuplicatedTagID }
func (d *DuplicatedTag) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for name, info := range an.Tags {
		if len(info.Directives) <= 1 {
			continue
		}
		for _, td := range info.Directives {
			finds = append(finds, Find{
				Code:    d.ID(),
				Message: fmt.Sprintf("duplicated tag declaration: %s", name),
				Span:    td.Span,
			})
		}
	}
	return finds
}
