package parser

import (
	"strconv"
	"strings"

	"modernc.org/golex/lex"
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
	*golexer
	pending *token
}

func newLexer(src string) *lexer {
	lx, err := newGolexer(strings.NewReader(src), "input.mygo")
	if err != nil {
		panic(err)
	}
	return &lexer{golexer: lx}
}

func (l *lexer) nextToken() token {
	if l.pending != nil {
		tok := *l.pending
		l.pending = nil
		return tok
	}
	c := l.scan()
	pos := l.File.Position(c.Pos())
	switch c.Rune {
	case lex.RuneEOF:
		return token{kind: tokEOF, line: pos.Line, col: pos.Column}
	case NEWLINE:
		return token{kind: tokNewline, lit: "\n", line: pos.Line, col: pos.Column}
	case IDENT:
		return token{kind: tokIdent, lit: string(l.TokenBytes(nil)), line: pos.Line, col: pos.Column}
	case NUMBER:
		return token{kind: tokNumber, lit: string(l.TokenBytes(nil)), line: pos.Line, col: pos.Column}
	case STRING:
		lit, err := strconv.Unquote(string(l.TokenBytes(nil)))
		if err != nil {
			lit = string(l.TokenBytes(nil))
		}
		return token{kind: tokString, lit: lit, line: pos.Line, col: pos.Column}
	case PACKAGE, IMPORT, ENUM, STRUCT, INTERFACE, IMPL, FUNC, IF, THEN, ELSE, SWITCH, CASE, END, USING, NOT, LET, VAR, EMBED, WHILE, RETURN:
		return token{kind: tokKeyword, lit: string(l.TokenBytes(nil)), line: pos.Line, col: pos.Column}
	case SLICE:
		l.pending = &token{kind: tokSym, lit: "]", line: pos.Line, col: pos.Column}
		return token{kind: tokSym, lit: "[", line: pos.Line, col: pos.Column}
	default:
		return token{kind: tokSym, lit: string(l.TokenBytes(nil)), line: pos.Line, col: pos.Column}
	}
}
