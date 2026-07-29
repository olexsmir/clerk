package ast

import "olexsmir.xyz/clerk/journal/token"

type AccountDirective struct {
	Account Account
	Comment *Comment
	Span    token.Span
}

func (AccountDirective) entryNode() {}

type CommodityDirective struct {
	Commodity     string
	CommoditySpan token.Span // span of the commodity token
	Format        Amount     // optional format hint: "1,000.00 UAH"
	Comment       *Comment   // optional inline comment
	Span          token.Span
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
	Comment *Comment // optional inline comment
	Span    token.Span
}

func (TagDirective) entryNode() {}

type IncludeDirective struct {
	Path    string
	Comment *Comment // optional inline comment
	Span    token.Span
}

func (IncludeDirective) entryNode() {}

type AliasDirective struct {
	From, To Account
	Comment  *Comment // optional inline comment
	Span     token.Span
}

func (AliasDirective) entryNode() {}

type YearDirective struct {
	Year    int
	Comment *Comment // optional inline comment
	Span    token.Span
}

func (YearDirective) entryNode() {}

type DecimalMarkDirective struct {
	Mark    byte     // '.' ','
	Comment *Comment // optional inline comment
	Span    token.Span
}

func (DecimalMarkDirective) entryNode() {}

type DefaultCommodityDirective struct {
	Amount  Amount
	Comment *Comment // optional inline comment
	Span    token.Span
}

func (DefaultCommodityDirective) entryNode() {}

type MarketPriceDirective struct {
	DateTime  DateTime
	Commodity string
	Amount    Amount
	Comment   *Comment // optional inline comment
	Span      token.Span
}

func (MarketPriceDirective) entryNode() {}

type ConversionDirective struct {
	From    Amount
	To      Amount
	Comment *Comment // optional inline comment
	Span    token.Span
}

func (ConversionDirective) entryNode() {}

type ApplyDirective struct {
	Expr    string // text after apply e.g "tag foo"
	Comment *Comment
	Span    token.Span
}

func (ApplyDirective) entryNode() {}

type EndDirective struct {
	Expr    string   // text after end e.g "tag"
	Comment *Comment // optional inline comment
	Span    token.Span
}

func (EndDirective) entryNode() {}

type CommentBlockDirective struct {
	Header  string // text after "comment" on the same line
	Content string
	Comment *Comment // optional inline comment
	Span    token.Span
}

func (CommentBlockDirective) entryNode() {}

type IgnoredDirective struct {
	Text    string   // content after N (commodity symbol or text)
	Comment *Comment // optional inline comment
	Span    token.Span
}

func (IgnoredDirective) entryNode() {}
