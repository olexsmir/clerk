package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/journal/ast"
)

// AccountDepthLimit checks that account names don't exceed MaxDepth
type AccountDepthLimit struct {
	MaxDepth int
}

func (AccountDepthLimit) ID() RuleID          { return "account-depth" }
func (AccountDepthLimit) Severity() Severity  { return SeverityWarning }
func (a *AccountDepthLimit) CheckEntry(entry ast.Entry) []Find {
	var finds []Find
	switch e := entry.(type) {
	case *ast.Transaction:
		for _, p := range e.Postings {
			a.check(&finds, p.Account)
		}
	case *ast.PeriodicTransaction:
		for _, p := range e.Postings {
			a.check(&finds, p.Account)
		}
	case *ast.AutomatedTransaction:
		for _, p := range e.Postings {
			a.check(&finds, p.Account)
		}
	case *ast.AccountDirective:
		a.check(&finds, e.Account)
	case *ast.AliasDirective:
		a.check(&finds, e.From)
		a.check(&finds, e.To)
	}
	return finds
}

func (a *AccountDepthLimit) check(finds *[]Find, acc ast.Account) {
	if depth := len(acc.Name); depth > a.MaxDepth {
		*finds = append(*finds, Find{
			Code:     a.ID(),
			Severity: a.Severity(),
			Message: fmt.Sprintf("account %q depth (%d) exceeds max allowed depth (%d)",
				acc.String(), depth, a.MaxDepth),
			Span: acc.Span,
		})
	}
}
