package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

// exitRecorder captures calls to cli.OsExiter so lintAction's cli.Exit(...)
// doesn't call os.Exit and kill the test binary.
type exitRecorder struct {
	called bool
	code   int
}

func recordExit(t *testing.T) *exitRecorder {
	t.Helper()
	rec := &exitRecorder{}
	orig := cli.OsExiter
	cli.OsExiter = func(code int) {
		rec.called = true
		rec.code = code
	}
	t.Cleanup(func() { cli.OsExiter = orig })
	return rec
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	_ = w.Close()
	os.Stdout = origStdout
	return <-done
}

func writeJournal(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "in.journal")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing journal: %v", err)
	}
	return path
}

func TestLintAction_CleanJournal(t *testing.T) {
	// Mirrors internal/linter/testdata/correct.txtar, which is asserted
	// elsewhere to produce zero findings under every rule.
	path := writeJournal(t, `account expenses:food
account assets:cash

commodity GBP
commodity 1,000.00 UAH
commodity 1000.00 USD

payee groceries

2024-05-01 * "groceries"
    expenses:food   50.00 USD
    assets:cash

2024-05-02 * "groceries"
    expenses:food   40.00 USD
    assets:cash
`)
	rec := recordExit(t)

	var runErr error
	out := captureStdout(t, func() {
		runErr = New("test").Run(context.Background(), []string{"clerk", "lint", path})
	})

	if runErr != nil {
		t.Fatalf("Run() error = %v, want nil", runErr)
	}
	if rec.called {
		t.Errorf("cli.OsExiter called with code %d, want not called", rec.code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
}

func TestLintAction_WarningFindingFails(t *testing.T) {
	path := writeJournal(t, `2024-05-01 *
    expenses:food   50.00 USD
    assets:cash
`)
	rec := recordExit(t)

	var runErr error
	out := captureStdout(t, func() {
		runErr = New("test").Run(context.Background(), []string{"clerk", "lint", path})
	})

	if runErr == nil {
		t.Fatal("Run() error = nil, want non-nil: a missing-payee warning must fail the lint")
	}
	if !rec.called {
		t.Fatal("cli.OsExiter was not called")
	}
	if rec.code != 1 {
		t.Errorf("exit code = %d, want 1", rec.code)
	}
	if !strings.Contains(out, "missing-payee") {
		t.Errorf("stdout = %q, want it to mention missing-payee", out)
	}
}

func TestLintAction_JSONFormat(t *testing.T) {
	path := writeJournal(t, `2024-05-01 *
    expenses:food   50.00 USD
    assets:cash
`)
	rec := recordExit(t)

	out := captureStdout(t, func() {
		_ = New("test").Run(context.Background(), []string{"clerk", "lint", "--format", "json", path})
	})
	if !rec.called || rec.code != 1 {
		t.Fatalf("exit called=%v code=%d, want called with code 1", rec.called, rec.code)
	}

	var findings []map[string]any
	if err := json.Unmarshal([]byte(out), &findings); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", err, out)
	}

	var found bool
	for _, f := range findings {
		if f["code"] == "missing-payee" {
			found = true
			if f["severity"] != "warning" {
				t.Errorf("severity = %v, want %q", f["severity"], "warning")
			}
		}
	}
	if !found {
		t.Errorf("missing-payee finding not present in JSON output: %s", out)
	}
}

func TestLintAction_Stdin(t *testing.T) {
	content := `2024-05-01 *
    expenses:food   50.00 USD
    assets:cash
`
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin })

	go func() {
		_, _ = w.Write([]byte(content))
		_ = w.Close()
	}()

	rec := recordExit(t)

	var runErr error
	out := captureStdout(t, func() {
		runErr = New("test").Run(context.Background(), []string{"clerk", "lint"})
	})

	if runErr == nil {
		t.Fatal("Run() error = nil, want non-nil for a stdin journal with a missing payee")
	}
	if !rec.called || rec.code != 1 {
		t.Errorf("exit called=%v code=%d, want called with code 1", rec.called, rec.code)
	}
	if !strings.Contains(out, "missing-payee") {
		t.Errorf("stdout = %q, want it to mention missing-payee", out)
	}
}