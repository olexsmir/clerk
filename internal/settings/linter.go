package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"olexsmir.xyz/clerk/internal/linter"
)

var (
	lintIndexOnce sync.Once
	lintIndex     map[string]linter.RuleID
)

// lintRuleID resolves a normalized rule key to its ID, building the index once
// and reusing it across calls (rule IDs are static). This avoids re-deriving
// the map and re-running normKey over every rule on each Apply.
func lintRuleID(name string) (linter.RuleID, bool) {
	lintIndexOnce.Do(func() {
		lintIndex = make(map[string]linter.RuleID, len(linter.Rules))
		for id := range linter.Rules {
			lintIndex[normKey(string(id))] = id
		}
	})
	id, ok := lintIndex[name]
	return id, ok
}

func (s *Settings) setLint(v any) ([]string, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid value %v (want table)", v)
	}
	rules := make(map[linter.RuleID]linter.RuleConfig, len(s.Linter.Rules))
	for id, rc := range s.Linter.Rules {
		rules[id] = rc
	}
	warns, err := applyMap(m, func(name string, rv any) ([]string, error) {
		id, ok := lintRuleID(normKey(name))
		if !ok {
			return []string{fmt.Sprintf("unknown lint rule %q", name)}, nil
		}
		rc := rules[id]
		if err := setRule(&rc, rv); err != nil {
			return nil, err
		}
		rules[id] = rc
		return nil, nil
	})
	if err != nil {
		return warns, err
	}
	s.Linter.Rules = rules
	return warns, nil
}

func setRule(rc *linter.RuleConfig, v any) error {
	switch v := v.(type) {
	case bool:
		if v {
			return errors.New(`true is not supported (want false, "off", a severity, or an options table)`)
		}
		rc.Disabled = true
	case string:
		if strings.EqualFold(v, "off") {
			rc.Disabled = true
			return nil
		}
		sev, ok := linter.ParseSeverity(v)
		if !ok {
			return fmt.Errorf("invalid severity %q (want %q, %q, %q, %q, or %q)",
				v, "off", "error", "warn", "info", "hint")
		}
		rc.Severity = sev
	case map[string]any:
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		rc.Options = raw
	default:
		return fmt.Errorf("invalid type %T (want bool, string, or table)", v)
	}
	return nil
}
