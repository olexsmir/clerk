package linter

import (
	"fmt"
	"slices"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/journal/token"
)

// A Find represents a single lint finding.
type Find struct {
	Code     RuleID
	Severity Severity // set during reporting
	Message  string
	Span     token.Span
}

// Config configures linter.
type Config struct {
	Rules map[RuleID]RuleConfig
}

// SeverityFor returns the severity for a rule. Returns config override if set,
// otherwise the rule's default from [Rules]
func (c Config) SeverityFor(rule RuleID) Severity {
	if rs, ok := c.Rules[rule]; ok && rs.Severity != severityNone {
		return rs.Severity
	}
	return Rules[rule].Severity
}

// Linter runs lint rules against a parsed journal.
type Linter struct {
	rules []Rule
}

// NewLinter creates a [Linter] with all built-in [Rules] configured by cfg.
// Disabled rules are omitted and options are applied to rule copies.
// Rules run in ID order for determinism.
func NewLinter(cfg Config) (*Linter, error) {
	ids := make([]RuleID, 0, len(Rules))
	for id := range Rules {
		if rc, ok := cfg.Rules[id]; ok && rc.Disabled {
			continue
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)

	var rules []Rule
	for _, id := range ids {
		rule := Rules[id].Rule
		rc := cfg.Rules[id]
		if len(rc.Options) > 0 {
			o, ok := rule.(RuleOptioner)
			if !ok {
				return nil, fmt.Errorf("rule %q does not accept options", rule.ID())
			}
			clone := o.Clone()
			if err := clone.(RuleOptioner).UnmarshalOptions(rc.Options); err != nil {
				return nil, fmt.Errorf("configuring rule %q: %w", rule.ID(), err)
			}
			rule = clone
		}
		rules = append(rules, rule)
	}
	return &Linter{rules: rules}, nil
}

// Run runs all rules against the analysis context.
func (l *Linter) Run(a *analyzer.Analysis) []Find {
	var finds []Find
	for _, rule := range l.rules {
		finds = append(finds, rule.CheckJournal(a)...)
	}
	return finds
}

// Severity represents severity of the litning find.
type Severity int

const (
	severityNone Severity = iota
	SeverityError
	SeverityWarning
	SeverityInfo
	SeverityHint
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	case SeverityHint:
		return "hint"
	}
	panic("impossible severity state")
}

func ParseSeverity(s string) (sev Severity, ok bool) {
	switch s {
	case "error":
		return SeverityError, true
	case "warn", "warning":
		return SeverityWarning, true
	case "info":
		return SeverityInfo, true
	case "hint":
		return SeverityHint, true
	}
	return severityNone, false
}
