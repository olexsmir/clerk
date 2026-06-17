package linter

import "olexsmir.xyz/clerk/journal/ast"

type RuleID string

// Rule is the best interface that every rule must implement.
type Rule interface {
	ID() RuleID
	Severity() Severity
	Description() string
}

// EntryChecker implements pre entry linting during.
type EntryChecker interface {
	CheckEntry(entry ast.Entry) []Find
	Rule
}

// JournalChecker implements whole journal linting.
type JournalChecker interface {
	CheckJournal(journal *ast.Journal) []Find
	Rule
}

// Rules is list of all available rules.
var Rules = []Rule{
	&ParseError{},
	&EmptyPostings{},
	&OmittedPrecision{},
	&MissingCommodity{},
	&MissingStatus{},
	&AccountDepthLimit{MaxDepth: 4},
}
