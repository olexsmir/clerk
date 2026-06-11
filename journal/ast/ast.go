package ast

import (
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

type Time struct {
	Hour, Minute, Second int
	Span                 token.Span
}

type DateTime struct {
	Date Date
	Time *Time
	Span token.Span
}

type Comment struct {
	Marker byte // ';' '#' '%' '*'
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
	StatusCleared StatusType = iota // * cleared
	StatusPending                   // ! pending
	StatusNone                      // not set
)

type Status struct {
	Value StatusType
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
