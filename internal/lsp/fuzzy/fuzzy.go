package fuzzy

import (
	"strings"
	"unicode"
)

// Matcher scores a precompiled pattern against candidate texts; [Compile]
// hoists the lowercasing and rune conversion out of a per-candidate loop.
type Matcher struct {
	p []rune // lowercased pattern runes; empty matches everything
}

// Compile builds a matcher for pattern. The empty pattern matches every
// text with score 1.
func Compile(pattern string) Matcher {
	if pattern == "" {
		return Matcher{}
	}
	return Matcher{p: []rune(strings.ToLower(pattern))}
}

// Score scores how well the pattern matches text as a subsequence.
// It returns 0 if pattern is not a case-insensitive subsequence of text,
// otherwise a score in (0, 1], where 1 is a perfect contiguous match at a
// segment boundary. An empty pattern matches everything with score 1.
//
// Matching is greedy and leftmost; each matched rune scores base 1, plus 3 if
// it starts a segment (text start or after a separator), plus 2 if it is
// contiguous with the previous matched rune, plus 1 if the case matches
// exactly. The total is normalized by 4L+1, the maximum score of a perfect
// match of length L, so exact segment matches score 1 regardless of length
// ("food" and "expenses" both match "expenses:food" at 1.0).
func (m Matcher) Score(text string) float64 {
	p := m.p
	if len(p) == 0 {
		return 1
	}
	t := []rune(text)

	total := 0
	prev := -1
	for i, pr := range p {
		j := prev + 1
		for ; j < len(t); j++ {
			if unicode.ToLower(t[j]) == pr {
				break
			}
		}
		if j == len(t) {
			return 0
		}
		w := 1
		if j == 0 || isFuzzySep(t[j-1]) {
			w += 3
		}
		if i > 0 && j == prev+1 {
			w += 2
		}
		if t[j] == p[i] {
			w++
		}
		total += w
		prev = j
	}
	score := float64(total) / float64(4*len(p)+1)
	if score > 1 {
		return 1
	}
	return score
}

// Score scores pattern against text; see [Matcher.Score].
func Score(pattern, text string) float64 { return Compile(pattern).Score(text) }

// isFuzzySep reports whether r is a segment boundary for fuzzy matching:
// account-name separators and word boundaries.
func isFuzzySep(r rune) bool {
	switch r {
	case ':', '.', '-', '_', '/', ' ', '\t':
		return true
	}
	return false
}
