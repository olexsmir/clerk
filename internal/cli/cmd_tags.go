package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v3"

	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/tags"
)

func (c *Cli) tagsAction(ctx context.Context, cmd *cli.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	journals := cmd.StringArgs("journals")
	if len(journals) == 0 {
		journals = []string{"."}
	}

	journalFiles, err := resolvePaths(journals)
	if err != nil {
		return err
	}

	if len(journalFiles) == 0 {
		return fmt.Errorf("no journal files found")
	}

	loader := journal.NewLoader()

	var errs []error
	for _, file := range journalFiles {
		if _, err := loadFile(loader, file); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("tags: %d errors", len(errs))
	}

	tagger := tags.New(loader, cwd)

	out := cmd.String("out")
	var w io.Writer = os.Stdout
	if out != "-" {
		f, err := os.Create(out)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer f.Close()
		w = f
	}

	if err := tagger.Write(w); err != nil {
		return fmt.Errorf("writing tags: %w", err)
	}

	return nil
}
