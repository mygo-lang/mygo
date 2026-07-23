package codegen

import (
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

// translateBlockStmts translates statements within a BlockExpr.
func (g *gen) translateBlockStmts(n *BlockExpr, ctx *egCtx, returnExpected string, retTypes []string) ([]ast.Stmt, error) {
	child := ctx.child()
	var stmts []ast.Stmt
	for i, stmt := range n.Stmts {
		isLast := i == len(n.Stmts)-1
		switch s := stmt.(type) {
		case *ExprStmt:
			// A block used as a statement already has the correct lexical
			// representation.  Flatten it instead of turning it into an IIFE.
			if block, ok := s.Expr.(*BlockExpr); ok && !(isLast && returnExpected != "") {
				blockStmts, err := g.translateBlockStmts(block, child, "", nil)
				if err != nil {
					return stmts, err
				}
				stmts = append(stmts, blockStmts...)
				continue
			}
			if ifExpr, ok := s.Expr.(*IfExpr); ok && !(isLast && returnExpected != "") {
				ifStmt, err := g.translateIfStmt(ifExpr, child, returnExpected, retTypes)
				if err != nil {
					return stmts, err
				}
				stmts = append(stmts, ifStmt)
				continue
			}
			if whileExpr, ok := s.Expr.(*WhileExpr); ok && !(isLast && returnExpected != "") {
				forStmt, err := g.translateWhileFor(whileExpr, child)
				if err != nil {
					return stmts, err
				}
				stmts = append(stmts, forStmt)
				continue
			}
			if switchExpr, ok := s.Expr.(*SwitchExpr); ok && !(isLast && returnExpected != "") {
				// This switch is used as a statement. In particular, a unit-returning
				// function has an empty returnExpected after Go lowering, so pass an
				// explicit unit marker to prevent case result inference from creating
				// an unused expression value.
				result, err := g.translateSwitch(switchExpr, child, "Unit")
				if err != nil {
					return stmts, err
				}
				stmts = append(stmts, result.Stmts...)
				if !isNilASTExpr(result.Expr) {
					stmts = append(stmts, stmtForExpr(s.Expr, result.Expr, result.Type))
				}
				continue
			}
			if branch := branchStmtForExpr(s.Expr); branch != nil {
				stmts = append(stmts, branch)
				continue
			}
			if goExpr, ok := s.Expr.(*GoExpr); ok && g.isUnitGoExpr(goExpr, child) {
				goStmts, err := g.translateGoUnitStmts(goExpr, child)
				if err == nil {
					if isLast && returnExpected != "" {
						stmts = append(stmts, goStmts...)
						stmts = append(stmts, &ast.ReturnStmt{})
					} else {
						stmts = append(stmts, goStmts...)
					}
					continue
				}
			}
			expectedType := ""
			if isLast && returnExpected != "" {
				expectedType = returnExpected
			}
			translated, err := g.translateExprResult(s.Expr, child, expectedType)
			if err != nil {
				return stmts, err
			}
			stmts = append(stmts, translated.Stmts...)
			code, typ := translated.Expr, translated.Type
			if isNilASTExpr(code) && len(translated.Stmts) == 0 {
				line, col := common.NodePos(s.Expr)
				return stmts, common.ErrorAtPos(g.currentFile, line, col, "expression statement produced nil Go AST")
			}
			if code == nil {
				if isLast && isUnitGoType(returnExpected) {
					stmts = append(stmts, &ast.ReturnStmt{})
				}
				continue
			}
			if isLast && returnExpected != "" && !isUnitGoType(returnExpected) {
				stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{code}})
			} else if branch := branchStmtForExpr(s.Expr); branch != nil {
				stmts = append(stmts, branch)
			} else {
				stmts = append(stmts, stmtForExpr(s.Expr, code, typ))
				if isLast && isUnitGoType(returnExpected) {
					stmts = append(stmts, &ast.ReturnStmt{})
				}
			}
		case *ReturnStmt:
			if ifExpr, ok := s.Value.(*IfExpr); ok {
				ifStmt, err := g.translateIfStmt(ifExpr, child, returnExpected, retTypes)
				if err != nil {
					return stmts, err
				}
				stmts = append(stmts, ifStmt)
				continue
			}
			if switchExpr, ok := s.Value.(*SwitchExpr); ok {
				result, err := g.translateSwitch(switchExpr, child, returnExpected)
				if err != nil {
					return stmts, err
				}
				stmts = append(stmts, result.Stmts...)
				if result.Expr != nil {
					stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{result.Expr}})
				}
				continue
			}
			if blockExpr, ok := s.Value.(*BlockExpr); ok {
				code, _, err := g.translateExpr(blockExpr, child, returnExpected)
				if err != nil {
					return stmts, err
				}
				if iife, ok := code.(*ast.CallExpr); ok {
					if fn, ok := iife.Fun.(*ast.FuncLit); ok && fn.Body != nil {
						stmts = append(stmts, fn.Body.List...)
						continue
					}
				}
			}
			if s.Value != nil {
				translated, err := g.translateExprResult(s.Value, child, returnExpected)
				if err != nil {
					return stmts, err
				}
				stmts = append(stmts, translated.Stmts...)
				if translated.Expr != nil {
					stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{translated.Expr}})
				}
			} else {
				stmts = append(stmts, &ast.ReturnStmt{})
			}
		case *LetStmt:
			if s.Bind != nil {
				if bind, ok := s.Bind.(*BindTuplePattern); ok {
					code, valType, err := g.translateExpr(s.Value, child, "")
					if err != nil {
						return stmts, err
					}
					stmts = g.emitBindDestructure(stmts, child, code, valType, bind)
					continue
				}
			}
			expectedType := ""
			if s.Type != nil {
				expectedType = g.goType(s.Type, child.typeParams)
			} else if s.Value != nil {
				expectedType = g.inferredType(s.Value)
				if _, ok := s.Value.(*SwitchExpr); ok {
					expectedType = ""
				}
			}
			translated, err := g.translateExprResult(s.Value, child, expectedType)
			if err != nil {
				return stmts, err
			}
			stmts = append(stmts, translated.Stmts...)
			code, valType := translated.Expr, translated.Type
			if s.Name == "_" {
				// Use ExprStmt to discard the result — handles multi-return Go calls safely.
				stmts = append(stmts, &ast.ExprStmt{X: code})
			} else {
				g.localSeq++
				base := sanitizeIdent(s.Name)
				if base == "" || base == "_" {
					base = "tmp"
				}
				lbType := valType
				if isUnresolvedGoTypeParam(lbType) {
					if call, ok := s.Value.(*CallExpr); ok {
						if field, ok := call.Callee.(*FieldExpr); ok && field.Field == "Fold" && len(call.Args) > 0 {
							if typ := g.goTypeFromExpr(call.Args[0], child); typ != "" && typ != "any" {
								lbType = typ
							}
						}
					}
				}
				if lbType == "" && s.Type != nil {
					lbType = g.goType(s.Type, child.typeParams)
				}
				actual := base + "_" + strconv.Itoa(g.localSeq)
				child.bindings[s.Name] = actual
				child.locals[s.Name] = lbType
				child.mutable[actual] = s.Mutable
				// If/switch expressions used to hide their branch statements in an
				// IIFE.  Move those statements into the current scope and rename each
				// return target to the freshly allocated binding.
				if (isControlExpr(s.Value)) && isIIFEExpr(code) && lbType != "" {
					if fn := code.(*ast.CallExpr).Fun.(*ast.FuncLit); fn.Body != nil {
						stmts = append(stmts, &ast.DeclStmt{Decl: &ast.GenDecl{
							Tok:   token.VAR,
							Specs: []ast.Spec{&ast.ValueSpec{Names: []*ast.Ident{ast.NewIdent(actual)}, Type: g.goTypeExprFromString(lbType)}},
						}})
						stmts = append(stmts, renameIIFEReturns(fn.Body.List, actual)...)
						continue
					}
				}
				if s.Type != nil {
					typeExpr := goastTypeExpr(s.Type)
					stmts = append(stmts, &ast.DeclStmt{
						Decl: &ast.GenDecl{
							Tok: token.VAR,
							Specs: []ast.Spec{
								&ast.ValueSpec{
									Names:  []*ast.Ident{ast.NewIdent(actual)},
									Type:   typeExpr,
									Values: []ast.Expr{code},
								},
							},
						},
					})
				} else {
					stmts = append(stmts, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(actual)}, Rhs: []ast.Expr{code}, Tok: token.DEFINE})
				}
			}
		case *LetRecStmt:
			for _, b := range s.Bindings {
				g.localSeq++
				base := sanitizeIdent(b.Name)
				if base == "" || base == "_" {
					return nil, common.ErrorAtPos(g.currentFile, b.Line, b.Column, "invalid letrec binding name %q", b.Name)
				}
				actual := base + "_" + strconv.Itoa(g.localSeq)
				goType := g.goType(b.Type, child.typeParams)
				child.bindings[b.Name] = actual
				child.locals[b.Name] = goType
				child.mutable[actual] = false
				stmts = append(stmts, &ast.DeclStmt{
					Decl: &ast.GenDecl{
						Tok: token.VAR,
						Specs: []ast.Spec{
							&ast.ValueSpec{
								Names: []*ast.Ident{ast.NewIdent(actual)},
								Type:  goastTypeExpr(b.Type),
							},
						},
					},
				})
			}
			for _, b := range s.Bindings {
				actual := child.bindings[b.Name]
				expectedType := g.goType(b.Type, child.typeParams)
				code, _, err := g.translateExpr(b.Value, child, expectedType)
				if err != nil {
					return stmts, err
				}
				stmts = append(stmts, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(actual)}, Rhs: []ast.Expr{code}, Tok: token.ASSIGN})
			}
		case *AssignStmt:
			actual, ok := child.bindings[s.Name]
			if !ok {
				return nil, common.ErrorAtPos(g.currentFile, s.Line, s.Column, "unknown binding %q", s.Name)
			}
			if !child.mutable[actual] {
				return nil, common.ErrorAtPos(g.currentFile, s.Line, s.Column, "cannot assign to immutable binding %q", s.Name)
			}
			code, _, err := g.translateExpr(s.Value, child, child.locals[s.Name])
			if err != nil {
				return stmts, err
			}
			stmts = append(stmts, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(actual)}, Rhs: []ast.Expr{code}, Tok: token.ASSIGN})
		}
	}
	return stmts, nil
}

