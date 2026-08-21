package settings

import (
	"fmt"
	"math"
	"strings"

	"olexsmir.xyz/clerk/journal/printer"
)

func applyFormatTable(f *printer.Config, v any) ([]string, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid value %v (want table)", v)
	}
	return applyMap(m, func(k string, v any) ([]string, error) {
		return setFormatField(f, k, v)
	})
}

func setFormatField(f *printer.Config, name string, v any) ([]string, error) {
	switch normKey(name) {
	case "tab_indent":
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("invalid value %v (want bool)", v)
		}
		f.TabIndent = b
	case "indent_width":
		n, ok := toInt(v)
		if !ok {
			return nil, fmt.Errorf("invalid value %v (want int)", v)
		}
		if n < 1 || n > 16 {
			return nil, fmt.Errorf("indent-width %d out of range (want 1..16)", n)
		}
		f.IndentWidth = n
	case "preserve_blank_lines":
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("invalid value %v (want bool)", v)
		}
		f.PreserveBlankLines = b
	case "align_style":
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("invalid value %v (want string)", v)
		}
		switch {
		case strings.EqualFold(s, "two-spaces"):
			f.AlignStyle = printer.AlignTwoSpaces
		case strings.EqualFold(s, "right"):
			f.AlignStyle = printer.AlignRight
		case strings.EqualFold(s, "tab"):
			f.AlignStyle = printer.AlignTab
		default:
			return nil, fmt.Errorf("invalid align-style %q (want %q, %q, or %q)", s, "two-spaces", "right", "tab")
		}
	case "align_column":
		n, ok := toInt(v)
		if !ok {
			return nil, fmt.Errorf("invalid value %v (want int)", v)
		}
		if n < 1 || n > 240 {
			return nil, fmt.Errorf("align-column %d out of range (want 1..240)", n)
		}
		f.AlignColumn = n
	case "commodity_pos":
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("invalid value %v (want string)", v)
		}
		switch {
		case strings.EqualFold(s, "after"):
			f.CommodityPos = printer.CommodityAfter
		case strings.EqualFold(s, "before"):
			f.CommodityPos = printer.CommodityBefore
		default:
			return nil, fmt.Errorf("invalid commodity-pos %q (want %q or %q)", s, "after", "before")
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
