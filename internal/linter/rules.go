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

// Rules is list of all available rules.
var Rules = []Rule{
	&AccountDepthLimit{MaxDepth: 4},
	&DuplicatedAccount{},
	&DuplicatedCommodity{},
	&DuplicatedTag{},
	&DuplicatedTransaction{},
	&EmptyPostings{},
	&InvalidDateTag{},
	&InvalidInclude{},
	&InvalidTypeTag{},
	&MissingCommodity{},
	&MissingPayee{},
	&MissingStatus{},
	&MultipleOmittedAmounts{},
	&OmittedPrecision{},
	&OrderDate{},
	&ParseError{},
	&UnbalancedTransaction{},
	&UndeclaredAccount{},
	&UndeclaredCommodity{},
	&UndeclaredPayee{},
	&UndeclaredTag{},
	&UnusedAccount{},
	&UnusedTag{},
}

var defaultSeverities = map[RuleID]Severity{
	AccountDepthLimitID:      SeverityWarning,
	DuplicatedAccountID:      SeverityWarning,
	DuplicatedCommodityID:    SeverityWarning,
	DuplicatedTagID:          SeverityWarning,
	DuplicatedTransactionID:  SeverityWarning,
	EmptyPostingsID:          SeverityError,
	InvalidDateTagID:         SeverityError,
	InvalidIncludeID:         SeverityError,
	InvalidTypeTagID:         SeverityError,
	MissingCommodityID:       SeverityWarning,
	MissingPayeeID:           SeverityWarning,
	MissingStatusID:          SeverityWarning,
	MultipleOmittedAmountsID: SeverityError,
	OmittedPrecisionID:       SeverityWarning,
	OrderDateID:              SeverityWarning,
	ParseErrorID:             SeverityError,
	UnbalancedTransactionID:  SeverityError,
	UndeclaredAccountID:      SeverityWarning,
	UndeclaredCommodityID:    SeverityWarning,
	UndeclaredPayeeID:        SeverityWarning,
	UndeclaredTagID:          SeverityWarning,
	UnusedAccountID:          SeverityWarning,
	UnusedTagID:              SeverityWarning,
}
