package linter

import "olexsmir.xyz/clerk/journal/ast"

// DuplicatedCommodity flags commodity declarations that appear more than once.
type DuplicatedCommodity struct{}

func (DuplicatedCommodity) ID() RuleID         { return "duplicated-commodity" }
func (DuplicatedCommodity) Severity() Severity { return SeverityWarning }
func (d *DuplicatedCommodity) CheckJournal(j *ast.Journal) []Find {
	var finds []Find
	seen := make(map[string]bool)

	for _, entry := range j.Entries {
		cd, ok := entry.(*ast.CommodityDirective)
		if !ok {
			continue
		}

		name := cd.Commodity
		if seen[name] {
			finds = append(finds, Find{
				Code:     d.ID(),
				Severity: d.Severity(),
				Message:  "duplicated commodity declaration: " + name,
				Span:     cd.Span,
			})
		}

		seen[name] = true
	}

	return finds
}
