package ast

import (
	"olexsmir.xyz/clerk/internal/decimal"
	"olexsmir.xyz/clerk/journal/token"
)

type BlankLine struct{ Span token.Span }

func (BlankLine) entryNode() {}

type Transaction struct {
	Date           Date
	SecondDate     *Date      // optional =2026-05-18 date
	Status         Status     // optional */! status
	Code           *Code      // optional (123) code
	Payee          *Payee     // optional payee
	Note           *Note      // part after |
	Comment        *Comment   // inline ; on header line
	HeaderComments []*Comment // indented ; lines before first posting
	Postings       []*Posting
	Span           token.Span
}

func (Transaction) entryNode() {}

type Period struct {
	Raw  string // "monthly", "every 2 weeks"
	From *Date
	To   *Date
	Span token.Span
}

func (Period) entryNode() {}

type PeriodicTransaction struct {
	Period         Period       // period-expr
	Status         Status       // optional */! status
	Code           *Code        // optional (123) code
	Description    *Description // optional description
	Comment        *Comment     // optional inline comment
	HeaderComments []*Comment
	Postings       []*Posting
	Span           token.Span
}

func (PeriodicTransaction) entryNode() {}

type AutomatedTransaction struct {
	Expr           Expr
	Postings       []*Posting
	Comment        *Comment   // inline ; on header line
	HeaderComments []*Comment // indented ; lines before first posting
	Span           token.Span
}

func (AutomatedTransaction) entryNode() {}

type PostingType int

const (
	PostingReal              PostingType = iota
	PostingVirtualBalanced               // '['
	PostingVirtualUnbalanced             // '('
)

func (p PostingType) String() string {
	switch p {
	case PostingReal:
		return "real"
	case PostingVirtualBalanced:
		return "balanced virtual"
	case PostingVirtualUnbalanced:
		return "unbalanced virtual"
	default:
		panic("unreachable")
	}
}

type Posting struct {
	Type     PostingType
	Status   Status
	Account  Account
	Amount   *Amount // nil == auto-balancing
	Cost     *Cost   // @ @@
	Balance  *BalanceAssertion
	Comment  *Comment
	Comments []Comment // continuation comment lines
	Span     token.Span
}

type Amount struct {
	IsNegative    bool
	Quantity      decimal.Decimal
	QuantityFmt   QuantityFormat
	Commodity     string
	CommodityPos  CommodityPos // Before | After
	CommoditySpan token.Span   // span of the commodity within the amount
	HasSpace      bool         // "$10" vs "$ 10"
	IsExpr        bool         // e.g: *-1
	Expr          string       // expression text e.g. "amount * -1". set only if IsExpr is true
	Span          token.Span
}

type Cost struct {
	IsTotal bool // @ vs @@
	Amount  Amount
	Span    token.Span
}

type BalanceAssertion struct {
	IsStrict    bool // ==  vs =
	IsInclusive bool // ===
	Amount      Amount
	Cost        *Cost // price after @/@@
	Span        token.Span
}

type CommodityPos int

func (c CommodityPos) String() string {
	if c == CommodityBefore {
		return "Before"
	}
	return "After"
}

const (
	CommodityBefore CommodityPos = iota
	CommodityAfter
)

type QuantityFormat struct {
	Decimal   byte // '.' or ','
	Thousands byte // ',' '.' ' ' or 0
	Precision int
}
