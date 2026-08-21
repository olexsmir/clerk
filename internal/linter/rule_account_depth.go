package linter

import (
	"bytes"
	"encoding/json"
	"fmt"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/journal/ast"
)

const AccountDepthLimitID = "account-depth"

// AccountDepthLimit checks that account names don't exceed MaxDepth
type AccountDepthLimit struct {
	MaxDepth int `json:"max-depth"`
}

func (AccountDepthLimit) ID() RuleID { return AccountDepthLimitID }
func (a *AccountDepthLimit) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find

	for _, txn := range an.Transactions {
		for _, p := range txn.Postings {
			a.check(&finds, p.Account)
		}
	}
	for _, ptx := range an.PeriodicTransactions {
		for _, p := range ptx.Postings {
			a.check(&finds, p.Account)
		}
	}
	for _, atx := range an.AutomatedTransactions {
		for _, p := range atx.Postings {
			a.check(&finds, p.Account)
		}
	}

	for _, d := range an.Directives {
		switch e := d.(type) {
		case *ast.AccountDirective:
			a.check(&finds, e.Account)
		case *ast.AliasDirective:
			a.check(&finds, e.From)
			a.check(&finds, e.To)
		}
	}

	return finds
}

func (a *AccountDepthLimit) Clone() Rule { cpy := *a; return &cpy }
func (a *AccountDepthLimit) UnmarshalOptions(data json.RawMessage) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return d.Decode(a)
}

func (a *AccountDepthLimit) check(finds *[]Find, acc ast.Account) {
	if depth := len(acc.Name); depth > a.MaxDepth {
		*finds = append(*finds, Find{
			Code: a.ID(),
			Message: fmt.Sprintf("account %q depth (%d) exceeds max allowed depth (%d)",
				acc.String(), depth, a.MaxDepth),
			Span: acc.Span,
		})
	}
}
