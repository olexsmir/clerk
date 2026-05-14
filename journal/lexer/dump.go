package lexer

import (
	"fmt"
	"strings"

	"github.com/olexsmir/ledger-tools/journal/token"
)

func (l *Lexer) Dump() string {
	var b strings.Builder
	for {
		t := l.Next()
		fmt.Fprintf(&b, "%-12s %-20q %d:%d-%d:%d\n",
			t.Type,
			t.Literal,
			t.Span.Start.Line,
			t.Span.Start.Col,
			t.Span.End.Line,
			t.Span.End.Col)
		if t.Type == token.EOF {
			break
		}
	}
	return b.String()
}