func isControlExpr(e Expr) bool {
	switch e.(type) {
	case *IfExpr, *SwitchExpr, *BlockExpr:
		return true
	default:
		return false
	}
}

func isIIFEExpr(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	_, ok = call.Fun.(*ast.FuncLit)
	return ok
}

func renameIIFEReturns(stmts []ast.Stmt, name string) []ast.Stmt {
	var out []ast.Stmt
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.ReturnStmt:
			if len(s.Results) == 1 {
				out = append(out, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(name)}, Rhs: s.Results, Tok: token.ASSIGN})
			}
		case *ast.IfStmt:
			s.Body.List = renameIIFEReturns(s.Body.List, name)
			if block, ok := s.Else.(*ast.BlockStmt); ok {
				block.List = renameIIFEReturns(block.List, name)
			}
			out = append(out, s)
		case *ast.BlockStmt:
			s.List = renameIIFEReturns(s.List, name)
			out = append(out, s)
		default:
			out = append(out, stmt)
		}
	}
	return out
}

func (g *gen) isUnitGoExpr(n *GoExpr, ctx *egCtx) bool {
	resultType := g.goType(n.Result, ctx.typeParams)
	return resultType == "" || resultType == "struct{}"
}

func branchStmtForExpr(e Expr) *ast.BranchStmt {
	id, ok := e.(*IdentExpr)
	if !ok {
		return nil
	}
	switch id.Name {
	case "break":
		return &ast.BranchStmt{Tok: token.BREAK}
	case "continue":
		return &ast.BranchStmt{Tok: token.CONTINUE}
	}
	return nil
}

