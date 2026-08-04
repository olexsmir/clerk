package lexer

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"olexsmir.xyz/clerk/journal/token"
)

type Mode uint

const (
	// start of a line, nothing consumed
	ModeDefault Mode = iota

	// after ; # * % ; at start of line, or anywhere inline
	// everything until \n is comment text
	ModeComment

	// after lexing a date at column 0
	// expects: optional status, optional code, description, comment
	ModeTransaction

	// after lexing an indent at start of line
	// expects: account name, then two spaces, then amount
	ModePosting

	// after ~, period expression
	// expects: period, optional description (after 2+ spaces), optional comment
	ModePeriodic

	// after =, automates transaction
	// expects: expression
	ModeAutomated

	// after a directive keyword like account, commodity, include
	// expects: rest of directive content
	ModeDirective
)

type Lexer struct {
	file  string
	input []byte
	mode  Mode

	ch     rune // current rune (0 = EOF/sentinel)
	chSize int  // byte size of current rune
	pos    int  // current byte offset (points at ch)
	rpos   int  // next byte offset to read (one ahead of pos)
	col    int  // current column (1-based)
	line   int  // current line (1-based)

	transactionPastStatus bool
	postingExpectAccount  bool
	readingNoteAfterPipe  bool

	// subdirective is set when the current line is an account/commodity
	// directive; the next indented line then lexes as directive content
	subdirective bool
}

func New(file string, input []byte) *Lexer {
	l := &Lexer{
		file:  file,
		input: input,
		line:  1,
	}
	l.advance()
	if l.ch == '\uFEFF' { // start of the input
		l.advance()
	}
	return l
}

// Next returns next token in the input
func (l *Lexer) Next() token.Token {
	switch l.mode {
	case ModeDefault:
		return l.lexDefault()
	case ModeComment:
		return l.lexComment()
	case ModeTransaction:
		return l.lexTransaction()
	case ModePosting:
		return l.lexPosting()
	case ModePeriodic:
		return l.lexPeriodic()
	case ModeAutomated:
		return l.lexAutomated()
	case ModeDirective:
		return l.lexDirective()
	}
	panic("unreachable")
}

func (l *Lexer) lexDefault() token.Token {
	if l.subdirective && l.ch != ' ' && l.ch != '\t' {
		l.subdirective = false
	}
	switch {
	case l.ch == 0:
		return l.token(token.EOF, "")
	case l.ch == '\n':
		return l.lexNewline()
	case l.ch == '\r':
		l.col = 0
		l.advance()
		return l.lexNewline()
	case l.ch == ' ' || l.ch == '\t':
		tok := l.lexIndent()
		if l.subdirective {
			l.mode = ModeDirective
			l.subdirective = false
		} else {
			l.mode = ModePosting
		}
		l.postingExpectAccount = true
		return tok
	case l.ch == ';' || l.ch == '#' || l.ch == '%':
		l.mode = ModeComment
		return l.lexSingle(token.SEMICOLON) // todo: ??
	case l.ch == '*': // * at col 0 == comment
		l.mode = ModeComment
		return l.lexSingle(token.STAR)
	case l.ch == '~':
		l.mode = ModePeriodic
		return l.lexSingle(token.TILDE)
	case l.ch == '=':
		l.mode = ModeAutomated
		return l.lexSingle(token.EQ)
	case l.ch == '+':
		return l.lexSingle(token.PLUS)
	case l.ch == '-':
		return l.lexSingle(token.MINUS)
	case l.ch == '.':
		return l.lexSingle(token.TEXT)
	case l.ch == '!':
		return l.lexSingle(token.BANG)
	case l.ch == '@':
		return l.lexSingle(token.AT)
	case l.isAlpha():
		return l.lexKeyword()
	case l.isDigit():
		if !l.isDate() {
			s := l.save()
			for l.isDigit() || l.ch == '-' || l.ch == '/' || l.ch == '.' {
				l.advance()
			}
			return token.Token{Type: token.ILLEGAL, Literal: string(l.input[s.offset:l.pos]), Span: l.span(s)}
		}
		tok := l.lexDate()
		l.mode = ModeTransaction
		l.transactionPastStatus = false
		return tok
	default:
		s := l.save()
		l.advance()
		return token.Token{Type: token.ILLEGAL, Literal: string(l.input[s.offset:l.pos]), Span: l.span(s)}
	}
}

