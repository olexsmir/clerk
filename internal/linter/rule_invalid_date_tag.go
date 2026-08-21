package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/journal/parser"
)

const InvalidDateTagID RuleID = "invalid-date-tag"

// InvalidDateTag flags date: and date2: tag values that are not valid dates.
type InvalidDateTag struct{}

func (InvalidDateTag) ID() RuleID { return InvalidDateTagID }
func (i *InvalidDateTag) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, name := range []string{"date", "date2"} {
		info, ok := an.Tags[name]
		if !ok {
			continue
		}
		for _, usage := range info.Usage {
			if _, _, _, _, err := parser.ParseDateLiteral(usage.Tag.Value); err != nil {
				finds = append(finds, Find{
					Code:    i.ID(),
					Span:    usage.Tag.Span,
					Message: fmt.Sprintf("invalid %s: tag value %q", name, usage.Tag.Value),
				})
			}
		}
	}
	return finds
}
