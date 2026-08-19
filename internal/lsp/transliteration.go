package lsp

import "strings"

// latinToCyrillicTable maps latin input to Ukrainian Cyrillic.
// Ordered longest-first so greedy scanning matches "shch" as щ before it can match "sh"+"ch".
var latinToCyrillicTable = [...]struct{ latin, cyrillic string }{
	{"shch", "щ"},
	{"sch", "щ"},
	{"dzh", "дж"},
	{"zh", "ж"},
	{"kh", "х"},
	{"ts", "ц"},
	{"ch", "ч"},
	{"sh", "ш"},
	{"dz", "дз"},
	{"yu", "ю"},
	{"ya", "я"},
	{"ye", "є"},
	{"yi", "ї"},
	{"ji", "ї"},
	{"a", "а"},
	{"b", "б"},
	{"v", "в"},
	{"g", "г"},
	{"d", "д"},
	{"e", "е"},
	{"z", "з"},
	{"i", "і"},
	{"j", "й"},
	{"k", "к"},
	{"l", "л"},
	{"m", "м"},
	{"n", "н"},
	{"o", "о"},
	{"p", "п"},
	{"r", "р"},
	{"s", "с"},
	{"t", "т"},
	{"u", "у"},
	{"f", "ф"},
	{"h", "х"},
	{"c", "ц"},
	{"q", "к"},
	{"x", "кс"},
	{"w", "в"},
	{"y", "и"},
}

func latinToCyrillic(pattern string) string {
	p := strings.ToLower(pattern)
	var b strings.Builder
	b.Grow(len(p))
	for i := 0; i < len(p); {
		for _, kv := range latinToCyrillicTable {
			if strings.HasPrefix(p[i:], kv.latin) {
				b.WriteString(kv.cyrillic)
				i += len(kv.latin)
				goto next
			}
		}
		b.WriteByte(p[i])
		i++
	next:
	}
	return b.String()
}
