package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"olexsmir.xyz/clerk/internal/settings"
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
			_ = filepath.Walk(p, func(fpath string, finfo os.FileInfo, err error) error {
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

func readStdin() ([]byte, error) {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	return src, nil
}

func findConfigFilePath(cmd *cli.Command) (string, error) {
	if p := cmd.String("config"); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("config %q: %w", p, err)
		}
		return p, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, "clerk.toml"), nil
}

func loadConfig(cmd *cli.Command) (settings.Settings, []string, error) {
	configPath, err := findConfigFilePath(cmd)
	if err != nil {
		return settings.Settings{}, nil, err
	}
	return settings.Load(configPath)
}