func (l *Lexer) lexComment() token.Token {
	if l.ch == '\n' || l.ch == '\r' || l.ch == 0 {
		l.mode = ModeDefault
		if l.ch == '\r' {
			l.col = 0
			l.advance()
		}
		return l.lexNewline()
	}

	for l.ch == ' ' || l.ch == '\t' {
		l.lexWhitespace()
	}

	if l.ch == '\n' || l.ch == '\r' || l.ch == 0 {
		l.mode = ModeDefault
		if l.ch == '\r' {
			l.col = 0
			l.advance()
		}
		return l.lexNewline()
	}

	s := l.save()
	for l.ch != '\n' && l.ch != '\r' && l.ch != 0 {
		l.advance()
	}
	return token.Token{Type: token.TEXT, Literal: string(l.input[s.offset:l.pos]), Span: l.span(s)}
}

func (l *Lexer) lexTransaction() token.Token {
	if l.readingNoteAfterPipe {
		l.readingNoteAfterPipe = false
		return l.lexNote()
	}

	switch l.ch {
	case 0:
		return l.token(token.EOF, "")
	case '\n':
		l.mode = ModeDefault
		return l.lexNewline()
	case '\r':
		l.col = 0
		l.advance()
		return l.lexNewline()
	case ' ', '\t':
		return l.lexWhitespace()
	case ';':
		l.mode = ModeComment
		return l.lexSingle(token.SEMICOLON)
	case '*':
		if !l.transactionPastStatus {
			l.transactionPastStatus = true
			return l.lexSingle(token.STAR)
		}
		return l.lexText()
	case '!':
		if !l.transactionPastStatus {
			l.transactionPastStatus = true
			return l.lexSingle(token.BANG)
		}
		return l.lexText()
	case '|':
		l.transactionPastStatus = true
		l.readingNoteAfterPipe = true
		return l.lexSingle(token.PIPE)
	case '+':
		return l.lexSingle(token.PLUS)
	case '-':
		return l.lexSingle(token.MINUS)
	case '=':
		return l.lexEquals()
	case '"', '\'':
		return l.lexString()
	default: // description / payee
		if l.isDate() { // secondsry date after =
			return l.lexDate()
		}
		return l.lexText()
	}
}

func (l *Lexer) lexNote() token.Token {
	for l.ch == ' ' || l.ch == '\t' {
		l.advance()
	}
	if l.ch == 0 || l.ch == '\n' || l.ch == '\r' || l.ch == ';' {
		return l.Next()
	}
	s := l.save()
	for l.ch != 0 && l.ch != '\n' && l.ch != '\r' && l.ch != ';' {
		l.advance()
	}
	lit := string(l.input[s.offset:l.pos])
	for len(lit) > 0 && (lit[len(lit)-1] == ' ' || lit[len(lit)-1] == '\t') {
		lit = lit[:len(lit)-1]
	}
	return token.Token{Type: token.TEXT, Literal: lit, Span: l.span(s)}
}

func (l *Lexer) lexPeriodic() token.Token {
	switch l.ch {
	case 0:
		return l.token(token.EOF, "")
	case '\n':
		l.mode = ModeDefault
		return l.lexNewline()
	case '\r':
		l.col = 0
		l.advance()
		return l.lexNewline()
	case ';':
		l.mode = ModeComment
		return l.lexSingle(token.SEMICOLON)
	case ' ', '\t':
		return l.lexWhitespace()
	default:
		return l.lexText()
	}
}

func (l *Lexer) lexAutomated() token.Token {
	switch l.ch {
	case 0:
		return l.token(token.EOF, "")
	case '\n':
		l.mode = ModeDefault
		return l.lexNewline()
	case '\r':
		l.col = 0
		l.advance()
		return l.lexNewline()
	case ' ', '\t':
		return l.lexWhitespace()
	case ';':
		l.mode = ModeComment
		return l.lexSingle(token.SEMICOLON)
	default:
		return l.lexText()
	}
}

