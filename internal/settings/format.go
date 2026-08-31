package settings

import (
	"fmt"
	"math"
	"strings"

	"olexsmir.xyz/clerk/journal/printer"
)

func (s *Settings) setFormat(v any) ([]string, error) {
	return applyTable(v, s.setFormatField)
}

func (s *Settings) setFormatField(name string, val any) ([]string, error) {
	switch normalizeKey(name) {
	case "tab_indent":
		b, ok := val.(bool)
		if !ok {
			return nil, fmt.Errorf("invalid value %v (want bool)", val)
		}
		s.Format.TabIndent = b
	case "indent_width":
		n, ok := toInt(val)
		if !ok {
			return nil, fmt.Errorf("invalid value %v (want int)", val)
		}
		if n < 1 || n > 16 {
			return nil, fmt.Errorf("indent-width %d out of range (want 1..16)", n)
		}
		s.Format.IndentWidth = n
	case "preserve_blank_lines":
		b, ok := val.(bool)
		if !ok {
			return nil, fmt.Errorf("invalid value %v (want bool)", val)
		}
		s.Format.PreserveBlankLines = b
	case "align_style":
		as, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("invalid value %v (want string)", val)
		}
		switch {
		case strings.EqualFold(as, "two-spaces"):
			s.Format.AlignStyle = printer.AlignTwoSpaces
		case strings.EqualFold(as, "right"):
			s.Format.AlignStyle = printer.AlignRight
		case strings.EqualFold(as, "tab"):
			s.Format.AlignStyle = printer.AlignTab
		default:
			return nil, fmt.Errorf("invalid align-style %q (want %q, %q, or %q)",
				as, "two-spaces", "right", "tab")
		}
	case "align_column":
		n, ok := toInt(val)
		if !ok {
			return nil, fmt.Errorf("invalid value %v (want int)", val)
		}
		if n < 1 || n > 240 {
			return nil, fmt.Errorf("align-column %d out of range (want 1..240)", n)
		}
		s.Format.AlignColumn = n
	case "commodity_pos":
		c, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("invalid value %v (want string)", val)
		}
		switch {
		case strings.EqualFold(c, "after"):
			s.Format.CommodityPos = printer.CommodityAfter
		case strings.EqualFold(c, "before"):
			s.Format.CommodityPos = printer.CommodityBefore
		default:
			return nil, fmt.Errorf("invalid commodity-pos %q (want %q or %q)", c, "after", "before")
		}
	default:
		return []string{fmt.Sprintf("unknown format option %q", name)}, nil
	}
	return nil, nil
}

// toInt converts an integer-like value to int. Integral floats (e.g. JSON 2.0)
// are accepted; non-integral floats (e.g. TOML 2.5) are rejected so callers
// surface a clear error instead of silently truncating.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n != math.Trunc(n) {
			return 0, false
		}
		return int(n), true
	default:
		return 0, false
	}
}
