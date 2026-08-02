package journal

import (
	"path/filepath"
	"strings"
)

var (
	extensionSet        map[string]bool
	SupportedExtensions = [...]string{
		".journal", ".jrnl", ".j", ".hledger",
		".dat", ".ledger",
	}
)

func init() {
	extensionSet = make(map[string]bool, len(SupportedExtensions))
	for _, ext := range SupportedExtensions {
		extensionSet[ext] = true
	}
}

func IsJournalFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return extensionSet[ext]
}
