package printer

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"olexsmir.xyz/clerk/journal/ast"
)

func (p *printer) writePostings(postings []*ast.Posting) {
	if len(postings) == 0 {
		return
	}

	maxAcct := measureTxAccts(postings)
	if p.cfg.AlignStyle == AlignTwoSpaces {
		p.writePostingsTwoSpaces(postings, maxAcct)
	} else {
		p.writePostingsTabbed(postings, maxAcct)
	}
}

func (p *printer) writePostingsTwoSpaces(postings []*ast.Posting, maxAcct int) {
	for _, pt := range postings {
		p.writePostingLine(pt, maxAcct)
		p.buf.WriteByte('\n')
		for _, c := range pt.Comments {
			p.buf.WriteString(p.indent)
			p.writeComment(&c)
			p.buf.WriteByte('\n')
		}
	}
}

func (p *printer) writePostingsTabbed(postings []*ast.Posting, maxAcct int) {
	var tmp strings.Builder
	tw := tabwriter.NewWriter(&tmp, 0, 0, 2, ' ', tabwriter.StripEscape)

	lp := &printer{cfg: p.cfg, indent: p.indent}
	for _, pt := range postings {
		lp.buf.Reset()
		lp.writePostingLine(pt, maxAcct)
		fmt.Fprintln(tw, lp.buf.String())
		for _, c := range pt.Comments {
			lp.buf.Reset()
			lp.buf.WriteString(lp.indent)
			lp.writeComment(&c)
			fmt.Fprintln(tw, lp.buf.String())
		}
	}
	tw.Flush()

	out := strings.TrimRight(tmp.String(), "\n")
	if out == "" {
		return
	}
	p.buf.WriteString(out)
	p.buf.WriteByte('\n')
}

func (p *printer) writePostingLine(pt *ast.Posting, maxAcct int) {
	p.buf.WriteString(p.indent)

	if pt.Status != nil && pt.Status.Value != ast.StatusNone {
		p.buf.WriteString(pt.Status.Value.String())
		p.buf.WriteByte(' ')
	}

	acct := pt.Account.Name
	switch pt.Type {
	case ast.PostingVirtualUnbalanced:
		acct = "(" + acct + ")"
	case ast.PostingVirtualBalanced:
		acct = "[" + acct + "]"
	}
	p.buf.WriteString(acct)

	pos := p.cfg.CommodityPos

	if pt.Amount != nil || pt.Balance != nil {
		switch p.cfg.AlignStyle {
		case AlignTwoSpaces:
			p.buf.WriteString("  ")
		case AlignRight:
			current := len(p.indent) + len(acct)
			if pt.Status != nil && pt.Status.Value != ast.StatusNone {
				current++
			}
			if pad := p.cfg.AlignColumn - current; pad > 0 {
				p.writeSpaces(pad)
			}
			p.buf.WriteByte('\v')
		case AlignTab:
			current := len(p.indent) + len(acct)
			if pt.Status != nil && pt.Status.Value != ast.StatusNone {
				current++
			}
			if pad := maxAcct + len(p.indent) - current; pad > 0 {
				p.writeSpaces(pad)
			}
			p.buf.WriteByte('\v')
		default:
			panic("impossible AlignStyle state")
		}
	}

	if pt.Amount != nil {
		p.writeAmount(pt.Amount, pos)
		if pt.Cost != nil {
			p.writeCost(pt.Cost, pos)
		}
	}

	if pt.Balance != nil {
		if pt.Amount != nil {
			p.buf.WriteByte(' ')
		}
		p.writeBalanceAssertion(pt.Balance, pos)
	}

	if pt.Comment != nil && pt.Comment.Text != "" {
		if p.cfg.AlignStyle == AlignTwoSpaces {
			p.buf.WriteString("  ")
		} else {
			p.buf.WriteByte('\v')
		}
		p.buf.WriteString("; ")
		p.buf.WriteString(pt.Comment.Text)
	}
}

func measureTxAccts(postings []*ast.Posting) int {
	maxAcct := 0
	for _, p := range postings {
		n := len(p.Account.Name)
		if p.Type == ast.PostingVirtualUnbalanced || p.Type == ast.PostingVirtualBalanced {
			n += 2
		}
		if n > maxAcct {
			maxAcct = n
		}
	}
	return maxAcct
}
