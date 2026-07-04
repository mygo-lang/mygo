package parser

import (
	"fmt"

	"github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

type parser struct {
	toks                      []token
	pos                       int
	skipNL                    bool
	err                       error
	result                    *ast.File
	packageName               string
	packageLine               int
	packageColumn             int
	decls                     []Decl
	currentName               string
	currentNameLine           int
	currentNameCol            int
	currentType               TypeExpr
	currentTypeLine           int
	currentTypeCol            int
	currentTypeParams         []string
	currentParams             []ast.Param
	currentWhere              []ast.Constraint
	currentConstraintArgs     []TypeExpr
	currentBlock              []ast.Stmt
	currentBlockStack         [][]ast.Stmt
	currentStmt               ast.Stmt
	currentExpr               ast.Expr
	currentLeftExpr           ast.Expr
	currentPipeLeftExpr       ast.Expr
	currentArgs               []ast.Expr
	currentCallCalleeStack    []ast.Expr
	currentArgsStack          [][]ast.Expr
	currentSliceElemsStack    [][]ast.Expr
	currentMapKey             ast.Expr
	currentMapValue           ast.Expr
	currentMapEntries         []ast.MapLitPair
	currentSetElems           []ast.Expr
	currentEnumFields         []ast.Field
	currentCollectionHasPair  bool
	currentIfCond             ast.Expr
	currentIfThen             ast.Expr
	currentIfElse             ast.Expr
	currentWhileCond          ast.Expr
	currentWhileBody          ast.Expr
	currentSwitchTarget       ast.Expr
	currentSwitchCases        []ast.SwitchCase
	currentPattern            ast.Pattern
	currentPatternArgs        []string
	currentStructFields       []ast.StructLitField
	currentStructTypeArgs     []ast.TypeExpr
	currentTypeArgStack       [][]ast.TypeExpr
	currentFuncTypeParamStack [][]ast.TypeExpr
	funcTypeParamDepth        int
	currentImplTypeParams     []string
	currentImplType           TypeExpr
	currentImplInterfaceArgs  []ast.TypeExpr
	currentImplLine           int
	currentImplCol            int
	currentSliceElems         []ast.Expr
	currentConstraintBindName string
	savedDeclName             string
	savedTypeNameStack        []typeNameEntry
	savedStructTypeArgs       []ast.TypeExpr
	expectTypeSuffix          bool
	expectStructTypeArgs      bool
	expectConstraintSuffix    bool
	parsingImplTypeParams     bool
	currentEnum               *ast.EnumDecl
	currentStruct             *ast.StructDecl
	currentInterface          *ast.InterfaceDecl
	currentImpl               *ast.ImplDecl
	currentFunc               *ast.FuncDecl
}

type typeNameEntry struct {
	name string
	line int
	col  int
	args []TypeExpr
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
	// Check for impl-level type params: impl[T] or impl[T, U]
	var implTypeParams []string
	if p.peekSym("[") {
		_ = p.next() // consume "["
		if !p.peekSym("]") {
			for {
				tp, err := p.expectIdent()
				if err != nil {
					return nil, err
				}
				implTypeParams = append(implTypeParams, tp)
				if p.matchSym("]") {
					break
				}
				if err := p.expectSym(","); err != nil {
					return nil, err
				}
			}
		}
	}
	// Parse the first expression: could be Type : Interface (named/generic impl)
	// or just Interface[Args] (anonymous impl like "impl Show[String]").
	typeExpr, err := p.parseType()
	if err != nil {
		return nil, common.ErrorAtPos(start.line, start.col, "expected type after impl")
	}
	// Peek ahead for ":" to distinguish between named and anonymous forms.
	if p.peekSym(":") {
		// Named/generic form: "impl Type : Interface[Args]"
		_ = p.next() // consume ":"
		ifaceName, ifaceArgs, err := p.parseNameAndTypeArgs()
		if err != nil {
			return nil, err
		}
		impl := &ImplDecl{
			Line:          start.line,
			Column:        start.col,
			InterfaceName: ifaceName,
			InterfaceArgs: ifaceArgs,
			Type:          typeExpr,
			TypeParams:    implTypeParams,
		}
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
	// Anonymous form: "impl Interface[Args]" — the parsed type is the interface reference.
	// Extract interface name and args from the NamedType.
	iface, ok := typeExpr.(*NamedType)
	if !ok {
		return nil, common.ErrorAtPos(start.line, start.col, "expected interface name after impl; use 'impl Interface[Args]' or 'impl Type: Interface[Args]'")
	}
	impl := &ImplDecl{
		Line:          start.line,
		Column:        start.col,
		InterfaceName: iface.Name,
		InterfaceArgs: iface.Args,
		Type:          nil,
		TypeParams:    implTypeParams,
	}
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
	// Reject legacy "where" keyword (tokenized as ident) with a migration hint.
	if p.peek().kind == tokIdent && p.peek().lit == "where" {
		_ = p.next()
		return nil, common.ErrorAtPos(start.line, start.col, "'where' is removed; use 'using ConstraintName' instead")
	}
	var where []Constraint
	if p.peekKeyword("using") {
		_ = p.next()
		for {
			c, err := p.parseConstraint()
			if err != nil {
				return nil, err
			}
			where = append(where, c)
			if !p.matchSym(",") {
				break
			}
		}
	}
	if allowEmpty && (p.peekKeyword("end") || p.peekKeyword("func") || p.peekKeyword("enum") || p.peekKeyword("struct") || p.peekKeyword("interface") || p.peekKeyword("impl") || p.peekKeyword("package") || p.peekKeyword("import")) {
		return &FuncDecl{Line: start.line, Column: start.col, Name: funcName, TypeParams: typeParams, Params: params, Ret: ret, Using: where}, nil
	}
	body, err := p.parseExprUntilEnd()
	if err != nil {
		return nil, err
	}
	return &FuncDecl{Line: start.line, Column: start.col, Name: funcName, TypeParams: typeParams, Params: params, Ret: ret, Using: where, Body: body}, nil
}

// parseConstraint parses a `using` constraint entry.
// Supports two forms:
//  1. Named:  `intShow: Show[Int]`     — BindName = "intShow", Name = "Show", Args = [Int]
//  2. Simple: `Show[Int]`              — BindName = "",       Name = "Show", Args = [Int]
func (p *parser) parseConstraint() (Constraint, error) {
	// Peek: is the next token an ident followed by ":" and another ident?
	// If so, treat it as a named binding: `name: Interface[..]`
	line := p.peek().line
	col := p.peek().col
	if p.peek().kind == tokIdent && p.pos+2 < len(p.toks) {
		next1 := p.toks[p.pos+1]
		next2 := p.toks[p.pos+2]
		if next1.kind == tokSym && next1.lit == ":" && next2.kind == tokIdent {
			// Named form: bindName : InterfaceName[Args]
			bindName, err := p.expectIdent()
			if err != nil {
				return Constraint{}, err
			}
			if err := p.expectSym(":"); err != nil {
				return Constraint{}, err
			}
			ifaceName, err := p.expectIdent()
			if err != nil {
				return Constraint{}, err
			}
			var args []TypeExpr
			if p.matchSym("[") {
				if !p.matchSym("]") {
					for {
						tp, err := p.parseType()
						if err != nil {
							return Constraint{}, err
						}
						args = append(args, tp)
						if p.matchSym("]") {
							break
						}
						if err := p.expectSym(","); err != nil {
							return Constraint{}, err
						}
					}
				}
			}
			return Constraint{Line: line, Column: col, Name: ifaceName, Args: args, BindName: bindName}, nil
		}
	}
	// Simple form: InterfaceName[Args]
	ifaceName, err := p.expectIdent()
	if err != nil {
		return Constraint{}, err
	}
	var args []TypeExpr
	if p.matchSym("[") {
		if !p.matchSym("]") {
			for {
				tp, err := p.parseType()
				if err != nil {
					return Constraint{}, err
				}
				args = append(args, tp)
				if p.matchSym("]") {
					break
				}
				if err := p.expectSym(","); err != nil {
					return Constraint{}, err
				}
			}
		}
	}
	return Constraint{Line: line, Column: col, Name: ifaceName, Args: args}, nil
}

// tryIdent peeks if the current token is an identifier and consumes it;
// returns (name, true) on match, ("", false) otherwise.
func (p *parser) tryIdent() (string, bool) {
	if p.peek().kind == tokIdent {
		return p.next().lit, true
	}
	return "", false
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
				// Interface type params can be simple names (A) or
				// higher-kinded types (C[A]).  Parse a full type expression
				// and use its root name as the parameter name.
				typ, err := p.parseType()
				if err != nil {
					return "", nil, err
				}
				var paramName string
				if nt, ok := typ.(*NamedType); ok {
					paramName = nt.Name
				} else {
					return "", nil, common.ErrorAtPos(p.peek().line, p.peek().col, "expected type parameter name")
				}
				params = append(params, paramName)
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
				// Function params in type annotations may have names like "item: A"
				// or be bare types like "A".  Try to consume an optional ident + ":".
				if ident, ok := p.tryIdent(); ok {
					if p.matchSym(":") {
						// Named param: skip the name, parse the actual type
						_ = ident
					}
				}
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
	for {
		if p.matchSym("[") {
			var nextArgs []TypeExpr
			for {
				tp, err := p.parseType()
				if err != nil {
					return nil, err
				}
				nextArgs = append(nextArgs, tp)
				if p.matchSym("]") {
					break
				}
				if err := p.expectSym(","); err != nil {
					return nil, err
				}
			}
			args = nextArgs
			continue
		}
		break
	}
	return &NamedType{Line: start.line, Column: start.col, Name: name, Args: args}, nil
}