func (l *Lexer) lexPosting() token.Token {
	switch {
	case l.ch == 0:
		l.postingExpectAccount = false
		return l.token(token.EOF, "")
	case l.ch == '\n':
		l.postingExpectAccount = false
		l.mode = ModeDefault
		return l.lexNewline()
	case l.ch == '\r':
		l.postingExpectAccount = false
		l.col = 0
		l.advance()
		l.mode = ModeDefault
		return l.lexNewline()
	case l.ch == ';':
		l.postingExpectAccount = false
		l.mode = ModeComment
		return l.lexSingle(token.SEMICOLON)
	case l.ch == ' ' || l.ch == '\t':
		return l.lexWhitespace()
	case l.postingExpectAccount && l.ch == '*':
		return l.lexSingle(token.STAR)
	case l.postingExpectAccount && l.ch == '!':
		return l.lexSingle(token.BANG)
	case l.ch == '=':
		return l.lexEquals()
	case l.ch == '@':
		return l.lexAt()
	case l.ch == '{':
		return l.lexLBrace()
	case l.ch == '}':
		return l.lexRBrace()
	case l.ch == '(':
		if !l.postingExpectAccount {
			return l.lexParenExpr()
		}
		return l.lexSingle(token.LPAREN)
	case l.ch == ')':
		return l.lexSingle(token.RPAREN)
	case l.ch == '[':
		return l.lexSingle(token.LBRACKET)
	case l.ch == ']':
		return l.lexSingle(token.RBRACKET)
	case l.ch == ':':
		return l.lexSingle(token.COLON)
	case l.postingExpectAccount && l.ch != '*' && l.ch != '!' && l.ch != '(' && l.ch != '[':
		return l.lexAccountNamePosting()
	case l.ch == '*': // after account name
		return l.lexSingle(token.STAR)
	case l.isDigit(), l.ch == '.':
		return l.lexNumber()
	case l.ch == '-':
		return l.lexSingle(token.MINUS)
	case l.ch == '+':
		return l.lexSingle(token.PLUS)
	case l.isCommodityStart():
		return l.lexCommodityMark()
	case l.ch == '"' || l.ch == '\'':
		return l.lexString()
	case l.ch >= 'a' && l.ch <= 'z':
		return l.lexCommodityMark()
	default:
		return l.lexAccountNamePosting()
	}
}

func (l *Lexer) lexDirective() token.Token {
	switch l.ch {
	case '\n', 0:
		l.mode = ModeDefault
		return l.lexNewline()
	case '\r':
		l.mode = ModeDefault
		l.col = 0
		l.advance()
		return l.lexNewline()
	case ';':
		l.mode = ModeComment
		return l.lexSingle(token.SEMICOLON)
	case ' ', '\t':
		return l.lexWhitespace()
	case '=':
		return l.lexSingle(token.EQ)
	case '+':
		return l.lexSingle(token.PLUS)
	case '-':
		return l.lexSingle(token.MINUS)
	case '"', '\'':
		return l.lexString()
	case ':':
		return l.lexSingle(token.COLON)
	case ')':
		return l.lexSingle(token.RPAREN)
	case ']':
		return l.lexSingle(token.RBRACKET)
	default:
		if l.isCommodityStart() {
			return l.lexCommodityMark()
		}
		if l.isTime() {
			return l.lexTime()
		}
		if l.isDate() {
			return l.lexDate()
		}
		if l.isDigit() {
			return l.lexNumber()
		}
		return l.lexAccountNameDirective()
	}
}

func (l *Lexer) lexSingle(kind token.Type) token.Token {
	s := l.save()
	l.advance()
	return token.Token{
		Type:    kind,
		Literal: string(l.input[s.offset:l.pos]),
		Span:    l.span(s),
	}
}

func (l *Lexer) lexNewline() token.Token {
	s := l.save()
	l.advance()
	l.mode = ModeDefault
	return token.Token{Type: token.NEWLINE, Literal: "\n", Span: l.span(s)}
}

func (l *Lexer) lexWhitespace() token.Token {
	s := l.save()
	for l.ch == ' ' || l.ch == '\t' {
		l.advance()
	}
	lit := string(l.input[s.offset:l.pos])
	return token.Token{Type: token.WHITESPACE, Literal: lit, Span: l.span(s)}
}

func (l *Lexer) lexIndent() token.Token {
	s := l.save()
	for l.ch == ' ' || l.ch == '\t' {
		l.advance()
	}
	lit := string(l.input[s.offset:l.pos])
	return token.Token{Type: token.INDENT, Literal: lit, Span: l.span(s)}
}

