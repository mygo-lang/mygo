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
	braceDepth int
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
		if l.braceDepth > 0 {
			// Skip NEWLINE inside braces
			return l.nextToken()
		}
		return token{kind: tokNewline, lit: "\n", line: pos.Line, col: pos.Column}
	case IDENT:
		return token{kind: tokIdent, lit: string(l.TokenBytes(nil)), line: pos.Line, col: pos.Column}
	case NUMBER:
		return token{kind: tokNumber, lit: string(l.TokenBytes(nil)), line: pos.Line, col: pos.Column}
	case STRING:
		if l.multilineContent != "" {
			lit := l.multilineContent
			l.multilineContent = ""
			return token{kind: tokString, lit: lit, line: pos.Line, col: pos.Column}
		}
		raw := string(l.TokenBytes(nil))
		// Backtick string (raw string literal): no escape processing
		if len(raw) >= 2 && raw[0] == '`' && raw[len(raw)-1] == '`' {
			return token{kind: tokString, lit: raw[1 : len(raw)-1], line: pos.Line, col: pos.Column}
		}
		// Normal string with escape processing
		lit, err := strconv.Unquote(raw)
		if err != nil {
			lit = raw
		}
		return token{kind: tokString, lit: lit, line: pos.Line, col: pos.Column}
	case PACKAGE, IMPORT, ENUM, STRUCT, INTERFACE, IMPL, FUNC, IF, THEN, ELSE, SWITCH, CASE, END, USING, NOT, LET, VAR, EMBED, WHILE, RETURN, GO, IN, TYPE:
		return token{kind: tokKeyword, lit: string(l.TokenBytes(nil)), line: pos.Line, col: pos.Column}
	case SLICE:
		l.pending = &token{kind: tokSym, lit: "]", line: pos.Line, col: pos.Column}
		return token{kind: tokSym, lit: "[", line: pos.Line, col: pos.Column}
	default:
		lit := string(l.TokenBytes(nil))
		switch lit {
		case "{":
			l.braceDepth++
		case "}":
			l.braceDepth--
		}
		return token{kind: tokSym, lit: lit, line: pos.Line, col: pos.Column}
	}
}

func (l *golexer) scanMultilineString() lex.Char {
	var buf strings.Builder
	for {
		r := l.Next()
		if r == lex.RuneEOF {
			break
		}
		if r == '"' {
			r2 := l.Next()
			if r2 == lex.RuneEOF {
				break
			}
			r3 := l.Next()
			if r3 == lex.RuneEOF {
				break
			}
			if r2 == '"' && r3 == '"' {
				// Consume the third " from lookahead so it is not left
				// for the next token.
				_ = l.Next()
				break
			}
			buf.WriteRune(rune('"'))
			buf.WriteRune(rune(r2))
			buf.WriteRune(rune(r3))
			continue
		}
		buf.WriteRune(rune(r))
	}
	l.multilineContent = buf.String()
	return l.char(STRING)
}
