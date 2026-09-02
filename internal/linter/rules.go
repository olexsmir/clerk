package linter

import (
	"encoding/json"

	"olexsmir.xyz/clerk/internal/analyzer"
)

type RuleID string

// Rule is the best interface that every rule must implement.
type Rule interface {
	ID() RuleID
	CheckJournal(an *analyzer.Analysis) []Find
}

type RuleConfig struct {
	// Disabled turns the rule off entirely.
	Disabled bool

	// Severity overrides the rule's default severity level.
	Severity Severity // [SeverityNone] if [Disabled] is true

	// Options holds JSON-encoded options, already validated against the rule.
	Options json.RawMessage // TODO: check if there's better options than json
}

type RuleOptioner interface {
	UnmarshalOptions(data json.RawMessage) error
	Clone() Rule
}

type builtinRule struct {
	Rule     Rule
	Severity Severity
}

// Rules maps every rule ID to its implementation and default severity.
var Rules = map[RuleID]builtinRule{
	AccountDepthLimitID:      {&AccountDepthLimit{MaxDepth: 4}, SeverityWarning},
	DuplicatedAccountID:      {&DuplicatedAccount{}, SeverityWarning},
	DuplicatedCommodityID:    {&DuplicatedCommodity{}, SeverityWarning},
	DuplicatedTagID:          {&DuplicatedTag{}, SeverityWarning},
	DuplicatedTransactionID:  {&DuplicatedTransaction{}, SeverityWarning},
	EmptyPostingsID:          {&EmptyPostings{}, SeverityError},
	InvalidDateTagID:         {&InvalidDateTag{}, SeverityError},
	InvalidIncludeID:         {&InvalidInclude{}, SeverityError},
	InvalidTypeTagID:         {&InvalidTypeTag{}, SeverityError},
	MissingCommodityID:       {&MissingCommodity{}, SeverityWarning},
	MissingPayeeID:           {&MissingPayee{}, SeverityWarning},
	MissingStatusID:          {&MissingStatus{}, SeverityWarning},
	MultipleOmittedAmountsID: {&MultipleOmittedAmounts{}, SeverityError},
	OmittedPrecisionID:       {&OmittedPrecision{}, SeverityWarning},
	OrderDateID:              {&OrderDate{}, SeverityWarning},
	ParseErrorID:             {&ParseError{}, SeverityError},
	UnbalancedTransactionID:  {&UnbalancedTransaction{}, SeverityError},
	UndeclaredAccountID:      {&UndeclaredAccount{}, SeverityWarning},
	UndeclaredCommodityID:    {&UndeclaredCommodity{}, SeverityWarning},
	UndeclaredPayeeID:        {&UndeclaredPayee{}, severityNone},
	UndeclaredTagID:          {&UndeclaredTag{}, severityNone},
	UnusedAccountID:          {&UnusedAccount{}, SeverityWarning},
	UnusedTagID:              {&UnusedTag{}, SeverityWarning},
}
