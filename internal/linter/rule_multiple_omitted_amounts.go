package linter

import "olexsmir.xyz/clerk/journal/ast"

// MultipleOmittedAmounts flags entries where more than one posting has an ommited amount.
type MultipleOmittedAmounts struct{}

func (MultipleOmittedAmounts) ID() RuleID         { return "multiple-omitted-amounts" }
func (MultipleOmittedAmounts) Severity() Severity { return SeverityError }
func (m *MultipleOmittedAmounts) CheckEntry(entry ast.Entry) []Find {
	switch e := entry.(type) {
	case *ast.Transaction:
		return m.check(e.Postings)
	case *ast.PeriodicTransaction:
		return m.check(e.Postings)
	case *ast.AutomatedTransaction:
		return m.check(e.Postings)
	default:
		return nil
	}
}

func (m *MultipleOmittedAmounts) check(postings []*ast.Posting) []Find {
	var finds []Find
	for _, p := range postings {
		// skipping postings with balance assertion, since those are often used as reconciliation entries
		if p.Amount == nil && p.Balance == nil {
			finds = append(finds, Find{
				Code:     m.ID(),
				Severity: m.Severity(),
				Message:  "more than one posting has omitted amount",
				Span:     p.Span,
			})
		}
	}
	if len(finds) > 1 {
		return finds
	}
	return nil
}
