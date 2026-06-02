package journal

import (
	"path/filepath"
	"strings"
)

func IsJournalFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return extensionSet[ext]
}

var (
	extensionSet        map[string]bool
	SupportedExtensions = [...]string{
		".journal", ".hledger",
		".dat", ".ledger",
		".jrnl",
	}
)

func init() {
	extensionSet = make(map[string]bool, len(SupportedExtensions))
	for _, ext := range SupportedExtensions {
		extensionSet[ext] = true
	}
}
