package linter

import (
	"fmt"

	"olexsmir.xyz/clerk/internal/decimal"
	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/token"
)

// UnbalancedTransaction flags transactions whose postings don't balance to zero.
type UnbalancedTransaction struct{}

func (UnbalancedTransaction) ID() RuleID         { return "unbalanced-transaction" }
func (UnbalancedTransaction) Severity() Severity { return SeverityError }
func (r *UnbalancedTransaction) CheckEntry(entry ast.Entry) []Find {
	switch e := entry.(type) {
	case *ast.Transaction:
		return r.check(e.Postings, e.Span)
	case *ast.PeriodicTransaction:
		return r.check(e.Postings, e.Span)
	case *ast.AutomatedTransaction:
		return r.check(e.Postings, e.Span)
	default:
		return nil
	}
}

func (r *UnbalancedTransaction) check(postings []*ast.Posting, span token.Span) []Find {
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
		// multiple auto-balancing postings is alreayd handled by [MultipleOmittedAmounts]
		autoBalancingPostings > 1 ||
		// too complex too verify
		hasExpr || hasCost {
		return nil
	}

	// sum amounts per commodity
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
				Code:     r.ID(),
				Severity: r.Severity(),
				Span:     span,
				Message:  msg,
			})
		}
	}
	return finds
}
