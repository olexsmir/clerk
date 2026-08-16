package ast

import "olexsmir.xyz/clerk/journal/token"

type AccountDirective struct {
	Account       Account
	Subdirectives []AccountSubdirective // indented block lines, in order
	Comment       *Comment
	Span          token.Span
}

func (AccountDirective) entryNode() {}

type AccountSubdirectiveKind uint8

const (
	SubdirectiveComment AccountSubdirectiveKind = iota
	SubdirectiveAlias
	SubdirectiveType
	SubdirectiveNote
)

func (k AccountSubdirectiveKind) String() string {
	switch k {
	case SubdirectiveAlias:
		return "alias"
	case SubdirectiveType:
		return "type"
	case SubdirectiveNote:
		return "note"
	default:
		panic("unreachable")
	}
}

type AccountSubdirective struct {
	Kind      AccountSubdirectiveKind // keyword; SubdirectiveComment for comment lines
	NameSpan  token.Span              // keyword span; for comment lines, the marker span
	Value     string                  // text after the keyword; "" for comment lines
	ValueSpan token.Span              // value span; zero for comment lines and empty values
	Comment   *Comment                // inline comment after the value; for comment lines, the line itself
}

type CommodityDirective struct {
	Commodity     string
	CommoditySpan token.Span          // span of the commodity token
	FormatSub     *FormatSubDirective // display format; nil when not given
	BlockComments []*Comment          // indented comment-only lines inside the block
	Comment       *Comment            // optional inline comment
	Span          token.Span
}

func (CommodityDirective) entryNode() {}

// FormatSubDirective is a commodity display format, either inline on the
// directive line ("commodity $1,000.00") or as a "format" subdirective.
type FormatSubDirective struct {
	Amount      Amount
	KeywordSpan token.Span // span of the "format" keyword; zero when inline
	Comment     *Comment   // inline comment; subdirective form only
}

type PayeeDirective struct {
	Name    *Payee
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
