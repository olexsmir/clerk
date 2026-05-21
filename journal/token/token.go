package token

import "fmt"

type Type int

type Token struct {
	Type    Type
	Literal string
	Span    Span
}

type Span struct{ Start, End Pos }

func (s Span) String() string {
	if s.Start.File != "" {
		return fmt.Sprintf("%s:%d:%d-%d:%d", s.Start.File,
			s.Start.Line, s.Start.Col,
			s.End.Line, s.End.Col)
	}
	return fmt.Sprintf("%d:%d-%d:%d",
		s.Start.Line, s.Start.Col,
		s.End.Line, s.End.Col)
}

type Pos struct {
	File   string // absolute path, "" for unknow
	Offset int
	Line   int
	Col    int
}

//go:generate go tool stringer -type=Type
const (
	ILLEGAL Type = iota
	EOF

	WHITESPACE // singe space or tab
	INDENT     // leading whitespace (posting/subdirective marker)
	NEWLINE    // \n

	INT     // 42
	DECIMAL // 420.69
	STRING  // quoted "string"
	TEXT    // free text, description, payee, account names

	BANG      // ! status
	STAR      // * status or comment marker
	PERCENT   // % comment
	HASH      // # comment
	SEMICOLON // ; comment or line break
	COLON     // :

	EQ     // =
	EQEQ   // ==
	EQEQEQ // ===
	AT     // @
	ATAT   // @@
	PIPE   // |
	PLUS   // +
	MINUS  // -
	TILDE  // ~

	LPAREN       // (
	RPAREN       // )
	LBRACE       // {
	LBRACELBRACE // {{
	RBRACERBRACE // }}
	RBRACE       // }
	LBRACKET     // [
	RBRACKET     // ]

	COMMODITYMARK // USD, $, £, "my fund"
	DATE          // 2024/01/01 / 2024-01-01
	TIME          // 12:00:00
	PARENEXPR     // (...) parenthesized expression in amount

	// directives
	COMMENTKW   // "comment"
	ACCOUNT     // "account"
	COMMODITY   // "commodity"
	INCLUDE     // "include"
	ALIAS       // "alias"
	PAYEE       // "payee"
	TAG         // "tag"
	APPLY       // "apply"
	END         // "end"
	YEAR        // "Y" or "year"
	DECIMALMARK // "decimal-mark"
	D           // "D" default commodity
	P           // "P" market price
	N           // "N" ignored price commodity
	C           // "C" commodity conversion
)
