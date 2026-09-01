package settings

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"olexsmir.xyz/clerk/internal/linter"
	"olexsmir.xyz/clerk/journal/printer"
)

type Settings struct {
	// SemanticHighlighting enables LSP semantic tokens.
	SemanticHighlighting bool

	// LatinToCyrillicCompletion matches Latin input against Cyrillic labels.
	LatinToCyrillicCompletion bool

	Linter linter.Config
	Format printer.Config
}

var DefaultConfig = Settings{
	SemanticHighlighting:      true,
	LatinToCyrillicCompletion: false,
	Linter:                    linter.DefaultConfig,
	Format:                    printer.DefaultConfig,
}

// Load reads and parses the TOML config file at path.
// A missing file yields defaults without error.
func Load(path string) (Settings, []string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultConfig, nil, nil
	}
	if err != nil {
		return DefaultConfig, nil, err
	}
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return Settings{}, nil, err
	}
	return parse(raw)
}

func parse(raw map[string]any) (settings Settings, warns []string, err error) {
	s := DefaultConfig
	warns, err = s.Apply(raw)
	return s, warns, err
}

// Apply merges raw setting from a config file into Settings object.
func (s *Settings) Apply(raw map[string]any) ([]string, error) {
	return applyMap(raw, s.applyFileField)
}

func (s *Settings) applyFileField(name string, val any) ([]string, error) {
	switch normalizeKey(name) {
	case "lint":
		return s.setLint(val)
	case "format":
		return s.setFormat(val)
	default:
		return []string{fmt.Sprintf("unknown setting %q", name)}, nil
	}
}

// ApplyLSP merges raw settings from lsp server config into Settings object.
func (s *Settings) ApplyLSP(raw map[string]any) ([]string, error) {
	return applyMap(raw, s.applyLSPField)
}

func (s *Settings) applyLSPField(name string, val any) ([]string, error) {
	switch normalizeKey(name) {
	case "semantic_highlighting":
		return nil, setBool(&s.SemanticHighlighting, val)
	case "latin_to_cyrillic_completion":
		return nil, setBool(&s.LatinToCyrillicCompletion, val)
	default:
		return s.applyFileField(name, val)
	}
}

func applyMap(m map[string]any, fn func(k string, val any) ([]string, error)) ([]string, error) {
	var (
		warns []string
		errs  []error
	)
	for k, val := range m {
		ws, err := fn(k, val)
		warns = append(warns, ws...)
		if err != nil {
			errs = append(errs, prefixLines(k, err))
		}
	}
	return warns, errors.Join(errs...)
}

func applyTable(v any, fn func(k string, val any) ([]string, error)) ([]string, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid value %v (want table)", v)
	}
	return applyMap(m, fn)
}

func normalizeKey(s string) string {
	// already canonical(lowercase, digits, underscores)
	canonical := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '_' && (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			canonical = false
			break
		}
	}
	if canonical {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	prevLower := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '-' || c == '_':
			b.WriteByte('_')
			prevLower = false
		case c >= 'A' && c <= 'Z':
			if prevLower {
				b.WriteByte('_')
			}
			b.WriteByte(c + 'a' - 'A')
			prevLower = false
		case c >= 'a' && c <= 'z':
			b.WriteByte(c)
			prevLower = true
		default:
			b.WriteByte(c)
			prevLower = false
		}
	}
	return b.String()
}

func prefixLines(prefix string, err error) error {
	lines := strings.Split(err.Error(), "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + ": " + line
		}
	}
	return errors.New(strings.Join(lines, "\n"))
}
