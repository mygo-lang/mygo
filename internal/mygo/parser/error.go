package parser

import __yyfmt__ "fmt"

func (p *parser) Error(s string) {
	if p.err == nil {
		p.err = __yyfmt__.Errorf("%s", s)
	}
}
