package printer

import (
	"strconv"
	"strings"

	"olexsmir.xyz/clerk/journal/ast"
)

func (p *printer) writeIncludeDirective(i *ast.IncludeDirective) {
	p.buf.WriteString("include ")
	p.buf.WriteString(quoteString(i.Path))
	p.writeInlineComment(i.Comment)
}

func (p *printer) writeAccountDirective(a *ast.AccountDirective) {
	p.buf.WriteString("account ")
	p.buf.WriteString(a.Account.String())
	p.writeInlineComment(a.Comment)
	for _, sd := range a.Subdirectives {
		p.buf.WriteByte('\n')
		p.buf.WriteString(p.indent)
		if sd.Name == "" {
			p.writeComment(sd.Comment)
			continue
		}
		p.buf.WriteString(sd.Name)
		p.buf.WriteByte(' ')
		p.buf.WriteString(sd.Value)
		p.writeInlineComment(sd.Comment)
	}
}

func (p *printer) writeCommodityDirective(c *ast.CommodityDirective) {
	p.buf.WriteString("commodity ")
	if c.FormatSub == nil {
		p.buf.WriteString(c.Commodity)
		p.writeInlineComment(c.Comment)
		return
	}
	if c.FormatSub.KeywordSpan.End.Offset == 0 {
		// inline form: "commodity $1,000.00"
		p.writeCommodityAmount(&c.FormatSub.Amount)
		p.writeInlineComment(c.Comment)
		return
	}
	p.buf.WriteString(c.Commodity)
	p.writeInlineComment(c.Comment)
	p.buf.WriteByte('\n')
	p.buf.WriteString(p.indent)
	p.buf.WriteString("format ")
	p.writeCommodityAmount(&c.FormatSub.Amount)
	p.writeInlineComment(c.FormatSub.Comment)
	p.writeBlockComments(c.BlockComments)
}

// writeCommodityAmount renders a commodity format amount, preserving the
// original commodity placement instead of the config's CommodityPos.
func (p *printer) writeCommodityAmount(a *ast.Amount) {
	prec := max(a.QuantityFmt.Precision, 2)
	if a.Commodity == "" {
		p.writeDecimal(a.Quantity, a.QuantityFmt, prec)
		return
	}
	switch a.CommodityPos {
	case ast.CommodityBefore:
		p.buf.WriteString(a.Commodity)
		if a.HasSpace {
			p.buf.WriteByte(' ')
		}
		p.writeDecimal(a.Quantity, a.QuantityFmt, prec)
	case ast.CommodityAfter:
		p.writeDecimal(a.Quantity, a.QuantityFmt, prec)
		if a.HasSpace {
			p.buf.WriteByte(' ')
		}
		p.buf.WriteString(a.Commodity)
	}
}

// writeBlockComments writes indented comment lines inside a directive block.
func (p *printer) writeBlockComments(cs []*ast.Comment) {
	for _, c := range cs {
		p.buf.WriteByte('\n')
		p.buf.WriteString(p.indent)
		p.writeComment(c)
	}
}

func (p *printer) writeAliasDirective(a *ast.AliasDirective) {
	p.buf.WriteString("alias ")
	p.buf.WriteString(a.From.String())
	p.buf.WriteString(" = ")
	p.buf.WriteString(a.To.String())
	p.writeInlineComment(a.Comment)
}

func (p *printer) writePayeeDirective(pd *ast.PayeeDirective) {
	p.buf.WriteString("payee ")
	p.buf.WriteString(quoteString(pd.Name.Name))
	p.writeInlineComment(pd.Comment)
}

func (p *printer) writeTagDirective(t *ast.TagDirective) {
	p.buf.WriteString("tag ")
	p.buf.WriteString(quoteString(t.Name))
	p.writeInlineComment(t.Comment)
}

func (p *printer) writeYearDirective(y *ast.YearDirective) {
	p.buf.WriteString("year ")
	p.writeInt(y.Year, 4)
	p.writeInlineComment(y.Comment)
}

func (p *printer) writeDecimalMarkDirective(d *ast.DecimalMarkDirective) {
	p.buf.WriteString("decimal-mark ")
	p.buf.WriteByte(d.Mark)
	p.writeInlineComment(d.Comment)
}

func (p *printer) writeDefaultCommodityDirective(d *ast.DefaultCommodityDirective) {
	p.buf.WriteString("D ")
	p.writeAmount(&d.Amount)
	p.writeInlineComment(d.Comment)
}

func (p *printer) writeConversionDirective(c *ast.ConversionDirective) {
	p.buf.WriteString("C ")
	p.writeAmount(&c.From)
	p.buf.WriteString(" = ")
	p.writeAmount(&c.To)
	p.writeInlineComment(c.Comment)
}

func (p *printer) writeMarketPriceDirective(m *ast.MarketPriceDirective) {
	p.buf.WriteString("P ")
	p.writeDate(m.DateTime.Date)
	p.writeTime(m.DateTime.Time)
	p.buf.WriteByte(' ')
	p.buf.WriteString(m.Commodity)
	p.buf.WriteByte(' ')
	p.writeAmount(&m.Amount)
	p.writeInlineComment(m.Comment)
}

func (p *printer) writeCommentBlockDirective(cb *ast.CommentBlockDirective) {
	p.buf.WriteString("comment")
	if cb.Header != "" {
		p.buf.WriteByte(' ')
		p.buf.WriteString(cb.Header)
	}
	p.buf.WriteByte('\n')
	if cb.Content != "" {
		p.buf.WriteString(cb.Content)
		if !strings.HasSuffix(cb.Content, "\n") {
			p.buf.WriteByte('\n')
		}
	}
	p.buf.WriteString("end\n")
}

func (p *printer) writeApplyDirective(a *ast.ApplyDirective) {
	p.buf.WriteString("apply ")
	p.buf.WriteString(a.Expr)
	p.writeInlineComment(a.Comment)
}

func (p *printer) writeEndDirective(e *ast.EndDirective) {
	p.buf.WriteString("end")
	if e.Expr != "" {
		p.buf.WriteByte(' ')
		p.buf.WriteString(e.Expr)
	}
	p.writeInlineComment(e.Comment)
}

func (p *printer) writeIgnoredDirective(e *ast.IgnoredDirective) {
	p.buf.WriteString("N ")
	p.buf.WriteString(e.Text)
	p.writeInlineComment(e.Comment)
}

func quoteString(s string) string {
	needsQuote := false
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '"' || c == ';' || c == '#' {
			needsQuote = true
			break
		}
	}
	if !needsQuote {
		return s
	}
	return strconv.Quote(s)
}
