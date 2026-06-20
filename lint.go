package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"olexsmir.xyz/clerk/internal/linter"
	"olexsmir.xyz/clerk/journal"
)

func runLint(args []string) {
	fs := flag.NewFlagSet("lint", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text, json")
	pathStyle := fs.String("path-style", "relative", "file path style: basename, relative, absolute")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: clerk lint [flags] [path...]\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	style := parsePathStyle(*pathStyle)
	l := linter.NewLinter(linter.Rules)
	reporter := linter.NewReporter(os.Stdout, style)
	paths := fs.Args()

	if len(paths) == 0 {
		finds := lintStdin(l)
		reporter.Collect(finds)
		reporter.Flush(*format)
		if hasFatal(finds) {
			os.Exit(1)
		}
		os.Exit(0)
	}

	exitCode := 0
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
			exitCode = 1
			continue
		}
		if info.IsDir() {
			filepath.Walk(path, func(fpath string, finfo os.FileInfo, err error) error {
				if err != nil || finfo.IsDir() || !journal.IsJournalFile(fpath) {
					return nil
				}
				finds, fatal := lintFile(l, fpath)
				reporter.Collect(finds)
				if fatal {
					exitCode = 1
				}
				return nil
			})
			continue
		}
		finds, fatal := lintFile(l, path)
		reporter.Collect(finds)
		if fatal {
			exitCode = 1
		}
	}
	reporter.Flush(*format)
	os.Exit(exitCode)
}

func lintFile(l *linter.Linter, path string) ([]linter.Find, bool) {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
		return nil, true
	}
	pf, err := journal.NewLoader().LoadBytes(path, src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
		return nil, true
	}
	finds := l.Run(pf.Ast)
	return finds, hasFatal(finds)
}

func lintStdin(l *linter.Linter) []linter.Find {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		os.Exit(1)
	}
	pf, err := journal.NewLoader().LoadBytes("stdin", src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return l.Run(pf.Ast)
}

func hasFatal(finds []linter.Find) bool {
	for _, f := range finds {
		if f.Severity <= 2 {
			return true
		}
	}
	return false
}

func parsePathStyle(s string) linter.PathStyle {
	switch strings.ToLower(s) {
	default:
		return linter.PathRelative
	case "basename":
		return linter.PathBasename
	case "absolute":
		return linter.PathAbsolute
	}
}
