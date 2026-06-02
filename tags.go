package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/tags"
)

func runTags(args []string) {
	fs := flag.NewFlagSet("tags", flag.ExitOnError)
	output := fs.String("o", "tags", "output file, set to - for stdout")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: clerk tags [-o tags] [<journals>...]\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting working directory: %v\n", err)
		os.Exit(1)
	}

	rawPaths := fs.Args()
	if len(rawPaths) == 0 {
		rawPaths = []string{"."}
	}

	var journals []string
	seen := make(map[string]bool)
	for _, p := range rawPaths {
		info, err := os.Stat(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", p, err)
			os.Exit(1)
		}

		if info.IsDir() {
			entries, err := os.ReadDir(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error reading directory %s: %v\n", p, err)
				os.Exit(1)
			}
			for _, entry := range entries {
				if entry.IsDir() { // TODO: keep traversing?
					continue
				}
				fpath := filepath.Join(p, entry.Name())
				if journal.IsJournalFile(fpath) && !seen[fpath] {
					seen[fpath] = true
					journals = append(journals, fpath)
				}
			}
		} else if journal.IsJournalFile(p) && !seen[p] {
			seen[p] = true
			journals = append(journals, p)
		}
	}

	if len(journals) == 0 {
		fmt.Fprintf(os.Stderr, "no journal files found\n")
		os.Exit(1)
	}

	loader := journal.NewLoader()
	for _, path := range journals {
		if _, err := loader.Load(path); err != nil {
			fmt.Fprintf(os.Stderr, "error loading %s: %v\n", path, err)
			os.Exit(1)
		}
	}

	tagger := tags.New(loader, cwd)

	w := os.Stdout
	if *output != "-" {
		f, err := os.Create(*output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating output file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}

	if err := tagger.Write(w); err != nil {
		fmt.Fprintf(os.Stderr, "error writing tags: %v\n", err)
		os.Exit(1)
	}
}
