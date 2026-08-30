// Package lex implements the Niraluli Tamil-0 lexer.
package lex

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"niraluli/internal/token"
)

// Error is a lexing error with source position.
type Error struct {
	Pos token.Pos
	Msg string
}

func (e Error) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Pos.Line, e.Pos.Col, e.Msg)
}

// Lexer tokenizes UTF-8 Niraluli source.
type Lexer struct {
	src        string
	file       string
	off        int
	line       int
	col        int
	errs       []error
	insertSemi bool
}

// New creates a lexer for src. file is used only in messages (may be empty).
func New(file, src string) *Lexer {
	return &Lexer{src: src, file: file, line: 1, col: 1}
}

// Errors returns lexing errors collected so far.
func (l *Lexer) Errors() []error { return l.errs }

func (l *Lexer) errf(pos token.Pos, format string, args ...any) {
	l.errs = append(l.errs, Error{Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

func (l *Lexer) peek() rune {
	if l.off >= len(l.src) {
		return -1
	}
	r, _ := utf8.DecodeRuneInString(l.src[l.off:])
	if r == utf8.RuneError {
		return -1
	}
	return r
}

func (l *Lexer) next() rune {
	if l.off >= len(l.src) {
		return -1
	}
	r, size := utf8.DecodeRuneInString(l.src[l.off:])
	l.off += size
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

func (l *Lexer) pos() token.Pos {
	return token.Pos{Line: l.line, Col: l.col, Offset: l.off}
}

// skipSpace skips spaces/tabs/CRs and newlines. Reports if a newline was seen.
func (l *Lexer) skipSpace() (newline bool) {
	for {
		r := l.peek()
		if r == ' ' || r == '\t' || r == '\r' {
			l.next()
			continue
		}
		if r == '\n' {
			newline = true
			l.next()
			continue
		}
		break
	}
	return newline
}

func (l *Lexer) skipLineComment() {
	for {
		r := l.peek()
		if r < 0 || r == '\n' {
			return
		}
		l.next()
	}
}

func (l *Lexer) skipBlockComment(start token.Pos) (newline bool) {
	for {
		r := l.peek()
		if r < 0 {
			l.errf(start, "unterminated block comment")
			return newline
		}
		if r == '\n' {
			newline = true
		}
		if r == '*' {
			l.next()
			if l.peek() == '/' {
				l.next()
				return newline
			}
			continue
		}
		l.next()
	}
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// isNumDigit accepts Arabic and Tamil decimal digits in numeric literals.
func isNumDigit(r rune) bool {
	return isASCIIDigit(r) || (r >= '௦' && r <= '௯')
}

func digitValue(r rune) (int, bool) {
	switch {
	case isASCIIDigit(r):
		return int(r - '0'), true
	case r >= '௦' && r <= '௯':
		return int(r - '௦'), true
	default:
		return 0, false
	}
}

func isIdentContinue(r rune) bool {
	// Tamil orthography uses combining marks (vowel signs, virama).
	// Identifiers use Arabic digits only; Tamil digits are for literals.
	return isIdentStart(r) || isASCIIDigit(r) || unicode.IsMark(r)
}

func (l *Lexer) readIdent(start token.Pos) token.Token {
	beg := start.Offset
	for {
		r := l.peek()
		if !isIdentContinue(r) {
			break
		}
		l.next()
	}
	lit := l.src[beg:l.off]
	return token.Token{Kind: token.LookupIdent(lit), Lit: lit, Pos: start}
}

func (l *Lexer) readNumber(start token.Pos) token.Token {
	// Normalize Tamil digits to ASCII in Lit so strconv works downstream.
	var ascii []byte
	for {
		r := l.peek()
		if v, ok := digitValue(r); ok {
			ascii = append(ascii, byte('0'+v))
			l.next()
			continue
		}
		break
	}
	if l.peek() == '.' {
		l.next()
		ascii = append(ascii, '.')
		for {
			r := l.peek()
			if v, ok := digitValue(r); ok {
				ascii = append(ascii, byte('0'+v))
				l.next()
				continue
			}
			break
		}
		return token.Token{Kind: token.FLOAT, Lit: string(ascii), Pos: start}
	}
	return token.Token{Kind: token.INT, Lit: string(ascii), Pos: start}
}

func (l *Lexer) readFloatFrac(start token.Pos) token.Token {
	// Leading '.' already consumed; require at least one fraction digit.
	var ascii []byte
	ascii = append(ascii, '.')
	for {
		r := l.peek()
		if v, ok := digitValue(r); ok {
			ascii = append(ascii, byte('0'+v))
			l.next()
			continue
		}
		break
	}
	return token.Token{Kind: token.FLOAT, Lit: string(ascii), Pos: start}
}

func (l *Lexer) readString(start token.Pos) token.Token {
	// consume opening "
	l.next()
	var b strings.Builder
	for {
		r := l.peek()
		if r < 0 {
			l.errf(start, "unterminated string literal")
			return token.Token{Kind: token.ILLEGAL, Lit: b.String(), Pos: start}
		}
		if r == '\n' {
			l.errf(start, "newline in string literal")
			return token.Token{Kind: token.ILLEGAL, Lit: b.String(), Pos: start}
		}
		if r == '\\' {
			l.next()
			esc := l.peek()
			if esc < 0 {
				l.errf(start, "unterminated string escape")
				return token.Token{Kind: token.ILLEGAL, Lit: b.String(), Pos: start}
			}
			l.next()
			switch esc {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			default:
				l.errf(start, "unknown escape \\%c", esc)
			}
			continue
		}
		if r == '"' {
			l.next() // closing quote
			return token.Token{Kind: token.STRING, Lit: b.String(), Pos: start}
		}
		b.WriteRune(r)
		l.next()
	}
}

func insertsSemi(k token.Kind) bool {
	switch k {
	case token.IDENT, token.INT, token.STRING, token.TRUE, token.FALSE, token.NIL,
		token.BREAK, token.CONTINUE, token.RETURN,
		token.RPAREN, token.RBRACK, token.RBRACE,
		token.TYPE_INT, token.TYPE_BOOL, token.TYPE_STRING:
		return true
	default:
		return false
	}
}

func (l *Lexer) finish(tok token.Token) token.Token {
	l.insertSemi = insertsSemi(tok.Kind)
	return tok
}

// Next returns the next non-comment token.
// Applies Go-style automatic semicolon insertion after line-final tokens.
func (l *Lexer) Next() token.Token {
	for {
		newline := l.skipSpace()
		start := l.pos()
		r := l.peek()
		if r < 0 {
			if l.insertSemi {
				l.insertSemi = false
				return token.Token{Kind: token.SEMICOLON, Lit: "\n", Pos: start}
			}
			return token.Token{Kind: token.EOF, Pos: start}
		}
		if newline && l.insertSemi {
			l.insertSemi = false
			return token.Token{Kind: token.SEMICOLON, Lit: "\n", Pos: start}
		}

		// comments
		if r == '/' {
			l.next()
			switch l.peek() {
			case '/':
				l.next()
				l.skipLineComment()
				continue
			case '*':
				l.next()
				if l.skipBlockComment(start) && l.insertSemi {
					// newline inside comment acts like ASI whitespace
					l.insertSemi = false
					return token.Token{Kind: token.SEMICOLON, Lit: "\n", Pos: l.pos()}
				}
				continue
			default:
				return l.finish(token.Token{Kind: token.QUO, Lit: "/", Pos: start})
			}
		}

		if isIdentStart(r) {
			return l.finish(l.readIdent(start))
		}
		if isNumDigit(r) {
			return l.finish(l.readNumber(start))
		}
		if r == '"' {
			return l.finish(l.readString(start))
		}

		switch r {
		case '+':
			l.next()
			return l.finish(token.Token{Kind: token.ADD, Lit: "+", Pos: start})
		case '-':
			l.next()
			return l.finish(token.Token{Kind: token.SUB, Lit: "-", Pos: start})
		case '*':
			l.next()
			return l.finish(token.Token{Kind: token.MUL, Lit: "*", Pos: start})
		case '&':
			l.next()
			return l.finish(token.Token{Kind: token.AND, Lit: "&", Pos: start})
		case '|':
			l.next()
			return l.finish(token.Token{Kind: token.OR, Lit: "|", Pos: start})
		case '%':
			l.next()
			return l.finish(token.Token{Kind: token.REM, Lit: "%", Pos: start})
		case '(':
			l.next()
			return l.finish(token.Token{Kind: token.LPAREN, Lit: "(", Pos: start})
		case ')':
			l.next()
			return l.finish(token.Token{Kind: token.RPAREN, Lit: ")", Pos: start})
		case '[':
			l.next()
			return l.finish(token.Token{Kind: token.LBRACK, Lit: "[", Pos: start})
		case ']':
			l.next()
			return l.finish(token.Token{Kind: token.RBRACK, Lit: "]", Pos: start})
		case '{':
			l.next()
			return l.finish(token.Token{Kind: token.LBRACE, Lit: "{", Pos: start})
		case '}':
			l.next()
			return l.finish(token.Token{Kind: token.RBRACE, Lit: "}", Pos: start})
		case ',':
			l.next()
			return l.finish(token.Token{Kind: token.COMMA, Lit: ",", Pos: start})
		case ';':
			l.next()
			l.insertSemi = false
			return token.Token{Kind: token.SEMICOLON, Lit: ";", Pos: start}
		case '.':
			l.next()
			if l.peek() == '.' {
				l.next()
				if l.peek() == '.' {
					l.next()
					return l.finish(token.Token{Kind: token.ELLIPSIS, Lit: "...", Pos: start})
				}
				// Lone ".." is illegal; treat as two periods by backing up one.
				// Simpler: emit ELLIPSIS only for "..."; otherwise PERIOD then let
				// the next '.' be lexed separately — but we already consumed two.
				return l.finish(token.Token{Kind: token.ILLEGAL, Lit: "..", Pos: start})
			}
			if isNumDigit(l.peek()) {
				return l.finish(l.readFloatFrac(start))
			}
			return l.finish(token.Token{Kind: token.PERIOD, Lit: ".", Pos: start})
		case '!':
			l.next()
			if l.peek() == '=' {
				l.next()
				return l.finish(token.Token{Kind: token.NEQ, Lit: "!=", Pos: start})
			}
			return l.finish(token.Token{Kind: token.NOT, Lit: "!", Pos: start})
		case '=':
			l.next()
			if l.peek() == '=' {
				l.next()
				return l.finish(token.Token{Kind: token.EQL, Lit: "==", Pos: start})
			}
			return l.finish(token.Token{Kind: token.ASSIGN, Lit: "=", Pos: start})
		case ':':
			l.next()
			if l.peek() == '=' {
				l.next()
				return l.finish(token.Token{Kind: token.DEFINE, Lit: ":=", Pos: start})
			}
			return l.finish(token.Token{Kind: token.COLON, Lit: ":", Pos: start})
		case '<':
			l.next()
			if l.peek() == '-' {
				l.next()
				return l.finish(token.Token{Kind: token.ARROW, Lit: "<-", Pos: start})
			}
			if l.peek() == '=' {
				l.next()
				return l.finish(token.Token{Kind: token.LEQ, Lit: "<=", Pos: start})
			}
			return l.finish(token.Token{Kind: token.LSS, Lit: "<", Pos: start})
		case '>':
			l.next()
			if l.peek() == '=' {
				l.next()
				return l.finish(token.Token{Kind: token.GEQ, Lit: ">=", Pos: start})
			}
			return l.finish(token.Token{Kind: token.GTR, Lit: ">", Pos: start})
		default:
			l.next()
			l.errf(start, "unexpected character %q", r)
			return l.finish(token.Token{Kind: token.ILLEGAL, Lit: string(r), Pos: start})
		}
	}
}

// All returns every token until EOF (inclusive).
func (l *Lexer) All() []token.Token {
	var out []token.Token
	for {
		tok := l.Next()
		out = append(out, tok)
		if tok.Kind == token.EOF {
			return out
		}
	}
}
