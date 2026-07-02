package parser

import (
	"strings"
	"unicode"
)

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokNewline
	tokIdent
	tokNumber
	tokString
	tokKeyword
	tokSym
)

type token struct {
	kind tokenKind
	lit  string
	line int
	col  int
}

type lexer struct {
	src  []rune
	pos  int
	line int
	col  int
}

func newLexer(src string) *lexer { return &lexer{src: []rune(src), line: 1, col: 1} }

func (l *lexer) nextToken() token {
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == '\n' {
			l.pos++
			tok := token{kind: tokNewline, lit: "\n", line: l.line, col: l.col}
			l.line++
			l.col = 1
			return tok
		}
		if unicode.IsSpace(ch) {
			l.pos++
			l.col++
			continue
		}
		if ch == '#' {
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
				l.col++
			}
			continue
		}
		break
	}
	if l.pos >= len(l.src) {
		return token{kind: tokEOF, line: l.line, col: l.col}
	}

	ch := l.src[l.pos]
	startCol := l.col
	switch {
	case isIdentStart(ch):
		start := l.pos
		l.pos++
		l.col++
		for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
			l.pos++
			l.col++
		}
		lit := string(l.src[start:l.pos])
		if isKeyword(lit) {
			return token{kind: tokKeyword, lit: lit, line: l.line, col: startCol}
		}
		return token{kind: tokIdent, lit: lit, line: l.line, col: startCol}
	case unicode.IsDigit(ch):
		start := l.pos
		l.pos++
		l.col++
		for l.pos < len(l.src) && unicode.IsDigit(l.src[l.pos]) {
			l.pos++
			l.col++
		}
		if l.pos+1 < len(l.src) && l.src[l.pos] == '.' && unicode.IsDigit(l.src[l.pos+1]) {
			l.pos++
			l.col++
			for l.pos < len(l.src) && unicode.IsDigit(l.src[l.pos]) {
				l.pos++
				l.col++
			}
		}
		return token{kind: tokNumber, lit: string(l.src[start:l.pos]), line: l.line, col: startCol}
	case ch == '"':
		l.pos++
		l.col++
		var b strings.Builder
		for l.pos < len(l.src) {
			c := l.src[l.pos]
			l.pos++
			l.col++
			if c == '"' {
				break
			}
			if c == '\\' && l.pos < len(l.src) {
				next := l.src[l.pos]
				l.pos++
				l.col++
				switch next {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				case '"':
					b.WriteByte('"')
				case '\\':
					b.WriteByte('\\')
				default:
					b.WriteRune(next)
				}
				continue
			}
			b.WriteRune(c)
		}
		return token{kind: tokString, lit: b.String(), line: l.line, col: startCol}
	default:
		if l.match("=>") || l.match("->") || l.match("<=") || l.match(">=") || l.match("<|") || l.match("|>") || l.match("==") || l.match("!=") || l.match("&&") || l.match("||") {
			return token{kind: tokSym, lit: string(l.src[l.pos-2 : l.pos]), line: l.line, col: startCol}
		}
		l.pos++
		l.col++
		return token{kind: tokSym, lit: string(ch), line: l.line, col: startCol}
	}
}

func (l *lexer) match(s string) bool {
	runes := []rune(s)
	if l.pos+len(runes) > len(l.src) {
		return false
	}
	for i, r := range runes {
		if l.src[l.pos+i] != r {
			return false
		}
	}
	l.pos += len(runes)
	l.col += len(runes)
	return true
}

func isIdentStart(r rune) bool { return unicode.IsLetter(r) || r == '_' }
func isIdentPart(r rune) bool  { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

func isKeyword(s string) bool {
	switch s {
	case "package", "import", "enum", "struct", "interface", "impl", "func", "if", "then", "else", "switch", "case", "end", "where", "not", "let", "var", "embed", "while":
		return true
	default:
		return false
	}
}
