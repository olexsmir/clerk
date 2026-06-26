package linter

import (
	"olexsmir.xyz/clerk/journal/ast"

	"olexsmir.xyz/clerk/journal/semantic"
)

type RuleID string

// Rule is the best interface that every rule must implement.
type Rule interface {
	ID() RuleID
	Severity() Severity
}

// EntryChecker implements per-entry linting.
type EntryChecker interface {
	Rule
	CheckEntry(entry ast.Entry) []Find
}

// JournalChecker implements whole-journal linting using the semantic context.
type JournalChecker interface {
	Rule
	CheckJournal(ctx *semantic.Context) []Find
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
	&DuplicatedCommodity{},
	&UndeclaredAccount{},
}
