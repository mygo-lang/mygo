package parser

import (
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func (p *parser) parseExprUntilEnd() (Expr, error) {
	expr, err := p.parseExprListUntil("end")
	if err != nil {
		return nil, err
	}
	if err := p.expectKeyword("end"); err != nil {
		return nil, err
	}
	return expr, nil
}

func (p *parser) parseFuncLit() (Expr, error) {
	start := p.peek()
	if err := p.expectKeyword("func"); err != nil {
		return nil, err
	}
	if err := p.expectSym("("); err != nil {
		return nil, err
	}
	params, err := p.parseParams()
	if err != nil {
		return nil, err
	}
	if err := p.expectSym(")"); err != nil {
		return nil, err
	}
	if err := p.expectSym("->"); err != nil {
		return nil, err
	}
	ret, err := p.parseType()
	if err != nil {
		return nil, err
	}
	body, err := p.parseExprUntilEnd()
	if err != nil {
		return nil, err
	}
	return &FuncLitExpr{Line: start.line, Column: start.col, Params: params, Ret: ret, Body: body}, nil
}

func (p *parser) parseSwitchExpr() (Expr, error) {
	start := p.peek()
	if err := p.expectKeyword("switch"); err != nil {
		return nil, err
	}
	target, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	var cases []SwitchCase
	for {
		p.skipNewlines()
		tok := p.peekRaw()
		if tok.kind == tokEOF {
			break
		}
		if tok.kind == tokKeyword && tok.lit == "end" {
			break
		}
		if err := p.expectKeyword("case"); err != nil {
			return nil, err
		}
		pat, err := p.parsePattern()
		if err != nil {
			return nil, err
		}
		if err := p.expectSym("=>"); err != nil {
			return nil, err
		}
		body, err := p.parseCaseBody()
		if err != nil {
			return nil, err
		}
		line, col := common.NodePos(pat)
		cases = append(cases, SwitchCase{Line: line, Column: col, Pattern: pat, Body: body})
	}
	if err := p.expectKeyword("end"); err != nil {
		return nil, err
	}
	return &SwitchExpr{Line: start.line, Column: start.col, Target: target, Cases: cases}, nil
}

func (p *parser) parseWhileExpr() (Expr, error) {
	start := p.peek()
	if err := p.expectKeyword("while"); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if p.peekRaw().kind != tokNewline {
		tok := p.peekRaw()
		return nil, common.ErrorAtPos(tok.line, tok.col, "expected newline after while condition")
	}
	body, err := p.parseExprUntilEnd()
	if err != nil {
		return nil, err
	}
	return &WhileExpr{Line: start.line, Column: start.col, Cond: cond, Body: body}, nil
}

func (p *parser) parseIfExpr() (Expr, error) {
	start := p.peek()
	if err := p.expectKeyword("if"); err != nil {
		return nil, err
	}
	cond, err := p.parseExprListUntil("then")
	if err != nil {
		return nil, err
	}
	if err := p.expectKeyword("then"); err != nil {
		return nil, err
	}
	thenExpr, err := p.parseExprListUntil("else")
	if err != nil {
		return nil, err
	}
	if err := p.expectKeyword("else"); err != nil {
		return nil, err
	}
	elseExpr, err := p.parseExprListUntil("end")
	if err != nil {
		return nil, err
	}
	return &IfExpr{Line: start.line, Column: start.col, Cond: cond, Then: thenExpr, Else: elseExpr}, nil
}

func (p *parser) parseCaseBody() (Expr, error) {
	return p.parseExprListUntil("case", "end")
}

func (p *parser) parsePattern() (Pattern, error) {
	start := p.peek()
	name, err := p.expectIdent()
	if err != nil {
		return nil, err
	}
	if !p.matchSym("(") {
		if name == "_" {
			return &WildcardPattern{Line: start.line, Column: start.col}, nil
		}
		return &VariantPattern{Line: start.line, Column: start.col, Name: name}, nil
	}
	var args []string
	if !p.matchSym(")") {
		for {
			arg, err := p.expectIdent()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if p.matchSym(")") {
				break
			}
			if err := p.expectSym(","); err != nil {
				return nil, err
			}
		}
	}
	return &VariantPattern{Line: start.line, Column: start.col, Name: name, Args: args}, nil
}

const (
	precOr = iota + 1
	precAnd
	precPipe
	precEq
	precAdd
	precMul
	precPostfix
)

func (p *parser) parseExpr(minPrec int) (Expr, error) {
	left, err := p.parsePrefix()
	if err != nil {
		return nil, err
	}
	for {
		op := p.peek()
		prec, ok := opPrecedence(op)
		if !ok || prec < minPrec {
			break
		}
		_ = p.next()
		if op.lit == "|>" {
			right, err := p.parseExpr(prec + 1)
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Line: op.line, Column: op.col, Op: "|>", Left: left, Right: right}
			continue
		}
		right, err := p.parseExpr(prec + 1)
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Line: op.line, Column: op.col, Op: op.lit, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parsePrefix() (Expr, error) {
	if p.peekKeyword("not") {
		tok := p.next()
		expr, err := p.parseExpr(precPostfix)
		if err != nil {
			return nil, err
		}
		return &PrefixExpr{Line: tok.line, Column: tok.col, Op: "not", Expr: expr}, nil
	}
	if p.peekKeyword("if") {
		return p.parseIfExpr()
	}
	if p.peekKeyword("switch") {
		return p.parseSwitchExpr()
	}
	if p.peekKeyword("while") {
		return p.parseWhileExpr()
	}
	if p.peekKeyword("func") {
		return p.parseFuncLit()
	}
	return p.parsePostfix()
}

func (p *parser) parsePostfix() (Expr, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	line, col := common.NodePos(left)
	if line == 0 {
		tok := p.peekRaw()
		line, col = tok.line, tok.col
	}
	structName := ""
	var structTypeArgs []TypeExpr
	if id, ok := left.(*IdentExpr); ok {
		structName = id.Name
	}
	for {
		switch {
		case structName != "" && p.peekSym("["):
			typeArgs, err := p.parseTypeArgs()
			if err != nil {
				return nil, err
			}
			structTypeArgs = typeArgs
		case p.matchSym("("):
			var args []Expr
			if !p.matchSym(")") {
				for {
					arg, err := p.parseExpr(0)
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					if p.matchSym(")") {
						break
					}
					if err := p.expectSym(","); err != nil {
						return nil, err
					}
				}
			}
			left = &CallExpr{Line: line, Column: col, Callee: left, Args: args}
			structName = ""
			structTypeArgs = nil
		case p.matchSym("{"):
			if structName == "" {
				return nil, common.ErrorAtPos(line, 0, "struct literal must start with a type name")
			}
			fields, err := p.parseStructLitFields()
			if err != nil {
				return nil, err
			}
			left = &StructLitExpr{Line: line, Column: col, TypeName: structName, TypeArgs: structTypeArgs, Fields: fields}
			structName = ""
			structTypeArgs = nil
		case p.matchSym("."):
			field, err := p.expectIdent()
			if err != nil {
				return nil, err
			}
			left = &FieldExpr{Line: line, Column: col, Expr: left, Field: field}
			structName = ""
			structTypeArgs = nil
		default:
			return left, nil
		}
	}
}

func (p *parser) parseTypeArgs() ([]TypeExpr, error) {
	if err := p.expectSym("["); err != nil {
		return nil, err
	}
	var args []TypeExpr
	if p.matchSym("]") {
		return args, nil
	}
	for {
		tp, err := p.parseType()
		if err != nil {
			return nil, err
		}
		args = append(args, tp)
		if p.matchSym("]") {
			break
		}
		if err := p.expectSym(","); err != nil {
			return nil, err
		}
	}
	return args, nil
}

func (p *parser) parseSliceLit() (Expr, error) {
	start := p.peek()
	_ = p.next() // consume '['

	var elems []Expr
	if !p.matchSym("]") {
		for {
			elem, err := p.parseExpr(0)
			if err != nil {
				return nil, err
			}
			elems = append(elems, elem)
			if p.matchSym("]") {
				break
			}
			if err := p.expectSym(","); err != nil {
				return nil, err
			}
		}
	}
	return &SliceLitExpr{Line: start.line, Column: start.col, Elem: nil, Elems: elems}, nil
}

// parseCollectionLit handles both Map and Set literals starting with '{'.
// Heuristic: if every entry is "key : value" we produce a MapLitExpr,
// otherwise a SetLitExpr. Mixed entries are an error.
func (p *parser) parseCollectionLit() (Expr, error) {
	start := p.peek()
	_ = p.next() // consume '{'

	var pairs []MapLitPair
	var setElems []Expr
	isMap := false

	if !p.matchSym("}") {
		for {
			p.skipNewlines()
			keyStart := p.peek()

			var keyTok token
			switch keyTok.kind = peekTokenKindForLit(p); keyTok.kind {
			case tokNumber, tokString, tokIdent, tokSym:
				keyTok = p.next()
			default:
				return nil, common.ErrorAtPos(keyStart.line, keyStart.col, "expected element or key-value pair")
			}

			// Check for "key : value" pattern
			if p.peekRaw().kind == tokSym && p.peekRaw().lit == ":" {
				isMap = true
				_ = p.next() // consume ':'
				value, err := p.parseExpr(0)
				if err != nil {
					return nil, err
				}
				line, col := common.NodePos(keyTok)
				pairs = append(pairs, MapLitPair{
					Line:  line,
					Col:   col,
					Key:   p.tokenToExpr(keyTok),
					Value: value,
				})
			} else {
				elem := p.tokenToExpr(keyTok)
				setElems = append(setElems, elem)
			}

			if p.matchSym("}") {
				break
			}
			if err := p.expectSym(","); err != nil {
				return nil, err
			}
		}
	}

	if isMap {
		return &MapLitExpr{Line: start.line, Column: start.col, Pairs: pairs}, nil
	}
	return &SetLitExpr{Line: start.line, Col: start.col, Elems: setElems}, nil
}

func peekTokenKindForLit(p *parser) tokenKind {
	t := p.peekRaw()
	return t.kind
}

func (p *parser) tokenToExpr(t token) Expr {
	switch t.kind {
	case tokNumber:
		return &LiteralExpr{Line: t.line, Column: t.col, Kind: "number", Value: t.lit}
	case tokString:
		return &LiteralExpr{Line: t.line, Column: t.col, Kind: "string", Value: t.lit}
	case tokIdent:
		return &IdentExpr{Line: t.line, Column: t.col, Name: t.lit}
	default:
		return nil
	}
}

func (p *parser) parseStructLitFields() ([]StructLitField, error) {
	var fields []StructLitField
	for {
		p.skipNewlines()
		tok := p.peekRaw()
		if tok.kind == tokEOF {
			break
		}
		if tok.kind == tokSym && tok.lit == "}" {
			p.pos++
			break
		}
		if len(fields) > 0 && tok.kind == tokSym && tok.lit == "," {
			p.pos++
			p.skipNewlines()
			tok = p.peekRaw()
			if tok.kind == tokSym && tok.lit == "}" {
				p.pos++
				break
			}
		}
		nameTok := p.peekRaw()
		name := ""
		var err error
		if p.peekKeyword("embed") {
			_ = p.next()
			name = "embed"
		} else {
			name, err = p.expectIdent()
			if err != nil {
				return nil, err
			}
		}
		if err := p.expectSym(":"); err != nil {
			return nil, err
		}
		value, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}
		fields = append(fields, StructLitField{Line: nameTok.line, Column: nameTok.col, Name: name, Value: value})
	}
	return fields, nil
}

func (p *parser) parsePrimary() (Expr, error) {
	tok := p.peek()
	switch tok.kind {
	case tokIdent:
		_ = p.next()
		return &IdentExpr{Line: tok.line, Column: tok.col, Name: tok.lit}, nil
	case tokNumber:
		_ = p.next()
		return &LiteralExpr{Line: tok.line, Column: tok.col, Kind: "number", Value: tok.lit}, nil
	case tokString:
		_ = p.next()
		return &LiteralExpr{Line: tok.line, Column: tok.col, Kind: "string", Value: tok.lit}, nil
	case tokSym:
		if tok.lit == "(" {
			_ = p.next()
			expr, err := p.parseExpr(0)
			if err != nil {
				return nil, err
			}
			if err := p.expectSym(")"); err != nil {
				return nil, err
			}
			return expr, nil
		}
		if tok.lit == "[" {
			return p.parseSliceLit()
		}
		if tok.lit == "{" {
			return p.parseCollectionLit()
		}
	}
	return nil, common.ErrorAtPos(tok.line, tok.col, "unexpected token %q", tok.lit)
}

func opPrecedence(tok token) (int, bool) {
	if tok.kind != tokSym {
		return 0, false
	}
	switch tok.lit {
	case "||":
		return precOr, true
	case "&&":
		return precAnd, true
	case "<|":
		return precPipe, true
	case "|>":
		return precPipe, true
	case "==", "!=":
		return precEq, true
	case "<", ">", "<=", ">=":
		return precEq, true
	case "+":
		return precAdd, true
	case "-":
		return precAdd, true
	case "*":
		return precMul, true
	case "/":
		return precMul, true
	default:
		return 0, false
	}
}
