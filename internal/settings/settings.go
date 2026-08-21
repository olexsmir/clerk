package settings

import (
	"errors"
	"fmt"
	"strings"

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

func Default() Settings {
	return Settings{
		SemanticHighlighting:      true,
		LatinToCyrillicCompletion: false,
		Format:                    *printer.DefaultConfig,
	}
}

// Parse decodes raw settings from a config file over the defaults.
func Parse(raw map[string]any) (Settings, []string, error) {
	s := Default()
	warns, err := s.Apply(raw)
	return s, warns, err
}

// Apply merges raw settings from a config file into s. The file schema knows
// only about format and lint; everything else (including LSP-only options) is
// reported as an unknown setting.
func (s *Settings) Apply(raw map[string]any) ([]string, error) {
	return applyMap(raw, s.applyFileField)
}

// ApplyLSP merges raw settings from the LSP server into s, including the
// LSP-only options the config file is not allowed to set.
func (s *Settings) ApplyLSP(raw map[string]any) ([]string, error) {
	return applyMap(raw, s.applyLSPField)
}

func (s *Settings) applyFileField(name string, val any) ([]string, error) {
	switch normKey(name) {
	case "format":
		return applyFormatTable(&s.Format, val)
	case "lint":
		return s.setLint(val)
	default:
		return []string{fmt.Sprintf("unknown setting %q", name)}, nil
	}
}

func (s *Settings) applyLSPField(name string, val any) ([]string, error) {
	switch normKey(name) {
	case "semantic_highlighting":
		b, ok := val.(bool)
		if !ok {
			return nil, fmt.Errorf("invalid value %v (want bool)", val)
		}
		s.SemanticHighlighting = b
	case "latin_to_cyrillic_completion":
		b, ok := val.(bool)
		if !ok {
			return nil, fmt.Errorf("invalid value %v (want bool)", val)
		}
		s.LatinToCyrillicCompletion = b
	default:
		return s.applyFileField(name, val)
	}
	return nil, nil
}

// applyMap iterates a settings table, collecting warnings and joining per-key
// errors so every handler shares one walk.
func applyMap(m map[string]any, fn func(k string, v any) ([]string, error)) ([]string, error) {
	var (
		warns []string
		errs  []error
	)
	for k, v := range m {
		ws, err := fn(k, v)
		warns = append(warns, ws...)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", k, err))
		}
	}
	return warns, errors.Join(errs...)
}

// normKey canonicalizes a setting name to lowercase snake_case, accepting
// snake_case, kebab-case, and camelCase input (e.g. from TOML or JSON).
func normKey(s string) string {
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