func (l *Lexer) lexEquals() token.Token {
	s := l.save()
	l.advance()
	if l.ch == '=' {
		l.advance()
		switch l.ch {
		case '=':
			l.advance()
			return token.Token{Type: token.EQEQEQ, Literal: "===", Span: l.span(s)}
		case '*':
			l.advance()
			return token.Token{Type: token.EQEQEQ, Literal: "==*", Span: l.span(s)}
		default:
			return token.Token{Type: token.EQEQ, Literal: "==", Span: l.span(s)}
		}
	}
	if l.ch == '*' {
		l.advance()
		return token.Token{Type: token.EQSTAR, Literal: "=*", Span: l.span(s)}
	}
	return token.Token{Type: token.EQ, Literal: "=", Span: l.span(s)}
}

func (l *Lexer) lexAt() token.Token {
	s := l.save()
	l.advance()
	if l.ch == '@' {
		l.advance()
		return token.Token{Type: token.ATAT, Literal: "@@", Span: l.span(s)}
	}
	return token.Token{Type: token.AT, Literal: "@", Span: l.span(s)}
}

func (l *Lexer) lexText() token.Token {
	s := l.save()
	l.advance()
	for l.ch != '\n' && l.ch != '\r' && l.ch != ';' && l.ch != 0 && l.ch != ' ' && l.ch != '\t' {
		l.advance()
	}
	lit := string(l.input[s.offset:l.pos])
	return token.Token{Type: token.TEXT, Literal: lit, Span: l.span(s)}
}

// lexAccountNameDirective reads accout name in directive context.
// stops at any whitespace, supports multi-word names("Taxi Fare").
func (l *Lexer) lexAccountNameDirective() token.Token {
	s := l.save()
	for l.ch != '\n' && l.ch != '\r' && l.ch != ';' && l.ch != 0 && l.ch != ')' && l.ch != ']' && l.ch != ':' && l.ch != ' ' && l.ch != '\t' {
		l.advance()
	}
	lit := string(l.input[s.offset:l.pos])
	return token.Token{Type: token.TEXT, Literal: lit, Span: l.span(s)}
}

// lexAccountNamePosting reads an account name in posting context.
// stops at two consecutive spaces.
func (l *Lexer) lexAccountNamePosting() token.Token {
	s := l.save()
	for l.ch != '\n' && l.ch != '\r' && l.ch != ';' && l.ch != 0 && l.ch != ')' && l.ch != ']' && l.ch != ':' {
		if l.isTwoSpaces() {
			break
		}
		l.advance()
	}
	if l.ch != ':' {
		l.postingExpectAccount = false
	}
	lit := string(l.input[s.offset:l.pos])
	return token.Token{Type: token.TEXT, Literal: lit, Span: l.span(s)}
}

func (l *Lexer) lexParenExpr() token.Token {
	s := l.save()
	depth := 0
	for l.ch != '\n' && l.ch != '\r' && l.ch != 0 {
		if l.ch == '(' {
			depth++
		} else if l.ch == ')' {
			depth--
			if depth == 0 {
				l.advance()
				break
			}
		}
		l.advance()
	}
	lit := string(l.input[s.offset:l.pos])
	return token.Token{Type: token.PARENEXPR, Literal: lit, Span: l.span(s)}
}

func (l *Lexer) lexNumber() token.Token {
	s := l.save()
	for {
		if l.isDigit() || l.ch == '.' || l.ch == ',' || l.ch == '_' || l.ch == '\'' {
			l.advance()
		} else if l.ch == ' ' && (l.peek() >= '0' && l.peek() <= '9') {
			l.advance()
		} else if l.ch == 'e' || l.ch == 'E' {
			// exponent: consume only when E[sign]digit, so `10E ` stays an integer
			p := l.pos + 1
			if p < len(l.input) && (l.input[p] == '+' || l.input[p] == '-') {
				p++
			}
			if p >= len(l.input) || l.input[p] < '0' || l.input[p] > '9' {
				break
			}
			l.advance() // e/E
			if l.ch == '+' || l.ch == '-' {
				l.advance()
			}
			for l.isDigit() {
				l.advance()
			}
		} else {
			break
		}
	}
	lit := string(l.input[s.offset:l.pos])
	kind := token.INT
	if strings.ContainsAny(lit, "., eE") {
		kind = token.DECIMAL
	}
	return token.Token{Type: kind, Literal: lit, Span: l.span(s)}
}

