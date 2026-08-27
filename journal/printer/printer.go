package printer

import (
	"fmt"
	"io"
	"strings"

	"olexsmir.xyz/clerk/journal/ast"
)

// AlignStyle controls how postings are aligned
type AlignStyle int

const (
	AlignTwoSpaces AlignStyle = iota // "  Account  $10.00"
	AlignRight                       // amounts at fixed col: "  Account         $10.00"
	AlignTab                         // elastic tabstops
)

// CommodityPos controls where the commodity marker is placed
type CommodityPos int

const (
	CommodityAfter  CommodityPos = iota // "10.00 EUR"
	CommodityBefore                     // "$10.00"
)

type Config struct {
	TabIndent          bool         // true = tabs, false = spaces
	IndentWidth        int          // spaces per indent level (default: 2)
	PreserveBlankLines bool         // preserve consecutive blank lines as-is
	AlignStyle         AlignStyle   // (default AlignTwoSpaces)
	AlignColumn        int          // fixed column for AlignRight
	CommodityPos       CommodityPos // where to place commodity
}

var DefaultConfig = Config{
	TabIndent:          false,
	IndentWidth:        2,
	PreserveBlankLines: false,
	AlignStyle:         AlignTwoSpaces,
	AlignColumn:        70,
	CommodityPos:       CommodityAfter,
}

func (c *Config) indent() string {
	if c.TabIndent {
		return "\t"
	}
	n := 2
	if c.IndentWidth > 0 {
		n = c.IndentWidth
	}
	return spaces(n)
}

// printer holds formatting state for a single Fprint call.
type printer struct {
	buf          strings.Builder
	cfg          *Config
	indent       string
	prevWasBlank bool
}

// Fprint formats using the default config.
func Fprint(w io.Writer, j *ast.Journal) error { return DefaultConfig.Fprint(w, j) }

// Fprint formats a parsed journal.
func (c *Config) Fprint(w io.Writer, j *ast.Journal) error {
	p := printer{cfg: c, indent: c.indent()}

	for _, e := range j.Entries {
		p.formatEntry(e)
	}

	// allow exactly one trailing newline
	out := strings.TrimRight(p.buf.String(), "\n") + "\n"
	_, err := io.WriteString(w, out)
	return err
}

// FprintEntry formats a single ast entry using the default config.
func FprintEntry(w io.Writer, e ast.Entry) error { return DefaultConfig.FprintEntry(w, e) }

// FprintEntry formats a single journal entry.
func (c *Config) FprintEntry(w io.Writer, e ast.Entry) error {
	p := printer{cfg: c, indent: c.indent()}
	p.formatEntry(e)
	_, err := io.WriteString(w, p.buf.String())
	return err
}

func (p *printer) formatEntry(e ast.Entry) {
	switch e := e.(type) {
	case *ast.BlankLine:
		if !p.prevWasBlank || p.cfg.PreserveBlankLines {
			p.buf.WriteByte('\n')
		}
		p.prevWasBlank = true
		return
	case *ast.Transaction:
		p.prevWasBlank = false
		p.writeTransaction(e)
		return
	case *ast.PeriodicTransaction:
		p.prevWasBlank = false
		p.writePeriodicTransaction(e)
		return
	case *ast.AutomatedTransaction:
		p.prevWasBlank = false
		p.writeAutomatedTransaction(e)
		return

	case *ast.IgnoredDirective:
		p.writeIgnoredDirective(e)
		return

	case *ast.Comment:
		p.writeComment(e)
	case *ast.AccountDirective:
		p.writeAccountDirective(e)
	case *ast.CommodityDirective:
		p.writeCommodityDirective(e)
	case *ast.IncludeDirective:
		p.writeIncludeDirective(e)
	case *ast.AliasDirective:
		p.writeAliasDirective(e)
	case *ast.PayeeDirective:
		p.writePayeeDirective(e)
	case *ast.TagDirective:
		p.writeTagDirective(e)
	case *ast.YearDirective:
		p.writeYearDirective(e)
	case *ast.DecimalMarkDirective:
		p.writeDecimalMarkDirective(e)
	case *ast.MarketPriceDirective:
		p.writeMarketPriceDirective(e)
	case *ast.ConversionDirective:
		p.writeConversionDirective(e)
	case *ast.DefaultCommodityDirective:
		p.writeDefaultCommodityDirective(e)
	case *ast.ApplyDirective:
		p.writeApplyDirective(e)
	case *ast.EndDirective:
		p.writeEndDirective(e)
	case *ast.CommentBlockDirective:
		p.writeCommentBlockDirective(e)
	default:
		fmt.Fprintf(&p.buf, "; unknown entry %T", e)
	}
	p.prevWasBlank = false
	p.buf.WriteByte('\n')
}

func (p *printer) writeSpaces(n int) {
	for range n {
		p.buf.WriteByte(' ')
	}
}

func isCommentChar(b byte) bool {
	return b == ';' || b == '#' || b == '%' || b == '*'
}

func (p *printer) writeComment(c *ast.Comment) {
	if c != nil && c.Text != "" {
		p.buf.WriteByte(c.Marker)
		if !isCommentChar(c.Text[0]) {
			p.buf.WriteByte(' ')
		}
		p.buf.WriteString(c.Text)
	}
}

func (p *printer) writeInlineComment(c *ast.Comment) {
	if c != nil && c.Text != "" {
		p.buf.WriteByte(' ')
		p.buf.WriteByte(' ')
		p.buf.WriteByte(c.Marker)
		if !isCommentChar(c.Text[0]) {
			p.buf.WriteByte(' ')
		}
		p.buf.WriteString(c.Text)
	}
}

func spaces(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
