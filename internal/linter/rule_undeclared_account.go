package linter

import "olexsmir.xyz/clerk/journal/ast"

// UndeclaredAccount flags postings that reference an account not declared via `account` directive.
type UndeclaredAccount struct{}

func (UndeclaredAccount) ID() RuleID         { return "undeclared-account" }
func (UndeclaredAccount) Severity() Severity { return SeverityWarning }
func (r *UndeclaredAccount) CheckJournal(j *ast.Journal) []Find {
	declared := make(map[string]bool)

	var finds []Find
	for _, entry := range j.Entries {
		switch e := entry.(type) {
		case *ast.AccountDirective:
			declared[e.Account.String()] = true
		case *ast.Transaction:
			for _, p := range e.Postings {
				name := p.Account.String()
				if !declared[name] {
					finds = append(finds, Find{
						Code:     r.ID(),
						Severity: r.Severity(),
						Message:  "undeclared account: " + name,
						Span:     p.Account.Span,
					})
				}
			}
		}
	}

	return finds
}
