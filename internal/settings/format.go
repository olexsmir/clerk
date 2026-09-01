package settings

import (
	"fmt"
	"strings"

	"olexsmir.xyz/clerk/journal/printer"
)

func (s *Settings) setFormat(v any) ([]string, error) {
	return applyTable(v, s.setFormatField)
}

func (s *Settings) setFormatField(name string, val any) ([]string, error) {
	switch normalizeKey(name) {
	case "tab_indent":
		return nil, setBool(&s.Format.TabIndent, val)
	case "indent_width":
		return nil, setInt(&s.Format.IndentWidth, val, 1, 32)
	case "preserve_blank_lines":
		return nil, setBool(&s.Format.PreserveBlankLines, val)
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
			return nil, fmt.Errorf("invalid value %q (want %q, %q, or %q)", as, "two-spaces", "right", "tab")
		}
	case "align_column":
		return nil, setInt(&s.Format.AlignColumn, val, 1, 240)
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
			return nil, fmt.Errorf("invalid value %q (want %q or %q)", c, "after", "before")
		}
	default:
		return []string{fmt.Sprintf("unknown format option %q", name)}, nil
	}
	return nil, nil
}

func setInt(n *int, v any, lo, hi int) error {
	var x int
	switch t := v.(type) {
	case int:
		x = t
	case int64:
		x = int(t)
	case float64:
		if t != float64(int(t)) {
			return fmt.Errorf("invalid value %v (want int)", v)
		}
		x = int(t)
	default:
		return fmt.Errorf("invalid value %v (want int)", v)
	}
	if x < lo || x > hi {
		return fmt.Errorf("%d out of range (want %d..%d)", x, lo, hi)
	}
	*n = x
	return nil
}

func setBool(b *bool, v any) error {
	x, ok := v.(bool)
	if !ok {
		return fmt.Errorf("invalid value %v (want bool)", v)
	}
	*b = x
	return nil
}
