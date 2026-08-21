package linter

import "testing"

// TestRuleIDs guards against accidentally renaming the exported RuleID
// constants (used as keys in [Config.Rules] and the built-in [Rules] map) or
// having a rule's ID() method drift from its declared constant.
func TestRuleIDs(t *testing.T) {
	cases := []struct {
		rule Rule
		id   RuleID // exported constant, e.g. AccountDepthLimitID
		want string // literal value the constant must have
	}{
		{&AccountDepthLimit{}, AccountDepthLimitID, "account-depth"},
		{&DuplicatedAccount{}, DuplicatedAccountID, "duplicated-account"},
		{&DuplicatedCommodity{}, DuplicatedCommodityID, "duplicated-commodity"},
		{&DuplicatedTag{}, DuplicatedTagID, "duplicated-tag"},
		{&DuplicatedTransaction{}, DuplicatedTransactionID, "duplicated-transaction"},
		{&EmptyPostings{}, EmptyPostingsID, "empty-postings"},
		{&InvalidDateTag{}, InvalidDateTagID, "invalid-date-tag"},
		{&InvalidInclude{}, InvalidIncludeID, "invalid-include"},
		{&InvalidTypeTag{}, InvalidTypeTagID, "invalid-type-tag"},
		{&MissingCommodity{}, MissingCommodityID, "missing-commodity"},
		{&MissingPayee{}, MissingPayeeID, "missing-payee"},
		{&MissingStatus{}, MissingStatusID, "missing-status"},
		{&MultipleOmittedAmounts{}, MultipleOmittedAmountsID, "multiple-omitted-amounts"},
		{&OmittedPrecision{}, OmittedPrecisionID, "omitted-precision"},
		{&OrderDate{}, OrderDateID, "orderdate"},
		{&ParseError{}, ParseErrorID, "parse-error"},
		{&UnbalancedTransaction{}, UnbalancedTransactionID, "unbalanced-transaction"},
		{&UndeclaredAccount{}, UndeclaredAccountID, "undeclared-account"},
		{&UndeclaredCommodity{}, UndeclaredCommodityID, "undeclared-commodity"},
		{&UndeclaredPayee{}, UndeclaredPayeeID, "undeclared-payee"},
		{&UndeclaredTag{}, UndeclaredTagID, "undeclared-tag"},
	}

	seen := make(map[RuleID]bool, len(cases))
	for _, c := range cases {
		if string(c.id) != c.want {
			t.Errorf("%T: constant = %q, want %q", c.rule, c.id, c.want)
		}
		if got := c.rule.ID(); got != c.id {
			t.Errorf("%T.ID() = %q, want %q", c.rule, got, c.id)
		}
		if _, ok := Rules[c.id]; !ok {
			t.Errorf("%T: %q missing from built-in Rules map", c.rule, c.id)
		}
		if seen[c.id] {
			t.Errorf("duplicate RuleID in test table: %q", c.id)
		}
		seen[c.id] = true
	}
}