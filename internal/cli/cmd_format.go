package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/printer"
)

func (c *Cli) formatAction(ctx context.Context, cmd *cli.Command) error {
	check := cmd.Bool("check")
	diff := cmd.Bool("diff")
	list := cmd.Bool("list")
	write := cmd.Bool("write")
	paths := cmd.StringArgs("journals")

	loader := journal.NewLoader()
	if len(paths) == 0 {
		pf, err := loadStdin(loader)
		if err != nil {
			return err
		}
		if len(pf.Errors) > 0 || len(pf.FileErrors) > 0 {
			for _, e := range pf.Errors {
				fmt.Fprintf(os.Stderr, "error: stdin: %s\n", e.Message)
			}
			for _, fe := range pf.FileErrors {
				fmt.Fprintf(os.Stderr, "error: stdin: %s\n", fe.Message)
			}
			return fmt.Errorf("stdin: has errors, refusing to format")
		}
		return c.formatFile("stdin", pf, check, diff, list, write)
	}

	files, err := resolvePaths(paths)
	if err != nil {
		return err
	}

	var errs []error
	for _, path := range files {
		pf, err := loadFile(loader, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			errs = append(errs, err)
			continue
		}
		if len(pf.Errors) > 0 || len(pf.FileErrors) > 0 {
			for _, e := range pf.Errors {
				fmt.Fprintf(os.Stderr, "error: %s: %s\n", path, e.Message)
			}
			for _, fe := range pf.FileErrors {
				fmt.Fprintf(os.Stderr, "error: %s: %s\n", path, fe.Message)
			}
			errs = append(errs, fmt.Errorf("%s: has errors, refusing to format", path))
			continue
		}

		if err := c.formatFile(path, pf, check, diff, list, write); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("format: %d error(s)", len(errs))
	}
	return nil
}

func (c *Cli) formatFile(path string, pf *journal.ParsedFile, check, wantDiff, list, write bool) error {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, pf.Ast); err != nil {
		return fmt.Errorf("format: %w", err)
	}
	formatted := buf.Bytes()
	changed := !bytes.Equal(pf.Src, formatted)

	switch {
	case check:
		if changed {
			return fmt.Errorf("not formatted")
		}
	case list:
		if changed {
			fmt.Println(path)
		}
	case wantDiff:
		if changed {
			diffLines(path, pf.Src, formatted)
		}
	case write:
		if changed {
			if err := os.WriteFile(path, formatted, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}
		}
	default:
		if _, err := os.Stdout.Write(formatted); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
	}
	return nil
}

func diffLines(path string, src, formatted []byte) {
	fmt.Printf("--- %s\n+++ %s\n", path, path)
	srcLines := bytes.Split(src, []byte("\n"))
	fmtLines := bytes.Split(formatted, []byte("\n"))
	lines := max(len(fmtLines), len(srcLines))
	for i := range lines {
		var sLine, fLine []byte
		if i < len(srcLines) {
			sLine = srcLines[i]
		}
		if i < len(fmtLines) {
			fLine = fmtLines[i]
		}
		if !bytes.Equal(sLine, fLine) {
			if len(sLine) > 0 {
				fmt.Printf("-%s\n", sLine)
			} else {
				fmt.Println("-")
			}
			if len(fLine) > 0 {
				fmt.Printf("+%s\n", fLine)
			} else {
				fmt.Println("+")
			}
		}
	}
}