func (l *Lexer) lexKeyword() token.Token {
	s := l.save()
	for l.ch != 0 && l.ch != '\n' && l.ch != '\r' && l.ch != ' ' && l.ch != '\t' && l.ch != ';' {
		l.advance()
	}
	lit := string(l.input[s.offset:l.pos])
	kind := l.keyword(lit)
	if kind == token.ILLEGAL { // todo: report an error ??
		kind = token.TEXT
	} else {
		l.mode = ModeDirective
		l.subdirective = lit == "account" || lit == "commodity"
	}
	return token.Token{Type: kind, Literal: lit, Span: l.span(s)}
}

func (l *Lexer) lexDate() token.Token {
	s := l.save()
	for l.isDigit() || (l.isDateSep() && l.peekIsDigit()) {
		l.advance()
	}
	return token.Token{Type: token.DATE, Literal: string(l.input[s.offset:l.pos]), Span: l.span(s)}
}

func isSymbolChar(r rune) bool {
	return r == '$' || unicode.In(r, unicode.Sc)
}

func (l *Lexer) lexString() token.Token {
	s := l.save()
	quote := l.ch
	l.advance() // consume the quote character
	for l.ch != quote && l.ch != '\n' && l.ch != 0 {
		l.advance()
	}
	if l.ch == quote {
		l.advance()
	}
	return token.Token{Type: token.STRING, Literal: string(l.input[s.offset:l.pos]), Span: l.span(s)}
}

func (l *Lexer) lexCommodityMark() token.Token {
	s := l.save()

	if l.ch == '"' {
		l.advance()
		for l.ch != '"' && l.ch != '\n' && l.ch != 0 {
			l.advance()
		}
		if l.ch == '"' {
			l.advance()
		}
		return token.Token{Type: token.COMMODITYMARK, Literal: string(l.input[s.offset:l.pos]), Span: l.span(s)}
	}

	if unicode.IsLetter(l.ch) {
		for unicode.IsLetter(l.ch) {
			l.advance()
		}
		return token.Token{Type: token.COMMODITYMARK, Literal: string(l.input[s.offset:l.pos]), Span: l.span(s)}
	}

	if isSymbolChar(l.ch) {
		for isSymbolChar(l.ch) {
			l.advance()
		}
		return token.Token{Type: token.COMMODITYMARK, Literal: string(l.input[s.offset:l.pos]), Span: l.span(s)}
	}

	l.advance()
	return token.Token{Type: token.COMMODITYMARK, Literal: string(l.input[s.offset:l.pos]), Span: l.span(s)}
}

func (l *Lexer) lexLBrace() token.Token {
	s := l.save()
	l.advance()
	if l.ch == '{' {
		l.advance()
		return token.Token{Type: token.LBRACELBRACE, Literal: "{{", Span: l.span(s)}
	}
	return token.Token{Type: token.LBRACE, Literal: "{", Span: l.span(s)}
}

func (l *Lexer) lexRBrace() token.Token {
	s := l.save()
	l.advance()
	if l.ch == '}' {
		l.advance()
		return token.Token{Type: token.RBRACERBRACE, Literal: "}}", Span: l.span(s)}
	}
	return token.Token{Type: token.RBRACE, Literal: "}", Span: l.span(s)}
}

func (l *Lexer) advance() {
	if l.rpos >= len(l.input) {
		l.ch = 0
		l.chSize = 0
	} else {
		r, size := utf8.DecodeRune(l.input[l.rpos:])
		l.ch = r
		l.chSize = size
	}
	l.pos = l.rpos
	l.rpos += l.chSize
	if l.ch == '\n' || l.ch == '\r' {
		l.line++
		l.col = 0
	} else {
		l.col++
	}
}

func (l *Lexer) peek() rune {
	r, _ := utf8.DecodeRune(l.input[l.rpos:])
	return r
}

func (l *Lexer) peekN(n int) byte {
	if l.pos+n >= len(l.input) {
		return 0
	}
	return l.input[l.pos+n]
}

func (l *Lexer) isDigit() bool { return l.ch >= '0' && l.ch <= '9' }
func (l *Lexer) isAlpha() bool {
	return (l.ch >= 'a' && l.ch <= 'z') ||
		(l.ch >= 'A' && l.ch <= 'Z')
}

