package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/linter"
	"olexsmir.xyz/clerk/internal/settings"
	"olexsmir.xyz/clerk/journal"
)

func (c *Cli) lintAction(ctx context.Context, cmd *cli.Command) error {
	format := cmd.String("format")
	pathStyle := cmd.String("path-style")

	configPath, err := resolveConfigPath(cmd)
	if err != nil {
		return err
	}
	set, warns, err := settings.Load(configPath)
	if err != nil {
		return err
	}
	for _, w := range warns {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}

	cfg := set.Linter
	lint, err := linter.NewLinter(cfg)
	if err != nil {
		return err
	}
	reporter := linter.NewReporter(os.Stdout, parsePathStyle(pathStyle), cfg)

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

	var hasFailures bool
	for _, f := range journalFiles {
		rj, err := loader.Resolve(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			hasFailures = true
			continue
		}
		reporter.Collect(lint.Run(analyzer.Build(rj)))
	}

	if err := reporter.Flush(format); err != nil {
		return fmt.Errorf("flushing report: %w", err)
	}

	if reporter.HasFailures() || hasFailures {
		return cli.Exit("", 1)
	}
	return nil
}

func (c *Cli) lintStdin(lint *linter.Linter, reporter *linter.Reporter, format string) error {
	src, err := readStdin()
	if err != nil {
		return err
	}
	loader := journal.NewLoader()
	rj := loader.ResolveBytes("stdin", src)
	reporter.Collect(lint.Run(analyzer.Build(rj)))

	if err := reporter.Flush(format); err != nil {
		return fmt.Errorf("flushing report: %w", err)
	}
	if reporter.HasFailures() {
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
