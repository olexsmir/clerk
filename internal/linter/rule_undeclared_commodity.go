package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/journal/semantic"
)

// UndeclaredCommodity flags amounts that reference a commodity not declared via `commodity` directive.
type UndeclaredCommodity struct{}

func (UndeclaredCommodity) ID() RuleID         { return "undeclared-commodity" }
func (UndeclaredCommodity) Severity() Severity { return SeverityWarning }
func (r *UndeclaredCommodity) CheckJournal(ctx *semantic.Context) []Find {
	var finds []Find
	for name, info := range ctx.Commodities {
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
