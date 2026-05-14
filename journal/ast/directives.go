package ast

import "olexsmir.xyz/clerk/journal/token"

type AccountDirective struct {
	Account Account
	Comment *Comment
	Span    token.Span
}

func (AccountDirective) entryNode() {}

type CommodityDirective struct {
	Commodity string
	Format    *Amount // optional format hint: "1,000.00 UAH"
	Comment   *Comment
	Span      token.Span
}

func (CommodityDirective) entryNode() {}

type PayeeDirective struct {
	Name    string
	Comment *Comment
	Span    token.Span
}

func (PayeeDirective) entryNode() {}

type TagDirective struct {
	Name    string
	Comment *Comment
	Span    token.Span
}

func (TagDirective) entryNode() {}

type IncludeDirective struct {
	Path    string
	Comment *Comment
	Span    token.Span
}

func (IncludeDirective) entryNode() {}

type AliasDirective struct {
	From, To string
	Span     token.Span
}

func (AliasDirective) entryNode() {}

type YearDirective struct {
	Year int
	Span token.Span
}

func (YearDirective) entryNode() {}

type DecimalMarkDirective struct {
	Mark byte // '.' ','
	Span token.Span
}

func (DecimalMarkDirective) entryNode() {}

type DefaultCommodityDirective struct {
	Amount Amount
	Span   token.Span
}

func (DefaultCommodityDirective) entryNode() {}

type MarketPriceDirective struct {
	DateTime  DateTime
	Commodity string
	Amount    Amount
	Span      token.Span
}

func (MarketPriceDirective) entryNode() {}

type ApplyDirective struct {
	Expr    string // text after apply e.g "tag foo"
	Comment *Comment
	Span    token.Span
}

func (ApplyDirective) entryNode() {}

type EndDirective struct {
	Expr    string // text after end e.g "tag"
	Comment *Comment
	Span    token.Span
}

func (EndDirective) entryNode() {}

type CommentBlockDirective struct {
	Header  string // text after "comment" on the same line
	Content string
	Comment *Comment
	Span    token.Span
}

func (CommentBlockDirective) entryNode() {}

type IgnoredDirective struct {
	Span token.Span
}

func (IgnoredDirective) entryNode() {}
