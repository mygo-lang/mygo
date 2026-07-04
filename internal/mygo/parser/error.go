package parser

import __yyfmt__ "fmt"

func (p *parser) Error(s string) {
	if p.err == nil {
		tok := p.peek()
		if tok.line != 0 || tok.col != 0 {
			p.err = __yyfmt__.Errorf("line %d, col %d: %s (near %q)", tok.line, tok.col, s, tok.lit)
		} else {
			p.err = __yyfmt__.Errorf("%s", s)
		}
	}
}
