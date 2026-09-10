package printer

import (
	"olexsmir.xyz/clerk/journal/ast"
)

func (p *printer) writeTransaction(t *ast.Transaction) {
	// date
	p.writeDate(t.Date)

	// second date
	if t.SecondDate != nil {
		p.buf.WriteByte('=')
		p.writeDate(*t.SecondDate)
	}

	// status
	if t.Status != ast.StatusNone {
		p.buf.WriteByte(' ')
		p.buf.WriteString(t.Status.String())
	}

	// code
	if t.Code != "" {
		p.buf.WriteString(" (")
		p.buf.WriteString(t.Code)
		p.buf.WriteByte(')')
	}

	// payee
	if t.Payee != "" {
		p.buf.WriteByte(' ')
		p.buf.WriteString(t.Payee)
	}

	// note
	if t.Note != "" {
		p.buf.WriteString(" | ")
		p.buf.WriteString(t.Note)
	}

	p.writeComment(t.Comment)
	p.buf.WriteByte('\n')

	// header comments (between transaction line and first posting)
	for _, c := range t.HeaderComments {
		p.buf.WriteString(p.indent)
		p.writeComment(c)
		p.buf.WriteByte('\n')
	}

	// postings
	p.writePostings(t.Postings)
}

func (p *printer) writePeriodicTransaction(t *ast.PeriodicTransaction) {
	p.buf.WriteByte('~')

	// period
	if t.Period.Raw != "" {
		p.buf.WriteByte(' ')
		p.buf.WriteString(t.Period.Raw)
	}

	// status
	if t.Status != ast.StatusNone {
		p.buf.WriteByte(' ')
		p.buf.WriteString(t.Status.String())
	}

	// code
	if t.Code != "" {
		p.buf.WriteString(" (")
		p.buf.WriteString(t.Code)
		p.buf.WriteByte(')')
	}

	// description
	if t.Description != "" {
		p.buf.WriteByte(' ')
		p.buf.WriteString(t.Description)
	}

	// comment
	p.writeInlineComment(t.Comment)
	p.buf.WriteByte('\n')

	// header comments (between periodic line and first posting)
	for _, c := range t.HeaderComments {
		p.buf.WriteString(p.indent)
		p.writeComment(c)
		p.buf.WriteByte('\n')
	}

	// postings
	p.writePostings(t.Postings)
}

func (p *printer) writeAutomatedTransaction(t *ast.AutomatedTransaction) {
	p.buf.WriteByte('=')

	// expression
	if t.Expr != "" {
		p.buf.WriteByte(' ')
		p.buf.WriteString(t.Expr)
	}

	// comment
	if t.Comment != nil && t.Comment.Text != "" {
		p.buf.WriteString("  ; ")
		p.buf.WriteString(t.Comment.Text)
	}

	p.buf.WriteByte('\n')

	// header comments (between auto line and first posting)
	for _, c := range t.HeaderComments {
		p.buf.WriteString(p.indent)
		p.writeComment(c)
		p.buf.WriteByte('\n')
	}

	// postings
	p.writePostings(t.Postings)
}

// writeDate writes date duh.
func (p *printer) writeDate(d ast.Date) {
	if d.Year != 0 {
		p.writeInt(d.Year, 4)
		p.buf.WriteByte('-')
	}
	p.writeInt(int(d.Month), 2)
	p.buf.WriteByte('-')
	p.writeInt(int(d.Day), 2)
}

func (p *printer) writeTime(t *ast.Time) {
	if t != nil {
		p.buf.WriteByte(' ')
		p.writeInt(t.Hour, 2)
		p.buf.WriteByte(':')
		p.writeInt(t.Minute, 2)
		p.buf.WriteByte(':')
		p.writeInt(t.Second, 2)
	}
}

// writeInt writes an integer left-padded to the given number of digits.
func (p *printer) writeInt(n int, digits int) {
	var b [10]byte
	pos := len(b)
	for range digits {
		pos--
		b[pos] = '0' + byte(n%10)
		n /= 10
	}
	p.buf.Write(b[pos:])
}
