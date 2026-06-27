package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"olexsmir.xyz/clerk/journal"
)

func resolvePaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	seen := make(map[string]bool)

	var files []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}

		if info.IsDir() {
			filepath.Walk(p, func(fpath string, finfo os.FileInfo, err error) error {
				if err != nil || finfo.IsDir() || !journal.IsJournalFile(fpath) {
					return nil
				}
				if !seen[fpath] {
					seen[fpath] = true
					files = append(files, fpath)
				}
				return nil
			})
			continue
		}

		if journal.IsJournalFile(p) && !seen[p] {
			seen[p] = true
			files = append(files, p)
		}
	}

	return files, nil
}

func loadFile(loader *journal.Loader, path string) (*journal.ParsedFile, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	pf, err := loader.LoadBytes(path, src)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return pf, nil
}

func readStdin() ([]byte, error) {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	return src, nil
}

func loadStdin(loader *journal.Loader) (*journal.ParsedFile, error) {
	src, err := readStdin()
	if err != nil {
		return nil, err
	}
	pf, err := loader.LoadBytes("stdin", src)
	if err != nil {
		return nil, fmt.Errorf("parsing stdin: %w", err)
	}
	return pf, nil
}
