package linter

import (
	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/journal/token"
)

// A Find represents a single lint finding.
type Find struct {
	Code     RuleID
	Severity Severity
	Message  string
	Span     token.Span
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
		if jc, ok := rule.(Rule); ok {
			finds = append(finds, jc.CheckJournal(a)...)
		}
	}

	return finds
}

// Severity represents severity of the litning find.
type Severity int

const (
	SeverityError   Severity = 1
	SeverityWarning Severity = 2
	SeverityInfo    Severity = 3
	SeverityHint    Severity = 4
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