func stmtForExpr(src Expr, code ast.Expr, typ string) ast.Stmt {
	if branch := branchStmtForExpr(src); branch != nil {
		return branch
	}
	if _, ok := src.(*UnitLitExpr); ok {
		return &ast.EmptyStmt{}
	}
	if typ != "" && !isUnitGoType(typ) && !exprCanBeStmt(src) {
		return &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent("_")}, Rhs: []ast.Expr{code}, Tok: token.ASSIGN}
	}
	return &ast.ExprStmt{X: code}
}

func isUnitGoType(typ string) bool {
	typ = strings.TrimSpace(typ)
	return typ == "Unit" || typ == "struct{}" || typ == "()"
}

func isNilASTExpr(expr ast.Expr) bool {
	if expr == nil {
		return true
	}
	v := reflect.ValueOf(expr)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func exprCanBeStmt(src Expr) bool {
	switch src.(type) {
	case *CallExpr, *GoExpr:
		return true
	default:
		return false
	}
}

// translateExpr is the main expression translator.
func (g *gen) translateExprResult(e Expr, ctx *egCtx, expected string) (translatedExpr, error) {
	if setLit, ok := e.(*SetLitExpr); ok {
		return g.translateSetLit(setLit, ctx, expected)
	}
	if switchExpr, ok := e.(*SwitchExpr); ok {
		if expected == "" && ctx != nil && isUnitGoType(ctx.retType) {
			expected = ctx.retType
		}
		if !isUnitGoType(expected) {
			for _, c := range switchExpr.Cases {
				if typ := switchBodyType(c.Body, g, ctx); typ != "" {
					if expected == "" || !sameSwitchResultType(expected, typ, g) {
						expected = typ
					}
					break
				}
			}
		}
		result, err := g.translateSwitch(switchExpr, ctx, expected)
		if err != nil {
			return translatedExpr{}, err
		}
		return result, nil
	}
	if whileExpr, ok := e.(*WhileExpr); ok {
		forStmt, err := g.translateWhileFor(whileExpr, ctx)
		if err != nil {
			return translatedExpr{}, err
		}
		return translatedExpr{Stmts: []ast.Stmt{forStmt}}, nil
	}
	if ifExpr, ok := e.(*IfExpr); ok && (expected == "" || isUnitGoType(expected)) {
		ifStmt, err := g.translateIfStmt(ifExpr, ctx, expected, nil)
		if err != nil {
			return translatedExpr{}, err
		}
		return translatedExpr{Stmts: []ast.Stmt{ifStmt}}, nil
	}
	if ifExpr, ok := e.(*IfExpr); ok && expected != "" && !isUnitGoType(expected) {
		cond, _, err := g.translateExpr(ifExpr.Cond, ctx, "bool")
		if err != nil {
			return translatedExpr{}, err
		}
		thenResult, err := g.translateExprResult(ifExpr.Then, ctx.child(), expected)
		if err != nil {
			return translatedExpr{}, err
		}
		elseResult, err := g.translateExprResult(ifExpr.Else, ctx.child(), expected)
		if err != nil {
			return translatedExpr{}, err
		}
		g.localSeq++
		tmp := "expr_" + strconv.Itoa(g.localSeq)
		thenStmts := append([]ast.Stmt{}, thenResult.Stmts...)
		thenStmts = append(thenStmts, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(tmp)}, Rhs: []ast.Expr{thenResult.Expr}, Tok: token.ASSIGN})
		elseStmts := append([]ast.Stmt{}, elseResult.Stmts...)
		elseStmts = append(elseStmts, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(tmp)}, Rhs: []ast.Expr{elseResult.Expr}, Tok: token.ASSIGN})
		return translatedExpr{
			Expr: ast.NewIdent(tmp),
			Type: expected,
			Stmts: []ast.Stmt{
				&ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
					Names: []*ast.Ident{ast.NewIdent(tmp)}, Type: g.goTypeExprFromString(expected),
				}}}},
				&ast.IfStmt{Cond: cond, Body: &ast.BlockStmt{List: thenStmts}, Else: &ast.BlockStmt{List: elseStmts}},
			},
		}, nil
	}
	if block, ok := e.(*BlockExpr); ok {
		if expected == "" {
			expected = g.inferredType(e)
		}
		stmts, err := g.translateBlockStmts(block, ctx, expected, nil)
		if err != nil {
			return translatedExpr{}, err
		}
		result := translatedExpr{Stmts: stmts, Type: expected}
		if expected == "" || isUnitGoType(expected) {
			return result, nil
		}
		g.localSeq++
		tmp := "expr_" + strconv.Itoa(g.localSeq)
		result.Stmts = renameIIFEReturns(result.Stmts, tmp)
		result.Stmts = append([]ast.Stmt{&ast.DeclStmt{Decl: &ast.GenDecl{
			Tok: token.VAR,
			Specs: []ast.Spec{&ast.ValueSpec{
				Names: []*ast.Ident{ast.NewIdent(tmp)},
				Type:  g.goTypeExprFromString(expected),
			}},
		}}}, result.Stmts...)
		result.Expr = ast.NewIdent(tmp)
		return result, nil
	}
	code, typ, err := g.translateExpr(e, ctx, expected)
	if err != nil {
		return translatedExpr{}, err
	}
	result := translatedExpr{Expr: code, Type: typ}
	call, ok := code.(*ast.CallExpr)
	if !ok {
		return result, nil
	}
	fn, ok := call.Fun.(*ast.FuncLit)
	if !ok || fn.Body == nil {
		return result, nil
	}
	// User/inline-Go function literals are real function values and may have
	// control-flow-sensitive returns. Only compiler-generated control IIFEs
	// are eligible for statement lifting.
	if !isControlExpr(e) {
		return result, nil
	}
	if typ == "" || isUnitGoType(typ) {
		result.Stmts = append(result.Stmts, fn.Body.List...)
		result.Expr = nil
		return result, nil
	}
	g.localSeq++
	tmp := "expr_" + strconv.Itoa(g.localSeq)
	result.Stmts = append(result.Stmts, &ast.DeclStmt{Decl: &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{&ast.ValueSpec{
			Names: []*ast.Ident{ast.NewIdent(tmp)},
			Type:  g.goTypeExprFromString(typ),
		}},
	}})
	result.Stmts = append(result.Stmts, renameIIFEReturns(fn.Body.List, tmp)...)
	result.Expr = ast.NewIdent(tmp)
	return result, nil
}

