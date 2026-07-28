package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/internal/analyzer"
)

// UndeclaredCommodity flags amounts that reference a commodity not declared via `commodity` directive.
type UndeclaredCommodity struct{}

func (UndeclaredCommodity) ID() RuleID         { return "undeclared-commodity" }
func (UndeclaredCommodity) Severity() Severity { return SeverityWarning }
func (r *UndeclaredCommodity) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for name, info := range an.Commodities {
		if len(info.Directives) > 0 {
			continue
		}
		for _, usage := range info.Usages {
			finds = append(finds, Find{
				Code:     r.ID(),
				Severity: r.Severity(),
				Span:     usage.Amount.Span,
				Message:  fmt.Sprintf("undeclared commodity: %s", name),
			})
		}
	}
	return finds
}
