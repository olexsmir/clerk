package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"olexsmir.xyz/clerk/internal/linter"
)

var ruleIndex = func() map[string]linter.RuleID {
	idx := make(map[string]linter.RuleID, len(linter.Rules))
	for id := range linter.Rules {
		idx[normalizeKey(string(id))] = id
	}
	return idx
}()

func (s *Settings) setLint(v any) ([]string, error) {
	return applyTable(v, func(name string, val any) ([]string, error) {
		id, ok := ruleIndex[normalizeKey(name)]
		if !ok {
			return []string{fmt.Sprintf("unknown lint rule %q", name)}, nil
		}
		rc, err := s.applyLintRule(s.Linter.Rules[id], val)
		if err != nil {
			return nil, err
		}
		s.Linter.Rules[id] = rc
		return nil, nil
	})
}

// applyLintRule returns a copy of rc with the raw rule value applied. It returns a
// new value (never mutates in place) so callers can store it back into a map.
func (s *Settings) applyLintRule(rc linter.RuleConfig, v any) (linter.RuleConfig, error) {
	switch v := v.(type) {
	case bool:
		if v {
			return rc, errors.New(`true is not supported (want false, "off", a severity, or an options table)`)
		}
		rc.Disabled = true
	case string:
		return applySeverity(rc, v)
	case map[string]any:
		opts := make(map[string]any, len(v))
		for k, val := range v {
			if normalizeKey(k) == "severity" {
				s, ok := val.(string)
				if !ok {
					return rc, fmt.Errorf("invalid severity %v (want string)", val)
				}
				var err error
				rc, err = applySeverity(rc, s)
				if err != nil {
					return rc, err
				}
				continue
			}
			opts[k] = val
		}
		if len(opts) > 0 {
			raw, err := json.Marshal(opts)
			if err != nil {
				return rc, err
			}
			rc.Options = raw
		}
	default:
		return rc, fmt.Errorf("invalid type %T (want bool, string, or table)", v)
	}
	return rc, nil
}

// applySeverity returns a copy of rc with a rule severity (or "off") applied.
func applySeverity(rc linter.RuleConfig, s string) (linter.RuleConfig, error) {
	if strings.EqualFold(s, "off") {
		rc.Disabled = true
		return rc, nil
	}
	sev, ok := linter.ParseSeverity(s)
	if !ok {
		return rc, fmt.Errorf("invalid severity %q (want %q, %q, %q, %q, or %q)",
			s, "off", "error", "warn", "info", "hint")
	}
	rc.Severity = sev
	return rc, nil
}
