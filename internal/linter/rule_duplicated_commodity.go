package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/journal/semantic"
)

// DuplicatedCommodity flags commodity declarations that appear more than once.
type DuplicatedCommodity struct{}

func (DuplicatedCommodity) ID() RuleID         { return "duplicated-commodity" }
func (DuplicatedCommodity) Severity() Severity { return SeverityWarning }
func (d *DuplicatedCommodity) CheckJournal(ctx *semantic.Context) []Find {
	var finds []Find
	for sym, info := range ctx.Commodities {
		if len(info.Directives) <= 1 {
			continue
		}
		for _, cd := range info.Directives {
			finds = append(finds, Find{
				Code:     d.ID(),
				Severity: d.Severity(),
				Message:  fmt.Sprintf("duplicated commodity declaration: %s", sym),
				Span:     cd.Span,
			})
		}
	}
	return finds
}
