package parser

import (
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func (p *parser) peek() token {
	if p.skipNL {
		p.skipNewlines()
	}
	return p.peekRaw()
}

func (p *parser) peekRaw() token {
	if p.pos >= len(p.toks) {
		return token{kind: tokEOF, line: p.lineAtEOF()}
	}
	return p.toks[p.pos]
}

func (p *parser) lineAtEOF() int {
	if len(p.toks) == 0 {
		return 1
	}
	return p.toks[len(p.toks)-1].line
}

func (p *parser) next() token {
	if p.skipNL {
		p.skipNewlines()
	}
	return p.nextRaw()
}

func (p *parser) nextRaw() token {
	tok := p.peekRaw()
	if p.pos < len(p.toks) {
		p.pos++
	}
	return tok
}

func (p *parser) skipNewlines() {
	for p.peekRaw().kind == tokNewline {
		p.pos++
	}
}

func (p *parser) peekEOF() bool { return p.peek().kind == tokEOF }

func (p *parser) peekKeyword(s string) bool {
	tok := p.peek()
	return tok.kind == tokKeyword && tok.lit == s
}

func (p *parser) peekSym(s string) bool {
	tok := p.peek()
	return tok.kind == tokSym && tok.lit == s
}

func (p *parser) matchSym(s string) bool {
	if p.peekSym(s) {
		p.pos++
		return true
	}
	return false
}

func (p *parser) expectKeyword(s string) error {
	tok := p.next()
	if tok.kind != tokKeyword || tok.lit != s {
		return common.ErrorAtPos(tok.line, tok.col, "expected keyword %q, got %q", s, tok.lit)
	}
	return nil
}

func (p *parser) expectSym(s string) error {
	tok := p.next()
	if tok.kind != tokSym || tok.lit != s {
		return common.ErrorAtPos(tok.line, tok.col, "expected %q, got %q", s, tok.lit)
	}
	return nil
}

func (p *parser) expectIdent() (string, error) {
	tok := p.next()
	if tok.kind != tokIdent {
		return "", common.ErrorAtPos(tok.line, tok.col, "expected identifier, got %q", tok.lit)
	}
	return tok.lit, nil
}

func (p *parser) parseExprListUntil(terms ...string) (Expr, error) {
	prev := p.skipNL
	p.skipNL = false
	defer func() { p.skipNL = prev }()
	var stmts []Stmt
	for {
		p.skipNewlines()
		if p.peekRaw().kind == tokEOF || p.peekAnyKeywordRaw(terms...) {
			break
		}
		stmt, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, stmt)
		if p.peekRaw().kind == tokEOF || p.peekAnyKeywordRaw(terms...) {
			break
		}
		if p.peekRaw().kind != tokNewline {
			tok := p.peekRaw()
			return nil, common.ErrorAtPos(tok.line, tok.col, "expected newline before %q", tok.lit)
		}
	}
	if len(stmts) == 0 {
		tok := p.peekRaw()
		return nil, common.ErrorAtPos(tok.line, tok.col, "expected expression")
	}
	if len(stmts) == 1 {
		if exprStmt, ok := stmts[0].(*ExprStmt); ok {
			return exprStmt.Expr, nil
		}
	}
	tok := p.peekRaw()
	return &BlockExpr{Line: tok.line, Column: tok.col, Stmts: stmts}, nil
}

func (p *parser) peekAnyKeyword(terms ...string) bool {
	for _, term := range terms {
		if p.peekKeyword(term) {
			return true
		}
	}
	return false
}

func (p *parser) peekAnyKeywordRaw(terms ...string) bool {
	for _, term := range terms {
		tok := p.peekRaw()
		if tok.kind == tokKeyword && tok.lit == term {
			return true
		}
	}
	return false
}

func (p *parser) parseStmt() (Stmt, error) {
	switch {
	case p.peekRaw().kind == tokKeyword && p.peekRaw().lit == "let":
		return p.parseBindingStmt(false)
	case p.peekRaw().kind == tokKeyword && p.peekRaw().lit == "var":
		return p.parseBindingStmt(true)
	case p.peekRaw().kind == tokIdent && p.peekRawN(1).kind == tokSym && p.peekRawN(1).lit == "=":
		name, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		if err := p.expectSym("="); err != nil {
			return nil, err
		}
		value, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		tok := p.peekRaw()
		return &AssignStmt{Line: tok.line, Column: tok.col, Name: name, Value: value}, nil
	default:
		expr, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		tok := p.peekRaw()
		return &ExprStmt{Line: tok.line, Column: tok.col, Expr: expr}, nil
	}
}

func (p *parser) parseBindingStmt(mutable bool) (*LetStmt, error) {
	if mutable {
		if err := p.expectKeyword("var"); err != nil {
			return nil, err
		}
	} else {
		if err := p.expectKeyword("let"); err != nil {
			return nil, err
		}
	}
	name, err := p.expectIdent()
	if err != nil {
		return nil, err
	}
	var typ TypeExpr
	if p.matchSym(":") {
		typ, err = p.parseType()
		if err != nil {
			return nil, err
		}
	}
	if err := p.expectSym("="); err != nil {
		return nil, err
	}
	value, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	tok := p.peekRaw()
	return &LetStmt{Line: tok.line, Column: tok.col, Mutable: mutable, Name: name, Type: typ, Value: value}, nil
}

func (p *parser) peekRawN(n int) token {
	if p.pos+n >= len(p.toks) {
		return token{kind: tokEOF}
	}
	return p.toks[p.pos+n]
}
