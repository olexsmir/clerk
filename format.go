package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/printer"
)

func runFormat(args []string) {
	fs := flag.NewFlagSet("format", flag.ExitOnError)
	check := fs.Bool("c", false, "Exit code 0 if already formatted, 1 otherwise")
	diff := fs.Bool("d", false, "Display diffs instead of rewriting files")
	list := fs.Bool("l", false, "List files whose formatting differs")
	write := fs.Bool("w", false, "Write result back to file instead of stdout")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: clerk format [flags] [path ...]\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	paths := fs.Args()

	// Read from stdin if no paths given
	if len(paths) == 0 {
		src, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
			os.Exit(1)
		}

		pf, err := journal.NewLoader().LoadBytes("stdin", src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
			os.Exit(1)
		}
		if len(pf.Errors) > 0 {
			fmt.Fprintf(os.Stderr, "parse error: %v\n", pf.Errors[0].Message)
			os.Exit(1)
		}

		var buf bytes.Buffer
		if err := printer.Fprint(&buf, pf.Ast); err != nil {
			fmt.Fprintf(os.Stderr, "format error: %v\n", err)
			os.Exit(1)
		}

		formatted := buf.Bytes()

		switch {
		case *check:
			if bytes.Equal(src, formatted) {
				os.Exit(0)
			}
			os.Exit(1)
		case *diff:
			diffLines("stdin", src, formatted)
		default:
			os.Stdout.Write(formatted)
		}
		return
	}

	// Process each file
	exitCode := 0
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
			exitCode = 1
			continue
		}
		if info.IsDir() {
			fmt.Fprintf(os.Stderr, "error: %s: is a directory\n", path)
			exitCode = 1
			continue
		}

		src, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
			exitCode = 1
			continue
		}

		pf, err := journal.NewLoader().LoadBytes(path, src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
			exitCode = 1
			continue
		}
		if len(pf.Errors) > 0 || len(pf.FileErrors) > 0 {
			fmt.Fprintf(os.Stderr, "error: %s: has errors, refusing to format\n", path)
			exitCode = 1
			continue
		}

		var buf bytes.Buffer
		if err := printer.Fprint(&buf, pf.Ast); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
			exitCode = 1
			continue
		}

		formatted := buf.Bytes()
		changed := !bytes.Equal(src, formatted)

		switch {
		case *check:
			if changed {
				fmt.Fprintf(os.Stderr, "%s: not formatted\n", path)
				exitCode = 1
			}
		case *list:
			if changed {
				fmt.Println(path)
			}
		case *diff:
			if changed {
				diffLines(path, src, formatted)
			}
		case *write:
			if changed {
				if err := os.WriteFile(path, formatted, 0o644); err != nil {
					fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
					exitCode = 1
				}
			}
		default:
			if _, err := os.Stdout.Write(formatted); err != nil {
				fmt.Fprintf(os.Stderr, "error writing stdout: %v\n", err)
				exitCode = 1
			}
		}
	}
	os.Exit(exitCode)
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
