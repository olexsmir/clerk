package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/internal/analyzer"
)

const DuplicatedCommodityID RuleID = "duplicated-commodity"

// DuplicatedCommodity flags commodity declarations that appear more than once.
type DuplicatedCommodity struct{}

func (DuplicatedCommodity) ID() RuleID { return DuplicatedCommodityID }
func (d *DuplicatedCommodity) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for sym, info := range an.Commodities {
		if len(info.Directives) <= 1 {
			continue
		}
		for _, cd := range info.Directives {
			finds = append(finds, Find{
				Code:    d.ID(),
				Message: fmt.Sprintf("duplicated commodity declaration: %s", sym),
				Span:    cd.Span,
			})
		}
	}
	return finds
}
