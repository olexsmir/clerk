package linter

import (
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
	if rs, ok := c.Rules[rule]; ok && rs.Severity != SeverityNone {
		return rs.Severity
	}
	return Rules[rule].Severity
}

// Linter runs lint rules against a parsed journal.
type Linter struct {
	rules []Rule
}

// NewLinter creates a [Linter] with the given rules.
func NewLinter(rules []Rule) *Linter {
	return &Linter{rules: rules}
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
	SeverityNone Severity = iota
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
