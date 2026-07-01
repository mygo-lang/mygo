package mygo

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	tokNumber
	tokString
	tokKeyword
	tokSym
)

type token struct {
	kind tokenKind
	lit  string
}

type lexer struct {
	src []rune
	pos int
}

func newLexer(src string) *lexer { return &lexer{src: []rune(src)} }

func (l *lexer) nextToken() token {
	l.skipSpaceAndComments()
	if l.pos >= len(l.src) {
		return token{kind: tokEOF}
	}

	ch := l.src[l.pos]
	switch {
	case isIdentStart(ch):
		start := l.pos
		l.pos++
		for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
			l.pos++
		}
		lit := string(l.src[start:l.pos])
		if isKeyword(lit) {
			return token{kind: tokKeyword, lit: lit}
		}
		return token{kind: tokIdent, lit: lit}
	case unicode.IsDigit(ch):
		start := l.pos
		l.pos++
		for l.pos < len(l.src) && unicode.IsDigit(l.src[l.pos]) {
			l.pos++
		}
		return token{kind: tokNumber, lit: string(l.src[start:l.pos])}
	case ch == '"':
		l.pos++
		var b strings.Builder
		for l.pos < len(l.src) {
			c := l.src[l.pos]
			l.pos++
			if c == '"' {
				break
			}
			if c == '\\' && l.pos < len(l.src) {
				next := l.src[l.pos]
				l.pos++
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
		return token{kind: tokString, lit: b.String()}
	default:
		if l.match("->") || l.match("|>") || l.match("==") || l.match("!=") {
			return token{kind: tokSym, lit: string(l.src[l.pos-2 : l.pos])}
		}
		l.pos++
		return token{kind: tokSym, lit: string(ch)}
	}
}

func (l *lexer) match(s string) bool {
	if l.pos+len([]rune(s)) > len(l.src) {
		return false
	}
	for i, r := range s {
		if l.src[l.pos+i] != r {
			return false
		}
	}
	l.pos += len([]rune(s))
	return true
}

func (l *lexer) skipSpaceAndComments() {
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if unicode.IsSpace(ch) {
			l.pos++
			continue
		}
		if ch == '#' {
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
			continue
		}
		break
	}
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func isKeyword(s string) bool {
	switch s {
	case "module", "import", "enum", "struct", "interface", "impl", "func", "switch", "case", "end", "where", "not":
		return true
	default:
		return false
	}
}

type parser struct {
	toks []token
	pos  int
}

func ParseFile(src string) (*File, error) {
	p := newParser(src)
	return p.parseFile()
}

func ParseFiles(srcs map[string]string) ([]*File, error) {
	files := make([]*File, 0, len(srcs))
	for path, src := range srcs {
		f, err := ParseFile(src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		files = append(files, f)
	}
	return files, nil
}

func newParser(src string) *parser {
	l := newLexer(src)
	var toks []token
	for {
		tok := l.nextToken()
		toks = append(toks, tok)
		if tok.kind == tokEOF {
			break
		}
	}
	return &parser{toks: toks}
}

func (p *parser) parseFile() (*File, error) {
	file := &File{}
	if p.peekKeyword("module") {
		if err := p.expectKeyword("module"); err != nil {
			return nil, err
		}
		name, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		file.Module = name
		if p.peekKeyword("end") {
			_ = p.next()
			return file, nil
		}
		for !p.peekKeyword("end") && !p.peekEOF() {
			decl, err := p.parseDecl()
			if err != nil {
				return nil, err
			}
			file.Decls = append(file.Decls, decl)
		}
		if err := p.expectKeyword("end"); err != nil {
			return nil, err
		}
		return file, nil
	}
	for !p.peekEOF() {
		decl, err := p.parseDecl()
		if err != nil {
			return nil, err
		}
		file.Decls = append(file.Decls, decl)
	}
	return file, nil
}

func (p *parser) parseDecl() (Decl, error) {
	switch {
	case p.peekKeyword("import"):
		return p.parseImport()
	case p.peekKeyword("enum"):
		return p.parseEnum()
	case p.peekKeyword("struct"):
		return p.parseStruct()
	case p.peekKeyword("interface"):
		return p.parseInterface()
	case p.peekKeyword("impl"):
		return p.parseImpl()
	case p.peekKeyword("func"):
		return p.parseFuncDecl(false)
	default:
		return nil, fmt.Errorf("unexpected token %q", p.peek().lit)
	}
}

func (p *parser) parseImport() (Decl, error) {
	if err := p.expectKeyword("import"); err != nil {
		return nil, err
	}
	alias := ""
	if p.peek().kind == tokIdent && p.pos+1 < len(p.toks) && p.toks[p.pos+1].kind == tokString {
		var err error
		alias, err = p.expectIdent()
		if err != nil {
			return nil, err
		}
	}
	tok := p.next()
	if tok.kind != tokString {
		return nil, fmt.Errorf("expected import string, got %q", tok.lit)
	}
	return &ImportDecl{Alias: alias, Path: tok.lit}, nil
}

func (p *parser) parseEnum() (Decl, error) {
	if err := p.expectKeyword("enum"); err != nil {
		return nil, err
	}
	name, typeParams, err := p.parseNameAndTypeParams()
	if err != nil {
		return nil, err
	}
	enum := &EnumDecl{Name: name, TypeParams: typeParams}
	for !p.peekKeyword("end") && !p.peekEOF() {
		variantName, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		variant := EnumVariant{Name: variantName}
		if p.matchSym("(") {
			if !p.matchSym(")") {
				for {
					fieldName := ""
					if p.peek().kind == tokIdent && p.pos+1 < len(p.toks) {
						next := p.toks[p.pos+1]
						if next.kind == tokSym && next.lit == ":" {
							fieldName, _ = p.expectIdent()
							if err := p.expectSym(":"); err != nil {
								return nil, err
							}
						}
					}
					fieldType, err := p.parseType()
					if err != nil {
						return nil, err
					}
					variant.Fields = append(variant.Fields, Field{Name: fieldName, Type: fieldType})
					if p.matchSym(")") {
						break
					}
					if err := p.expectSym(","); err != nil {
						return nil, err
					}
				}
			}
		}
		enum.Variants = append(enum.Variants, variant)
	}
	if err := p.expectKeyword("end"); err != nil {
		return nil, err
	}
	return enum, nil
}

func (p *parser) parseStruct() (Decl, error) {
	if err := p.expectKeyword("struct"); err != nil {
		return nil, err
	}
	name, typeParams, err := p.parseNameAndTypeParams()
	if err != nil {
		return nil, err
	}
	st := &StructDecl{Name: name, TypeParams: typeParams}
	for !p.peekKeyword("end") && !p.peekEOF() {
		fieldName, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		if err := p.expectSym(":"); err != nil {
			return nil, err
		}
		fieldType, err := p.parseType()
		if err != nil {
			return nil, err
		}
		st.Fields = append(st.Fields, Field{Name: fieldName, Type: fieldType})
	}
	if err := p.expectKeyword("end"); err != nil {
		return nil, err
	}
	return st, nil
}

func (p *parser) parseInterface() (Decl, error) {
	if err := p.expectKeyword("interface"); err != nil {
		return nil, err
	}
	name, typeParams, err := p.parseNameAndTypeParams()
	if err != nil {
		return nil, err
	}
	it := &InterfaceDecl{Name: name, TypeParams: typeParams}
	for !p.peekKeyword("end") && !p.peekEOF() {
		fd, err := p.parseFuncDecl(true)
		if err != nil {
			return nil, err
		}
		it.Methods = append(it.Methods, fd)
	}
	if err := p.expectKeyword("end"); err != nil {
		return nil, err
	}
	return it, nil
}

func (p *parser) parseImpl() (Decl, error) {
	if err := p.expectKeyword("impl"); err != nil {
		return nil, err
	}
	name, typeArgs, err := p.parseNameAndTypeArgs()
	if err != nil {
		return nil, err
	}
	impl := &ImplDecl{Name: name, TypeArgs: typeArgs}
	for !p.peekKeyword("end") && !p.peekEOF() {
		fd, err := p.parseFuncDecl(false)
		if err != nil {
			return nil, err
		}
		impl.Methods = append(impl.Methods, fd)
	}
	if err := p.expectKeyword("end"); err != nil {
		return nil, err
	}
	return impl, nil
}

func (p *parser) parseFuncDecl(allowEmpty bool) (*FuncDecl, error) {
	if err := p.expectKeyword("func"); err != nil {
		return nil, err
	}
	funcName, err := p.expectIdent()
	if err != nil {
		return nil, err
	}
	var typeParams []string
	if p.matchSym("[") {
		if !p.matchSym("]") {
			for {
				tp, err := p.expectIdent()
				if err != nil {
					return nil, err
				}
				typeParams = append(typeParams, tp)
				if p.matchSym("]") {
					break
				}
				if err := p.expectSym(","); err != nil {
					return nil, err
				}
			}
		}
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
	if err := p.expectSym(":"); err != nil {
		return nil, err
	}
	ret, err := p.parseType()
	if err != nil {
		return nil, err
	}
	var where []Constraint
	if p.peekKeyword("where") {
		_ = p.next()
		for {
			constraintName, args, err := p.parseConstraint()
			if err != nil {
				return nil, err
			}
			where = append(where, Constraint{Name: constraintName, Args: args})
			if !p.matchSym(",") {
				break
			}
		}
	}
	if allowEmpty && (p.peekKeyword("end") || p.peekKeyword("func") || p.peekKeyword("enum") || p.peekKeyword("struct") || p.peekKeyword("interface") || p.peekKeyword("impl") || p.peekKeyword("module") || p.peekKeyword("import")) {
		return &FuncDecl{Name: funcName, Params: params, Ret: ret, Where: where}, nil
	}
	body, err := p.parseExprUntilEnd()
	if err != nil {
		return nil, err
	}
	return &FuncDecl{Name: funcName, TypeParams: typeParams, Params: params, Ret: ret, Where: where, Body: body}, nil
}

func (p *parser) parseConstraint() (string, []TypeExpr, error) {
	name, err := p.expectIdent()
	if err != nil {
		return "", nil, err
	}
	var args []TypeExpr
	if p.matchSym("[") {
		if !p.matchSym("]") {
			for {
				tp, err := p.parseType()
				if err != nil {
					return "", nil, err
				}
				args = append(args, tp)
				if p.matchSym("]") {
					break
				}
				if err := p.expectSym(","); err != nil {
					return "", nil, err
				}
			}
		}
	}
	return name, args, nil
}

func (p *parser) parseParams() ([]Param, error) {
	var params []Param
	if p.peekSym(")") {
		return params, nil
	}
	for {
		name, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		if err := p.expectSym(":"); err != nil {
			return nil, err
		}
		tp, err := p.parseType()
		if err != nil {
			return nil, err
		}
		params = append(params, Param{Name: name, Type: tp})
		if !p.matchSym(",") {
			break
		}
	}
	return params, nil
}

func (p *parser) parseNameAndTypeParams() (string, []string, error) {
	name, err := p.expectIdent()
	if err != nil {
		return "", nil, err
	}
	var params []string
	if p.matchSym("[") {
		if !p.matchSym("]") {
			for {
				param, err := p.expectIdent()
				if err != nil {
					return "", nil, err
				}
				params = append(params, param)
				if p.matchSym("]") {
					break
				}
				if err := p.expectSym(","); err != nil {
					return "", nil, err
				}
			}
		}
	}
	return name, params, nil
}

func (p *parser) parseNameAndTypeArgs() (string, []TypeExpr, error) {
	name, err := p.expectIdent()
	if err != nil {
		return "", nil, err
	}
	var args []TypeExpr
	if p.matchSym("[") {
		if !p.matchSym("]") {
			for {
				tp, err := p.parseType()
				if err != nil {
					return "", nil, err
				}
				args = append(args, tp)
				if p.matchSym("]") {
					break
				}
				if err := p.expectSym(","); err != nil {
					return "", nil, err
				}
			}
		}
	}
	return name, args, nil
}

func (p *parser) parseType() (TypeExpr, error) {
	if p.peekKeyword("func") {
		_ = p.next()
		if err := p.expectSym("("); err != nil {
			return nil, err
		}
		var params []TypeExpr
		if !p.peekSym(")") {
			for {
				tp, err := p.parseType()
				if err != nil {
					return nil, err
				}
				params = append(params, tp)
				if !p.matchSym(",") {
					break
				}
			}
		}
		if err := p.expectSym(")"); err != nil {
			return nil, err
		}
		if err := p.expectSym(":"); err != nil {
			return nil, err
		}
		ret, err := p.parseType()
		if err != nil {
			return nil, err
		}
		return &FuncType{Params: params, Ret: ret}, nil
	}
	name, err := p.expectIdent()
	if err != nil {
		return nil, err
	}
	for p.matchSym(".") {
		part, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		name += "." + part
	}
	var args []TypeExpr
	if p.matchSym("[") {
		if !p.matchSym("]") {
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
		}
	}
	return &NamedType{Name: name, Args: args}, nil
}

func (p *parser) parseExprUntilEnd() (Expr, error) {
	if p.peekKeyword("switch") {
		expr, err := p.parseSwitchExpr()
		if err != nil {
			return nil, err
		}
		if err := p.expectKeyword("end"); err != nil {
			return nil, err
		}
		return expr, nil
	}
	if p.peekKeyword("func") {
		expr, err := p.parseFuncLit()
		if err != nil {
			return nil, err
		}
		if err := p.expectKeyword("end"); err != nil {
			return nil, err
		}
		return expr, nil
	}
	expr, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	if err := p.expectKeyword("end"); err != nil {
		return nil, err
	}
	return expr, nil
}

func (p *parser) parseFuncLit() (Expr, error) {
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
	if err := p.expectSym(":"); err != nil {
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
	return &FuncLitExpr{Params: params, Ret: ret, Body: body}, nil
}

func (p *parser) parseSwitchExpr() (Expr, error) {
	if err := p.expectKeyword("switch"); err != nil {
		return nil, err
	}
	target, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}
	var cases []SwitchCase
	for !p.peekKeyword("end") && !p.peekEOF() {
		if err := p.expectKeyword("case"); err != nil {
			return nil, err
		}
		pat, err := p.parsePattern()
		if err != nil {
			return nil, err
		}
		if err := p.expectSym("->"); err != nil {
			return nil, err
		}
		body, err := p.parseCaseBody()
		if err != nil {
			return nil, err
		}
		cases = append(cases, SwitchCase{Pattern: pat, Body: body})
	}
	if err := p.expectKeyword("end"); err != nil {
		return nil, err
	}
	return &SwitchExpr{Target: target, Cases: cases}, nil
}

func (p *parser) parseCaseBody() (Expr, error) {
	if p.peekKeyword("switch") {
		return p.parseSwitchExpr()
	}
	if p.peekKeyword("func") {
		return p.parseFuncLit()
	}
	return p.parseExpr(0)
}

func (p *parser) parsePattern() (Pattern, error) {
	name, err := p.expectIdent()
	if err != nil {
		return nil, err
	}
	if !p.matchSym("(") {
		if name == "_" {
			return &WildcardPattern{}, nil
		}
		return &VariantPattern{Name: name}, nil
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
	return &VariantPattern{Name: name, Args: args}, nil
}

const (
	precPipe = iota + 1
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
			left = &BinaryExpr{Op: "|>", Left: left, Right: right}
			continue
		}
		right, err := p.parseExpr(prec + 1)
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: op.lit, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parsePrefix() (Expr, error) {
	if p.peekKeyword("not") {
		_ = p.next()
		expr, err := p.parseExpr(precPostfix)
		if err != nil {
			return nil, err
		}
		return &PrefixExpr{Op: "not", Expr: expr}, nil
	}
	if p.peekKeyword("switch") {
		return p.parseSwitchExpr()
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
	for {
		switch {
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
			left = &CallExpr{Callee: left, Args: args}
		case p.matchSym("."):
			field, err := p.expectIdent()
			if err != nil {
				return nil, err
			}
			left = &FieldExpr{Expr: left, Field: field}
		default:
			return left, nil
		}
	}
}

func (p *parser) parsePrimary() (Expr, error) {
	tok := p.peek()
	switch tok.kind {
	case tokIdent:
		_ = p.next()
		return &IdentExpr{Name: tok.lit}, nil
	case tokNumber:
		_ = p.next()
		return &LiteralExpr{Kind: "number", Value: tok.lit}, nil
	case tokString:
		_ = p.next()
		return &LiteralExpr{Kind: "string", Value: tok.lit}, nil
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
	}
	return nil, fmt.Errorf("unexpected token %q", tok.lit)
}

func opPrecedence(tok token) (int, bool) {
	if tok.kind != tokSym {
		return 0, false
	}
	switch tok.lit {
	case "|>":
		return precPipe, true
	case "==", "!=":
		return precEq, true
	case "+":
		return precAdd, true
	case "*":
		return precMul, true
	default:
		return 0, false
	}
}

func (p *parser) peek() token {
	if p.pos >= len(p.toks) {
		return token{kind: tokEOF}
	}
	return p.toks[p.pos]
}

func (p *parser) next() token {
	tok := p.peek()
	if p.pos < len(p.toks) {
		p.pos++
	}
	return tok
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
		return fmt.Errorf("expected keyword %q, got %q", s, tok.lit)
	}
	return nil
}

func (p *parser) expectSym(s string) error {
	tok := p.next()
	if tok.kind != tokSym || tok.lit != s {
		return fmt.Errorf("expected %q, got %q", s, tok.lit)
	}
	return nil
}

func (p *parser) expectIdent() (string, error) {
	tok := p.next()
	if tok.kind != tokIdent {
		return "", fmt.Errorf("expected identifier, got %q", tok.lit)
	}
	return tok.lit, nil
}

func MustParseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
