package printer

import (
	"olexsmir.xyz/clerk/internal/decimal"
	"olexsmir.xyz/clerk/journal/ast"
)

func (p *printer) writeAmount(a *ast.Amount, pos CommodityPos) {
	if a == nil {
		return
	}

	if a.IsExpr {
		p.buf.WriteString(a.Expr)
		return
	}

	prec := a.QuantityFmt.Precision
	if prec < 2 {
		prec = 2
	}

	comm := a.Commodity
	if comm == "" {
		p.writeDecimal(a.Quantity, a.QuantityFmt, prec)
		return
	}

	switch pos {
	case CommodityBefore:
		p.buf.WriteString(comm)
		if a.HasSpace {
			p.buf.WriteByte(' ')
		}
		p.writeDecimal(a.Quantity, a.QuantityFmt, prec)
	case CommodityAfter:
		p.writeDecimal(a.Quantity, a.QuantityFmt, prec)
		if a.HasSpace {
			p.buf.WriteByte(' ')
		}
		p.buf.WriteString(comm)
	default:
		panic("impossible CommodityPos value")
	}
}

func (p *printer) writeCost(c *ast.Cost, pos CommodityPos) {
	if c == nil {
		return
	}
	if c.IsTotal {
		p.buf.WriteString(" @@ ")
	} else {
		p.buf.WriteString(" @ ")
	}
	p.writeAmount(&c.Amount, pos)
}

func (p *printer) writeBalanceAssertion(ba *ast.BalanceAssertion, pos CommodityPos) {
	if ba == nil {
		return
	}
	switch {
	case ba.IsInclusive:
		p.buf.WriteString("=== ")
	case ba.IsStrict:
		p.buf.WriteString("== ")
	default:
		p.buf.WriteString("= ")
	}
	p.writeAmount(&ba.Amount, pos)
}

func (p *printer) writeDecimal(d decimal.Decimal, fmt ast.QuantityFormat, forcePrec int) {
	d.WriteFixed(&p.buf, forcePrec, fmt.Decimal, fmt.Thousands)
}
