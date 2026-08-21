package linter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"olexsmir.xyz/clerk/journal/token"
)

func mkFind(code RuleID, msg, file string, line, col int) Find {
	return Find{
		Code:    code,
		Message: msg,
		Span: token.Span{
			Start: token.Pos{File: file, Line: line, Col: col},
		},
	}
}

func TestReporter_Collect_AssignsSeverityFromConfig(t *testing.T) {
	cfg := Config{Rules: map[RuleID]RuleConfig{
		MissingPayeeID: {Severity: SeverityHint},
	}}
	r := NewReporter(&bytes.Buffer{}, PathBasename, cfg)

	finds := []Find{
		mkFind(MissingPayeeID, "transaction has no payee", "a.journal", 1, 1),
		mkFind(EmptyPostingsID, "transaction has no postings", "a.journal", 2, 1),
	}
	r.Collect(finds)

	if got, want := r.finds[0].Severity, SeverityHint; got != want {
		t.Errorf("overridden rule severity = %v, want %v", got, want)
	}
	if got, want := r.finds[1].Severity, Rules[EmptyPostingsID].Severity; got != want {
		t.Errorf("default rule severity = %v, want %v", got, want)
	}

	// Collect assigns severities in place on the slice passed in, since callers
	// (e.g. the cli) rely on this to build up findings across files.
	if finds[0].Severity != SeverityHint {
		t.Errorf("caller's slice was not mutated: got %v", finds[0].Severity)
	}
}

func TestReporter_Collect_Accumulates(t *testing.T) {
	r := NewReporter(&bytes.Buffer{}, PathBasename, Config{})
	r.Collect([]Find{mkFind(ParseErrorID, "e1", "a.journal", 1, 1)})
	r.Collect([]Find{mkFind(ParseErrorID, "e2", "b.journal", 2, 1)})

	if got, want := len(r.finds), 2; got != want {
		t.Fatalf("len(finds) = %d, want %d", got, want)
	}
}

func TestReporter_HasFailures(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		want     bool
	}{
		{"error", SeverityError, true},
		{"warning", SeverityWarning, true},
		{"info", SeverityInfo, false},
		{"hint", SeverityHint, false},
		// A finding whose rule has no configured/default severity (e.g. an
		// unregistered rule ID) resolves to SeverityNone, which is treated as
		// a failure rather than being silently ignored.
		{"none", SeverityNone, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &Reporter{finds: []Find{{Code: "some-rule", Severity: tc.severity}}}
			if got := r.HasFailures(); got != tc.want {
				t.Errorf("HasFailures() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReporter_HasFailures_NoFinds(t *testing.T) {
	r := NewReporter(&bytes.Buffer{}, PathBasename, Config{})
	if r.HasFailures() {
		t.Error("HasFailures() = true, want false when there are no findings")
	}
}

func TestReporter_HasFailures_MixedSeverities(t *testing.T) {
	r := &Reporter{finds: []Find{
		{Code: "a", Severity: SeverityInfo},
		{Code: "b", Severity: SeverityHint},
		{Code: "c", Severity: SeverityWarning},
	}}
	if !r.HasFailures() {
		t.Error("HasFailures() = false, want true when any finding is warning-or-worse")
	}
}

func TestReporter_Flush_Text(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf, PathBasename, Config{})
	r.Collect([]Find{mkFind(MissingPayeeID, "transaction has no payee", "/tmp/x/a.journal", 5, 1)})

	if err := r.Flush("text"); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	want := "a.journal:5:1: missing-payee: transaction has no payee\n"
	if buf.String() != want {
		t.Errorf("Flush(text) = %q, want %q", buf.String(), want)
	}
}

func TestReporter_Flush_Text_NoFindings(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf, PathBasename, Config{})

	if err := r.Flush("text"); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if buf.String() != "" {
		t.Errorf("Flush(text) with no findings = %q, want empty", buf.String())
	}
}

func TestReporter_Flush_JSON(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf, PathBasename, Config{})
	r.Collect([]Find{mkFind(MissingPayeeID, "transaction has no payee", "/tmp/x/a.journal", 5, 1)})

	if err := r.Flush("json"); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	var got []findJSON
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("Flush(json) produced invalid JSON: %v\noutput: %s", err, buf.String())
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	want := findJSON{
		Message:  "transaction has no payee",
		Severity: "warning",
		Code:     "missing-payee",
		File:     "a.journal",
		Line:     5,
		Column:   1,
	}
	if got[0] != want {
		t.Errorf("got %+v, want %+v", got[0], want)
	}
}

func TestReporter_Flush_UnsupportedFormat(t *testing.T) {
	r := NewReporter(&bytes.Buffer{}, PathBasename, Config{})
	if err := r.Flush("xml"); err == nil {
		t.Error("Flush(\"xml\") error = nil, want error for unsupported format")
	}
}

func TestReporter_Flush_OrdersFindingsByPosition(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf, PathBasename, Config{})
	r.Collect([]Find{
		mkFind(MissingPayeeID, "second", "a.journal", 5, 1),
		mkFind(EmptyPostingsID, "first", "a.journal", 1, 1),
	})

	if err := r.Flush("text"); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], "first") || !strings.Contains(lines[1], "second") {
		t.Errorf("findings not ordered by position:\n%s", buf.String())
	}
}

func TestNewReporter(t *testing.T) {
	var buf bytes.Buffer
	cfg := Config{Rules: map[RuleID]RuleConfig{MissingPayeeID: {Severity: SeverityInfo}}}
	r := NewReporter(&buf, PathAbsolute, cfg)

	if r.w != &buf {
		t.Error("NewReporter did not store the given writer")
	}
	if r.style != PathAbsolute {
		t.Errorf("style = %v, want %v", r.style, PathAbsolute)
	}
	if r.cfg.SeverityFor(MissingPayeeID) != SeverityInfo {
		t.Error("NewReporter did not store the given config")
	}
}