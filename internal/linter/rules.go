package linter

import "olexsmir.xyz/clerk/journal/ast"

type RuleID string

// Rule is the best interface that every rule must implement.
type Rule interface {
	ID() RuleID
	Severity() Severity
}

// EntryChecker implements pre entry linting during.
type EntryChecker interface {
	Rule
	CheckEntry(entry ast.Entry) []Find
}

// JournalChecker implements whole journal linting.
type JournalChecker interface {
	Rule
	CheckJournal(journal *ast.Journal) []Find
}

// Rules is list of all available rules.
var Rules = []Rule{
	&ParseError{},
	&EmptyPostings{},
	&OmittedPrecision{},
	&MissingCommodity{},
	&MissingStatus{},
	&AccountDepthLimit{MaxDepth: 4},
	&MultipleOmittedAmounts{},
	&OrderDate{},
	&DuplicatedAccount{},
}