func (l *Lexer) isTwoSpaces() bool { return l.ch == ' ' && l.peek() == ' ' }

func (l *Lexer) isDateSep() bool { return l.ch == '-' || l.ch == '/' || l.ch == '.' }

func (l *Lexer) peekIsDigit() bool {
	r := l.peek()
	return r >= '0' && r <= '9'
}

func (l *Lexer) isCommodityStart() bool {
	if l.ch == '$' || (l.ch >= 'A' && l.ch <= 'Z') {
		return true
	}
	if l.ch < utf8.RuneSelf {
		return false
	}
	return unicode.In(l.ch, unicode.Sc) || unicode.IsLetter(l.ch)
}

func (l *Lexer) isDate() bool {
	if !l.isDigit() {
		return false
	}
	// YYYY/M/D or YYYY/MM/DD
	if l.peekN(1) >= '0' && l.peekN(1) <= '9' &&
		l.peekN(2) >= '0' && l.peekN(2) <= '9' &&
		l.peekN(3) >= '0' && l.peekN(3) <= '9' {
		sep := l.peekN(4)
		if sep == '/' || sep == '-' || sep == '.' {
			if l.peekN(5) >= '0' && l.peekN(5) <= '9' {
				if l.peekN(6) == sep {
					return l.peekN(7) >= '0' && l.peekN(7) <= '9'
				}
				if l.peekN(7) == sep {
					return l.peekN(8) >= '0' && l.peekN(8) <= '9'
				}
			}
		}
		return false
	}
	// M/D or MM/DD(year inferred, only / and - separators; . is ambiguous with decimal numbers like 1.01)
	if (l.peekN(1) == '/' || l.peekN(1) == '-') &&
		l.peekN(2) >= '0' && l.peekN(2) <= '9' &&
		l.ch >= '1' && l.ch <= '9' {
		return validDay(l.peekN(2), l.peekN(3))
	}
	if (l.peekN(2) == '/' || l.peekN(2) == '-') &&
		l.peekN(3) >= '0' && l.peekN(3) <= '9' {
		m := int(l.ch-'0')*10 + int(l.peekN(1)-'0')
		return m >= 1 && m <= 12 && validDay(l.peekN(3), l.peekN(4))
	}
	return false
}

func validDay(first, second byte) bool {
	d := int(first - '0')
	if second >= '0' && second <= '9' {
		d = d*10 + int(second-'0')
	}
	return d >= 1 && d <= 31
}

func (l *Lexer) isTime() bool {
	if !l.isDigit() {
		return false
	}
	return l.peekN(2) == ':'
}

func (l *Lexer) lexTime() token.Token {
	s := l.save()
	for l.isDigit() || l.ch == ':' {
		l.advance()
	}
	return token.Token{Type: token.TIME, Literal: string(l.input[s.offset:l.pos]), Span: l.span(s)}
}

type savedPos struct{ offset, line, col int }

func (l *Lexer) save() savedPos {
	return savedPos{l.pos, l.line, l.col}
}

func (l *Lexer) span(s savedPos) token.Span {
	return token.Span{
		Start: token.Pos{File: l.file, Offset: s.offset, Line: s.line, Col: s.col},
		End:   token.Pos{File: l.file, Offset: l.pos, Line: l.line, Col: l.col},
	}
}

func (l *Lexer) token(kind token.Type, literal string) token.Token {
	s := savedPos{l.pos, l.line, l.col}
	return token.Token{Type: kind, Literal: literal, Span: l.span(s)}
}

func (l *Lexer) keyword(s string) token.Type {
	switch s {
	case "comment":
		return token.COMMENTKW
	case "account":
		return token.ACCOUNT
	case "commodity":
		return token.COMMODITY
	case "include":
		return token.INCLUDE
	case "alias":
		return token.ALIAS
	case "payee":
		return token.PAYEE
	case "tag":
		return token.TAG
	case "apply":
		return token.APPLY
	case "end":
		return token.END
	case "Y", "year":
		return token.YEAR
	case "decimal-mark":
		return token.DECIMALMARK
	case "D":
		return token.D
	case "P":
		return token.P
	case "N":
		return token.N
	case "C":
		return token.C
	default:
		return token.ILLEGAL
	}
}
