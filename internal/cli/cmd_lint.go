package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/linter"
	"olexsmir.xyz/clerk/journal"
)

func (c *Cli) lintAction(ctx context.Context, cmd *cli.Command) error {
	format := cmd.String("format")
	pathStyle := cmd.String("path-style")

	lint := linter.NewLinter(linter.Rules)
	reporter := linter.NewReporter(os.Stdout, parsePathStyle(pathStyle))

	journals := cmd.StringArgs("journals")
	if len(journals) == 0 {
		return c.lintStdin(lint, reporter, format)
	}

	journalFiles, err := resolvePaths(journals)
	if err != nil {
		return err
	}
	if len(journalFiles) == 0 {
		return nil
	}

	loader := journal.NewLoader()

	var hasIssues bool
	for _, f := range journalFiles {
		if _, err := loadFile(loader, f); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			hasIssues = true
		}
	}

	for _, root := range loader.Roots() {
		reporter.Collect(lint.Run(analyzer.Build(journal.CollectFiles(root))))
	}

	if err := reporter.Flush(format); err != nil {
		return fmt.Errorf("flushing report: %w", err)
	}

	if reporter.HasIssues() || hasIssues {
		return cli.Exit("", 1)
	}
	return nil
}

func (c *Cli) lintStdin(lint *linter.Linter, reporter *linter.Reporter, format string) error {
	loader := journal.NewLoader()
	if _, err := loadStdin(loader); err != nil {
		return err
	}

	for _, root := range loader.Roots() {
		reporter.Collect(lint.Run(analyzer.Build(journal.CollectFiles(root))))
	}

	if err := reporter.Flush(format); err != nil {
		return fmt.Errorf("flushing report: %w", err)
	}
	if reporter.HasIssues() {
		return cli.Exit("", 1)
	}
	return nil
}

func parsePathStyle(s string) linter.PathStyle {
	switch strings.ToLower(s) {
	case "basename":
		return linter.PathBasename
	case "absolute":
		return linter.PathAbsolute
	default:
		return linter.PathRelative
	}
}
