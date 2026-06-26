package linter

import "olexsmir.xyz/clerk/journal/ast"

// DuplicatedAccount flags account declarations that appear more than once.
type DuplicatedAccount struct{}

func (DuplicatedAccount) ID() RuleID         { return "duplicated-account" }
func (DuplicatedAccount) Severity() Severity { return SeverityWarning }
func (d *DuplicatedAccount) CheckJournal(j *ast.Journal) []Find {
	var finds []Find
	seen := make(map[string]bool)

	for _, entry := range j.Entries {
		ad, ok := entry.(*ast.AccountDirective)
		if !ok {
			continue
		}

		name := ad.Account.String()
		if seen[name] {
			finds = append(finds, Find{
				Code:     d.ID(),
				Severity: d.Severity(),
				Message:  "duplicated account declaration: " + name,
				Span:     ad.Account.Span,
			})
		}
		seen[name] = true
	}
	return finds
}
