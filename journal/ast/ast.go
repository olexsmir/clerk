package ast

import (
	"fmt"
	"strings"

	"olexsmir.xyz/clerk/journal/token"
)

type Journal struct {
	Entries []Entry
	Errors  []*ParseError
}

type Entry interface {
	entryNode()
}

type ParseError struct {
	Span    token.Span
	Message string
}

type FileError struct {
	Path    string
	Span    token.Span
	Message string
}

type Date struct {
	Year, Month, Day int
	Sep              byte // '-' '/' '.'
	Span             token.Span
}

// Compare returns -1 if d is before other, 0 if equal, 1 if after.
func (d Date) Compare(other Date) int {
	if d.Year != other.Year {
		if d.Year < other.Year {
			return -1
		}
		return 1
	}
	if d.Month != other.Month {
		if d.Month < other.Month {
			return -1
		}
		return 1
	}
	if d.Day != other.Day {
		if d.Day < other.Day {
			return -1
		}
		return 1
	}
	return 0
}

// String renders d as written: with its own separator, and without the year
// when the date carried none. Returns "" for an unset or invalid date.
func (d Date) String() string {
	if (d.Year == 0 && d.Month == 0 && d.Day == 0) ||
		(d.Month < 1 || d.Month > 12 || d.Day < 1 || d.Day > 31) {
		return ""
	}
	sep := d.Sep
	if sep == 0 {
		sep = '-'
	}
	if d.Year == 0 {
		return fmt.Sprintf("%d%c%d", d.Month, sep, d.Day)
	}
	return fmt.Sprintf("%04d%c%02d%c%02d", d.Year, sep, d.Month, sep, d.Day)
}

type Time struct {
	Hour, Minute, Second int
	Span                 token.Span
}

type DateTime struct {
	Date Date
	Time *Time
	Span token.Span
}

type Tag struct {
	Key, Value string
	Span       token.Span
}

type Comment struct {
	Marker byte // ';' '#' '%' '*'
	Tags   []Tag
	Text   string
	Span   token.Span
}

func (Comment) entryNode() {}

type StatusType int

func (s StatusType) String() string {
	switch s {
	case StatusCleared:
		return "*"
	case StatusPending:
		return "!"
	case StatusNone:
		return ""
	default:
		panic("unreachable")
	}
}

const (
	StatusNone    StatusType = iota // not set
	StatusCleared                   // * cleared
	StatusPending                   // ! pending
)

type Status struct {
	Value StatusType
	Span  token.Span
}

type Code struct {
	Value string
	Span  token.Span
}

type Note struct {
	Value string
	Span  token.Span
}

type Description struct {
	Value string
	Span  token.Span
}

type Expr struct {
	Value string
	Span  token.Span
}

type Payee struct {
	Name string
	Span token.Span
}

type SubAccount struct {
	Name string
	Span token.Span
}

type Account struct {
	Name []SubAccount // ['expenses' 'food']
	Span token.Span
}

func (a Account) String() string {
	if len(a.Name) == 0 {
		return ""
	}
	var name strings.Builder
	name.Grow(len(a.Name))
	name.WriteString(a.Name[0].Name)
	for _, s := range a.Name[1:] {
		name.WriteByte(':')
		name.WriteString(s.Name)
	}
	return name.String()
}
