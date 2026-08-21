package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/decimal"
	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/token"
)

const UnbalancedTransactionID RuleID = "unbalanced-transaction"

// UnbalancedTransaction flags transactions whose postings don't balance to zero.
type UnbalancedTransaction struct{}

func (UnbalancedTransaction) ID() RuleID { return UnbalancedTransactionID }
func (u *UnbalancedTransaction) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, txn := range an.Transactions {
		finds = append(finds, u.check(txn.Postings, txn.Span)...)
	}
	for _, ptx := range an.PeriodicTransactions {
		finds = append(finds, u.check(ptx.Postings, ptx.Span)...)
	}
	for _, atx := range an.AutomatedTransactions {
		finds = append(finds, u.check(atx.Postings, atx.Span)...)
	}
	return finds
}

func (u *UnbalancedTransaction) check(postings []*ast.Posting, span token.Span) []Find {
	var hasExpr, hasCost bool
	var realPostings []*ast.Posting
	autoBalancingPostings := 0

	for _, posting := range postings {
		if posting.Type != ast.PostingReal {
			continue
		}
		realPostings = append(realPostings, posting)
		if posting.Amount == nil {
			autoBalancingPostings++
		} else {
			if posting.Amount.IsExpr {
				hasExpr = true
			}
			if posting.Cost != nil {
				hasCost = true
			}
		}
	}

	if len(realPostings) == 0 ||
		autoBalancingPostings == 1 ||
		autoBalancingPostings > 1 ||
		hasExpr || hasCost {
		return nil
	}

	sums := make(map[string]decimal.Decimal)
	for _, posting := range realPostings {
		if posting.Amount == nil {
			continue
		}
		sums[posting.Amount.Commodity] = sums[posting.Amount.Commodity].Add(posting.Amount.Quantity)
	}

	var finds []Find
	for commodity, sum := range sums {
		if !sum.IsZero() {
			var msg string
			if commodity != "" {
				msg = fmt.Sprintf("transaction is unbalanced; %s balance is %s", commodity, sum.String())
			} else {
				msg = fmt.Sprintf("transaction is unbalanced; net balance is %s", sum.String())
			}
			finds = append(finds, Find{
				Code:    u.ID(),
				Span:    span,
				Message: msg,
			})
		}
	}
	return finds
}
