package linter

import "olexsmir.xyz/clerk/internal/analyzer"

type RuleID string

// Rule is the best interface that every rule must implement.
type Rule interface {
	ID() RuleID
	Severity() Severity
	CheckJournal(an *analyzer.Analysis) []Find
}

// Rules is list of all available rules.
var Rules = []Rule{
	&ParseError{},
	&EmptyPostings{},
	&OmittedPrecision{},
	&MissingCommodity{},
	&MissingStatus{},
	&MissingPayee{},
	&AccountDepthLimit{MaxDepth: 4},
	&MultipleOmittedAmounts{},
	&OrderDate{},
	&DuplicatedAccount{},
	&DuplicatedCommodity{},
	&UndeclaredCommodity{},
	&UndeclaredAccount{},
	&UnbalancedTransaction{},
	// &UnusedAccount{},
}
