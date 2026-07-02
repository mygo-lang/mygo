package parser

import (
	"fmt"

	"github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

type parser struct {
	toks   []token
	pos    int
	skipNL bool
}

func parseFile(src string) (*ast.File, error) {
	p := newParser(src)
	return p.parseFile()
}

func parseFiles(srcs map[string]string) ([]*ast.File, error) {
	files := make([]*ast.File, 0, len(srcs))
	for path, src := range srcs {
		f, err := parseFile(src)
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
	return &parser{toks: toks, skipNL: true}
}

func (p *parser) parseFile() (*ast.File, error) {
	file := &ast.File{}
	if p.peekKeyword("package") {
		tok := p.next()
		if tok.kind != tokKeyword || tok.lit != "package" {
			return nil, common.ErrorAtPos(tok.line, tok.col, "expected keyword %q, got %q", "package", tok.lit)
		}
		file.PackageLine = tok.line
		file.PackageColumn = tok.col
		name, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		file.PackageName = name
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
	case p.peekKeyword("let"):
		return p.parseBindingStmt(false)
	case p.peekKeyword("var"):
		return p.parseBindingStmt(true)
	default:
		tok := p.peek()
		return nil, common.ErrorAtPos(tok.line, tok.col, "unexpected token %q", tok.lit)
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
		return nil, common.ErrorAtPos(tok.line, tok.col, "expected import string, got %q", tok.lit)
	}
	return &ImportDecl{Line: tok.line, Column: tok.col, Alias: alias, Path: tok.lit}, nil
}

func (p *parser) parseEnum() (Decl, error) {
	start := p.peek()
	if err := p.expectKeyword("enum"); err != nil {
		return nil, err
	}
	name, typeParams, err := p.parseNameAndTypeParams()
	if err != nil {
		return nil, err
	}
	enum := &EnumDecl{Line: start.line, Column: start.col, Name: name, TypeParams: typeParams}
	for !p.peekKeyword("end") && !p.peekEOF() {
		p.skipNewlines()
		tok := p.peekRaw()
		if tok.kind == tokSym && tok.lit == "," {
			p.pos++
			p.skipNewlines()
			continue
		}
		variantName, err := p.expectIdent()
		if err != nil {
			return nil, err
		}
		variant := EnumVariant{Line: start.line, Column: start.col, Name: variantName}
		if p.matchSym("(") {
			if !p.matchSym(")") {
				for {
					var fieldName string
					if p.peek().kind == tokIdent && p.pos+1 < len(p.toks) {
						next := p.toks[p.pos+1]
						if next.kind == tokSym && next.lit == ":" {
							f, err := p.expectIdent()
							if err != nil {
								return nil, err
							}
							fieldName = f
							if err := p.expectSym(":"); err != nil {
								return nil, err
							}
						}
					}
					fieldType, err := p.parseType()
					if err != nil {
						return nil, err
					}
					variant.Fields = append(variant.Fields, Field{Line: tok.line, Column: tok.col, Name: fieldName, Type: fieldType})
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
	p.skipNewlines()
	if err := p.expectKeyword("end"); err != nil {
		return nil, err
	}
	return enum, nil
}

func (p *parser) parseStruct() (Decl, error) {
	start := p.peek()
	if err := p.expectKeyword("struct"); err != nil {
		return nil, err
	}
	name, typeParams, err := p.parseNameAndTypeParams()
	if err != nil {
		return nil, err
	}
	st := &StructDecl{Line: start.line, Column: start.col, Name: name, TypeParams: typeParams}
	if p.matchSym("(") {
		if !p.matchSym(")") {
			for i := 0; ; i++ {
				fieldType, err := p.parseType()
				if err != nil {
					return nil, err
				}
				st.Fields = append(st.Fields, Field{Line: start.line, Column: start.col, Name: fmt.Sprintf("F%d", i), Type: fieldType})
				if p.matchSym(")") {
					break
				}
				if err := p.expectSym(","); err != nil {
					return nil, err
				}
			}
		}
	} else {
		for !p.peekKeyword("end") && !p.peekEOF() {
			fieldName := ""
			if p.peekKeyword("embed") {
				_ = p.next()
				fieldName = "embed"
			} else {
				var err error
				fieldName, err = p.expectIdent()
				if err != nil {
					return nil, err
				}
				if err := p.expectSym(":"); err != nil {
					return nil, err
				}
			}
			fieldType, err := p.parseType()
			if err != nil {
				return nil, err
			}
			st.Fields = append(st.Fields, Field{Line: p.peekRaw().line, Column: p.peekRaw().col, Name: fieldName, Type: fieldType})
		}
	}
	if err := p.expectKeyword("end"); err != nil {
		return nil, err
	}
	return st, nil
}

func (p *parser) parseInterface() (Decl, error) {
	start := p.peek()
	if err := p.expectKeyword("interface"); err != nil {
		return nil, err
	}
	name, typeParams, err := p.parseNameAndTypeParams()
	if err != nil {
		return nil, err
	}
	it := &InterfaceDecl{Line: start.line, Column: start.col, Name: name, TypeParams: typeParams}
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
	start := p.peek()
	if err := p.expectKeyword("impl"); err != nil {
		return nil, err
	}
	name, typeArgs, err := p.parseNameAndTypeArgs()
	if err != nil {
		return nil, err
	}
	impl := &ImplDecl{Line: start.line, Column: start.col, Name: name, TypeArgs: typeArgs}
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
	start := p.peek()
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
	if err := p.expectSym("->"); err != nil {
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
			where = append(where, Constraint{Line: p.peekRaw().line, Column: p.peekRaw().col, Name: constraintName, Args: args})
			if !p.matchSym(",") {
				break
			}
		}
	}
	if allowEmpty && (p.peekKeyword("end") || p.peekKeyword("func") || p.peekKeyword("enum") || p.peekKeyword("struct") || p.peekKeyword("interface") || p.peekKeyword("impl") || p.peekKeyword("package") || p.peekKeyword("import")) {
		return &FuncDecl{Line: start.line, Column: start.col, Name: funcName, Params: params, Ret: ret, Where: where}, nil
	}
	body, err := p.parseExprUntilEnd()
	if err != nil {
		return nil, err
	}
	return &FuncDecl{Line: start.line, Column: start.col, Name: funcName, TypeParams: typeParams, Params: params, Ret: ret, Where: where, Body: body}, nil
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
		params = append(params, Param{Line: p.peekRaw().line, Column: p.peekRaw().col, Name: name, Type: tp})
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
	start := p.peek()
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
		var ret TypeExpr
		var err error
		if p.matchSym("->") {
			ret, err = p.parseType()
			if err != nil {
				return nil, err
			}
		} else {
			ret, err = p.parseType()
			if err != nil {
				return nil, err
			}
		}
		return &FuncType{Line: start.line, Column: start.col, Params: params, Ret: ret}, nil
	}
	if p.matchSym("(") {
		if p.matchSym(")") {
			return &NamedType{Line: start.line, Column: start.col, Name: "Unit"}, nil
		}
		inner, err := p.parseType()
		if err != nil {
			return nil, err
		}
		if err := p.expectSym(")"); err != nil {
			return nil, err
		}
		return inner, nil
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
	// Check for [] suffix (slice shorthand: Int[] → Slice[Int])
	if p.matchSym("[]") {
		base := &NamedType{Line: start.line, Column: start.col, Name: name, Args: args}
		return &NamedType{Line: start.line, Column: start.col, Name: "Slice", Args: []TypeExpr{base}}, nil
	}
	return &NamedType{Line: start.line, Column: start.col, Name: name, Args: args}, nil
}