func (g *gen) translateExpr(e Expr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	switch n := e.(type) {
	case *IdentExpr:
		// Check bindings for renamed identifiers
		if bound, ok := ctx.bindings[n.Name]; ok {
			return ast.NewIdent(bound), ctx.locals[n.Name], nil
		}
		// Handle enum variant constructors with no args (e.g., bare `None` as IdentExpr)
		if n.Name == "None" {
			useExpected := expected
			if useExpected == "" {
				useExpected = ctx.retType
			}
			if base, tas := splitTypeArgs(useExpected); base == "Option" && len(tas) > 0 {
				callee := &ast.IndexExpr{X: ast.NewIdent("None"), Index: g.goTypeExprFromString(tas[0])}
				return &ast.CallExpr{Fun: callee}, useExpected, nil
			}
		}
		return ast.NewIdent(n.Name), ctx.locals[n.Name], nil
	case *LiteralExpr:
		switch n.Kind {
		case "number":
			info := ParseNumericLiteral(n.Value)
			switch info.Type {
			case "Float32":
				return ast.NewIdent(info.Value), "float32", nil
			case "Float64":
				return ast.NewIdent(info.Value), "float64", nil
			case "Int8":
				return ast.NewIdent(info.Value), "int8", nil
			case "Int16":
				return ast.NewIdent(info.Value), "int16", nil
			case "Int32":
				return ast.NewIdent(info.Value), "int32", nil
			case "Int64":
				return ast.NewIdent(info.Value), "int64", nil
			case "UInt":
				return ast.NewIdent(info.Value), "uint", nil
			case "UInt8":
				return ast.NewIdent(info.Value), "uint8", nil
			case "UInt16":
				return ast.NewIdent(info.Value), "uint16", nil
			case "UInt32":
				return ast.NewIdent(info.Value), "uint32", nil
			case "UInt64":
				return ast.NewIdent(info.Value), "uint64", nil
			default:
				return ast.NewIdent(info.Value), "int", nil
			}
		case "string":
			return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(n.Value)}, "string", nil
		case "rune":
			return &ast.BasicLit{Kind: token.CHAR, Value: strconv.QuoteRune([]rune(n.Value)[0])}, "rune", nil
		case "bool":
			if n.Value == "true" {
				return ast.NewIdent("true"), "bool", nil
			}
			return ast.NewIdent("false"), "bool", nil
		}
	case *BinaryExpr:
		left, lt, err := g.translateExpr(n.Left, ctx, "")
		if err != nil {
			return nil, "", err
		}
		if left == nil {
			line, col := common.NodePos(n.Left)
			return nil, "", common.ErrorAtPos(g.currentFile, line, col, "binary left operand produced nil Go AST")
		}
		right, rt, err := g.translateExpr(n.Right, ctx, lt)
		if err != nil {
			return nil, "", err
		}
		if right == nil {
			line, col := common.NodePos(n.Right)
			return nil, "", common.ErrorAtPos(g.currentFile, line, col, "binary right operand produced nil Go AST")
		}
		switch n.Op {
		case "+":
			return &ast.BinaryExpr{X: left, Op: token.ADD, Y: right}, chooseType(lt, rt), nil
		case "-":
			return &ast.BinaryExpr{X: left, Op: token.SUB, Y: right}, chooseType(lt, rt), nil
		case "*":
			return &ast.BinaryExpr{X: left, Op: token.MUL, Y: right}, chooseType(lt, rt), nil
		case "/":
			return &ast.BinaryExpr{X: left, Op: token.QUO, Y: right}, chooseType(lt, rt), nil
		case "&&":
			return &ast.BinaryExpr{X: left, Op: token.LAND, Y: right}, "bool", nil
		case "||":
			return &ast.BinaryExpr{X: left, Op: token.LOR, Y: right}, "bool", nil
		case "==", "!=", "<", ">", "<=", ">=":
			if err := g.ensureRelationAllowed(n, lt, rt); err != nil {
				return nil, "", err
			}
			tok := token.EQL
			switch n.Op {
			case "==":
				tok = token.EQL
			case "!=":
				tok = token.NEQ
			case "<":
				tok = token.LSS
			case ">":
				tok = token.GTR
			case "<=":
				tok = token.LEQ
			case ">=":
				tok = token.GEQ
			}
			return &ast.BinaryExpr{X: left, Op: tok, Y: right}, "bool", nil
		case "|>":
			if call, ok := n.Right.(*CallExpr); ok {
				callee, _, _ := g.translateExpr(call.Callee, ctx, "")
				args := make([]ast.Expr, 0, len(call.Args)+1)
				for _, a := range call.Args {
					ac, _, _ := g.translateExpr(a, ctx, "")
					args = append(args, ac)
				}
				args = append(args, left)
				return &ast.CallExpr{Fun: callee, Args: args}, lt, nil
			}
			return &ast.CallExpr{Fun: right, Args: []ast.Expr{left}}, lt, nil
		case "<|":
			if call, ok := n.Left.(*CallExpr); ok {
				callee, _, _ := g.translateExpr(call.Callee, ctx, "")
				args := make([]ast.Expr, 0, len(call.Args)+1)
				for _, a := range call.Args {
					ac, _, _ := g.translateExpr(a, ctx, "")
					args = append(args, ac)
				}
				args = append(args, right)
				return &ast.CallExpr{Fun: callee, Args: args}, lt, nil
			}
			return &ast.CallExpr{Fun: left, Args: []ast.Expr{right}}, lt, nil
		}
	case *PrefixExpr:
		expr, typ, err := g.translateExpr(n.Expr, ctx, "")
		if err != nil {
			return nil, "", err
		}
		if expr == nil {
			line, col := common.NodePos(n.Expr)
			return nil, "", common.ErrorAtPos(g.currentFile, line, col, "prefix operand produced nil Go AST")
		}
		switch n.Op {
		case "!":
			return &ast.UnaryExpr{Op: token.NOT, X: expr}, "bool", nil
		case "-":
			return &ast.UnaryExpr{Op: token.SUB, X: expr}, typ, nil
		}
	case *CastExpr:
		code, _, err := g.translateExpr(n.Expr, ctx, g.goType(n.Type, ctx.typeParams))
		if err != nil {
			return nil, "", err
		}
		if code == nil {
			line, col := common.NodePos(n.Expr)
			return nil, "", common.ErrorAtPos(g.currentFile, line, col, "cast operand produced nil Go AST")
		}
		target := g.goType(n.Type, ctx.typeParams)
		return &ast.CallExpr{Fun: ast.NewIdent(target), Args: []ast.Expr{code}}, target, nil
	case *FieldExpr:
		if alias, enumName, arity, ok := g.importedQualifiedEnumVariant(n); ok && arity == 0 {
			typ := g.inferredType(n)
			if typ == "" {
				typ = alias + "." + enumName
			}
			return &ast.CallExpr{Fun: ast.NewIdent(alias + "." + enumConstructorGoName(enumName, n.Field))}, typ, nil
		}
		if enumName, arity, ok := g.qualifiedEnumVariant(n); ok && arity == 0 {
			typ := g.inferredType(n)
			if typ == "" {
				typ = enumName
			}
			return &ast.CallExpr{Fun: ast.NewIdent(enumConstructorGoName(enumName, n.Field))}, typ, nil
		}
		if enumName := exprQualifiedName(n.Expr); enumName != "" {
			if dotIdx := strings.LastIndexByte(enumName, '.'); dotIdx > 0 {
				if baseName, _ := splitTypeArgs(g.inferredType(n)); baseName == enumName {
					alias := enumName[:dotIdx]
					localEnum := enumName[dotIdx+1:]
					ctor := enumConstructorGoName(localEnum, n.Field)
					return &ast.CallExpr{Fun: ast.NewIdent(alias + "." + ctor)}, enumName, nil
				}
			}
		}
		base, bt, err := g.translateExpr(n.Expr, ctx, "")
		if err != nil {
			return nil, "", err
		}
		if base == nil {
			line, col := common.NodePos(n.Expr)
			return nil, "", common.ErrorAtPos(g.currentFile, line, col, "field receiver produced nil Go AST")
		}
		// Handle Ref.value — dereference pointer
		if n.Field == "value" {
			btNorm := strings.TrimSpace(bt)
			if strings.HasPrefix(btNorm, "Ref[") && strings.HasSuffix(btNorm, "]") {
				inner := btNorm[4 : len(btNorm)-1]
				return &ast.UnaryExpr{Op: token.MUL, X: base}, inner, nil
			}
			if strings.HasPrefix(btNorm, "*") {
				return &ast.UnaryExpr{Op: token.MUL, X: base}, btNorm[1:], nil
			}
		}
		// Try to look up the field type from the base type
		ft := g.fieldType(bt, n.Field)
		if ft == "" {
			ft = lookupMyGoFieldType(n.Expr, n.Field, g)
		}
		if ft != "" {
			if inferred := g.inferredType(n); inferred != "" && !containsGeneratedTypeVar(inferred) && !g.containsUnresolvedTypeName(inferred) {
				ft = inferred
			}
			return &ast.SelectorExpr{X: base, Sel: ast.NewIdent(goastFieldName(n.Field))}, ft, nil
		}
		if inferred := g.inferredType(n); inferred != "" {
			return &ast.SelectorExpr{X: base, Sel: ast.NewIdent(goastFieldName(n.Field))}, inferred, nil
		}
		return &ast.SelectorExpr{X: base, Sel: ast.NewIdent(goastFieldName(n.Field))}, bt, nil
	case *CallExpr:
		code, typ, err := g.translateCall(n, ctx, expected)
		if inferred := g.inferredType(n); inferred != "" && (typ == "" || typ == "any" || containsGeneratedTypeVar(typ) || g.containsUnresolvedTypeName(typ)) {
			typ = inferred
		}
		return code, typ, err
	case *IfExpr:
		if expected == "" {
			expected = g.inferredType(n)
		}
		result, err := g.translateIf(n, ctx, expected)
		return result.Expr, result.Type, err
	case *SwitchExpr:
		if expected == "" {
			expected = g.inferredType(n)
		}
		result, err := g.translateSwitch(n, ctx, expected)
		if err != nil {
			return nil, "", err
		}
		return result.Expr, result.Type, nil
	case *WhileExpr:
		result, err := g.translateWhile(n, ctx)
		return result.Expr, result.Type, err
	case *BlockExpr:
		return nil, "", common.ErrorAtPos(g.currentFile, 0, 0, "block expression must be translated through translatedExpr")
	case *StructLitExpr:
		return g.translateStructLit(n, ctx, expected)
	case *SliceLitExpr:
		return g.translateSliceLit(n, ctx, expected)
	case *MapLitExpr:
		return g.translateMapLit(n, ctx, expected)
	case *SetLitExpr:
		result, err := g.translateSetLit(n, ctx, expected)
		return result.Expr, result.Type, err
	case *TupleLitExpr:
		return g.translateTupleLit(n, ctx, expected)
	case *UnitLitExpr:
		return &ast.CompositeLit{Type: &ast.StructType{Fields: &ast.FieldList{}}}, "Unit", nil
	case *FuncLitExpr:
		return g.translateFuncLit(n, ctx)
	case *GoExpr:
		return g.translateGoExpr(n, ctx, expected)
	}
	line, col := common.NodePos(e)
	return nil, "", common.ErrorAtPos(g.currentFile, line, col, "unsupported expression %T", e)
}

func (g *gen) fieldListForReturn(expected string) *ast.FieldList {
	if expected == "" || isUnitGoType(expected) {
		return nil
	}
	return &ast.FieldList{List: []*ast.Field{{Type: g.goTypeExprFromString(expected)}}}
}

func fieldListIfNonEmptyGoast(fields []*ast.Field) *ast.FieldList {
	if len(fields) == 0 {
		return nil
	}
	return &ast.FieldList{List: fields}
}
