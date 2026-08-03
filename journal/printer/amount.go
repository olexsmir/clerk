package printer

import (
	"olexsmir.xyz/clerk/internal/decimal"
	"olexsmir.xyz/clerk/journal/ast"
)

func (p *printer) writeAmount(a *ast.Amount) {
	if a == nil {
		return
	}

	if a.IsExpr {
		if a.Expr != "" {
			// parenthesized expression, e.g. (amount * 0.5)
			p.buf.WriteByte('(')
			p.buf.WriteString(a.Expr)
			p.buf.WriteByte(')')
			return
		} else {
			// bare scaling expr, e.g. `*-1`, `*.33` — parser stored the value in Quantity
			p.buf.WriteByte('*')
		}
	}

	prec := max(a.QuantityFmt.Precision, 2)

	comm := a.Commodity
	if comm == "" {
		p.writeDecimal(a.Quantity, a.QuantityFmt, prec)
		return
	}

	switch p.cfg.CommodityPos {
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

func (p *printer) writeCost(c *ast.Cost) {
	if c.IsTotal {
		p.buf.WriteString(" @@ ")
	} else {
		p.buf.WriteString(" @ ")
	}
	p.writeAmount(&c.Amount)
}

func (p *printer) writeBalanceAssertion(ba *ast.BalanceAssertion) {
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
	p.writeAmount(&ba.Amount)
	if ba.Cost != nil {
		p.writeCost(ba.Cost)
	}
}

func (p *printer) writeDecimal(d decimal.Decimal, fmt ast.QuantityFormat, forcePrec int) {
	d.WriteFixed(&p.buf, forcePrec, fmt.Decimal, fmt.Thousands)
}
