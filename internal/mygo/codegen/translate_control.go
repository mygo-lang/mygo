package codegen

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

// translateIf handles if expressions.
func (g *gen) translateIf(n *IfExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	cond, _, _ := g.translateExpr(n.Cond, ctx, "bool")
	thenCtx := ctx.child()
	elseCtx := ctx.child()
	thenCode, thenType, _ := g.translateExpr(n.Then, thenCtx, expected)
	elseCode, elseType, _ := g.translateExpr(n.Else, elseCtx, expected)

	resultType := expected
	if resultType == "" {
		if thenType != "" {
			resultType = thenType
		} else {
			resultType = elseType
		}
	}
	if resultType == "" || resultType == "any" {
		// Statement form: wrap in IIFE so both branches are expressions
		ifStmt := &ast.IfStmt{
			Cond: cond,
			Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: thenCode}}},
		}
		if elseCode != nil {
			ifStmt.Else = &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: elseCode}}}
		}
		fn := astFuncLit(nil, nil, &ast.BlockStmt{List: []ast.Stmt{ifStmt}})
		return &ast.CallExpr{Fun: fn}, "", nil
	}
	// Expression form: wrap in IIFE returning resultType
	fn := &ast.FuncLit{
		Type: &ast.FuncType{
			Params:  &ast.FieldList{},
			Results: &ast.FieldList{List: []*ast.Field{{Type: ast.NewIdent(resultType)}}},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.IfStmt{
					Cond: cond,
					Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{thenCode}}}},
					Else: &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{elseCode}}}},
				},
			},
		},
	}
	return &ast.CallExpr{Fun: fn}, resultType, nil
}

// translateSwitch handles switch expressions.
func (g *gen) translateSwitch(n *SwitchExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	target, ttype, _ := g.translateExpr(n.Target, ctx, "")
	_, _ = target, ttype

	lastIsWildcard := false
	if len(n.Cases) > 0 {
		if _, ok := n.Cases[len(n.Cases)-1].Pattern.(*WildcardPattern); ok {
			lastIsWildcard = true
		}
	}

	var tail ast.Stmt
	for i := len(n.Cases) - 1; i >= 0; i-- {
		c := n.Cases[i]
		if _, ok := c.Pattern.(*WildcardPattern); ok {
			code, _, _ := g.translateExpr(c.Body, ctx.child(), expected)
			if expected == "" {
				tail = &ast.ExprStmt{X: code}
			} else {
				tail = &ast.ReturnStmt{Results: []ast.Expr{code}}
			}
			continue
		}
		if lit, ok := c.Pattern.(*LiteralPattern); ok {
			patExpr := litToExpr(lit)
			child := ctx.child()
			code, _, _ := g.translateExpr(c.Body, child, expected)
			var bodyBlock *ast.BlockStmt
			if expected == "" {
				bodyBlock = &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: code}}}
			} else {
				bodyBlock = &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{code}}}}
			}
			cond := &ast.BinaryExpr{X: target, Op: token.EQL, Y: patExpr}
			ifStmt := &ast.IfStmt{Cond: cond, Body: bodyBlock}
			if tail != nil {
				ifStmt.Else = &ast.BlockStmt{List: []ast.Stmt{tail}}
			}
			tail = ifStmt
			continue
		}
		if vp, ok := c.Pattern.(*VariantPattern); ok {
			g.switchVarSeq++
			varName := "v_" + strconv.Itoa(g.switchVarSeq)

			// Construct type assertion name from enum info
			assertTypeName := vp.Name
			enumName, found := g.variantByName[vp.Name]
			if !found {
				if baseName, _ := splitTypeArgs(ttype); baseName != "" && baseName != vp.Name {
					enumName = baseName
					found = true
				}
			}
			if found {
				assertTypeName = variantNameForEnum(enumName, vp.Name)
			}
			var assertType ast.Expr = ast.NewIdent(assertTypeName)
			if found {
				if _, typeArgs := splitTypeArgs(ttype); len(typeArgs) > 0 {
					taExprs := make([]ast.Expr, len(typeArgs))
					for i, ta := range typeArgs {
						taExprs[i] = ast.NewIdent(ta)
					}
					assertType = genericIdent(assertTypeName, taExprs...)
				}
			}
			// Check if any pattern arg is used in the body
			hasBindings := false
			for _, arg := range vp.Args {
				if arg != "_" && exprUsesIdent(c.Body, arg) {
					hasBindings = true
					break
				}
			}
			child := ctx.child()
			varNameOrBlank := ast.NewIdent("_")
			if hasBindings {
				varNameOrBlank = ast.NewIdent(varName)
				for i, arg := range vp.Args {
					if arg != "_" {
						child.bindings[arg] = fmt.Sprintf("%s.F%d", varName, i)
						child.locals[arg] = ""
					}
				}
			}
			code, _, _ := g.translateExpr(c.Body, child, expected)
			var bodyBlock *ast.BlockStmt
			if expected == "" {
				bodyBlock = &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: code}}}
			} else {
				bodyBlock = &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{code}}}}
			}
			ifStmt := &ast.IfStmt{
				Init: &ast.AssignStmt{
					Lhs: []ast.Expr{varNameOrBlank, ast.NewIdent("ok")},
					Rhs: []ast.Expr{&ast.TypeAssertExpr{X: target, Type: assertType}},
					Tok: token.DEFINE,
				},
				Cond: ast.NewIdent("ok"),
				Body: bodyBlock,
			}
			if tail != nil {
				ifStmt.Else = &ast.BlockStmt{List: []ast.Stmt{tail}}
			} else if expected != "" && !lastIsWildcard {
				ifStmt.Else = &ast.BlockStmt{
					List: []ast.Stmt{&ast.ExprStmt{X: &ast.CallExpr{Fun: ast.NewIdent("panic"), Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote("unreachable")}}}}},
				}
			}
			tail = ifStmt
		}
	}
	_ = lastIsWildcard
	if tail == nil {
		return ast.NewIdent("_"), "", nil
	}
	if expected == "" {
		// Wrap in IIFE since Stmt can't be returned as Expr
		fn := astFuncLit(nil, nil, &ast.BlockStmt{List: []ast.Stmt{tail}})
		return &ast.CallExpr{Fun: fn}, "", nil
	}
	// Wrap in IIFE for expression form
	fn := &ast.FuncLit{
		Type: &ast.FuncType{
			Params:  &ast.FieldList{},
			Results: &ast.FieldList{List: []*ast.Field{{Type: ast.NewIdent(expected)}}},
		},
		Body: &ast.BlockStmt{List: []ast.Stmt{tail}},
	}
	return &ast.CallExpr{Fun: fn}, expected, nil
}

func litToExpr(l *LiteralPattern) ast.Expr {
	switch l.Kind {
	case "string":
		return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(l.Value)}
	case "number":
		return &ast.BasicLit{Kind: token.INT, Value: l.Value}
	default:
		return ast.NewIdent(l.Value)
	}
}

// translateWhile handles while loops.
func (g *gen) translateWhile(n *WhileExpr, ctx *egCtx) (ast.Expr, string, error) {
	cond, _, _ := g.translateExpr(n.Cond, ctx, "bool")
	body := &ast.BlockStmt{}
	switch b := n.Body.(type) {
	case *BlockExpr:
		for _, stmt := range b.Stmts {
			g.translateWhileStmt(stmt, ctx, body)
		}
	default:
		code, _, _ := g.translateExpr(n.Body, ctx, "")
		body.List = append(body.List, &ast.ExprStmt{X: code})
	}
	forStmt := &ast.ForStmt{Cond: cond, Body: body}
	fn := astFuncLit(nil, nil, &ast.BlockStmt{List: []ast.Stmt{forStmt}})
	return &ast.CallExpr{Fun: fn}, "", nil
}

func (g *gen) translateWhileStmt(stmt Stmt, ctx *egCtx, body *ast.BlockStmt) {
	switch s := stmt.(type) {
	case *ExprStmt:
		code, _, _ := g.translateExpr(s.Expr, ctx, "")
		body.List = append(body.List, &ast.ExprStmt{X: code})
	case *LetStmt:
		code, _, _ := g.translateExpr(s.Value, ctx, "")
		actual := sanitizeIdent(s.Name)
		ctx.bindings[s.Name] = actual
		body.List = append(body.List, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(actual)}, Rhs: []ast.Expr{code}, Tok: token.DEFINE})
	case *AssignStmt:
		code, _, _ := g.translateExpr(s.Value, ctx, "")
		actual := ctx.bindings[s.Name]
		body.List = append(body.List, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(actual)}, Rhs: []ast.Expr{code}, Tok: token.ASSIGN})
	}
}

// translateFuncLit handles function literals.
func (g *gen) translateFuncLit(n *FuncLitExpr, ctx *egCtx) (ast.Expr, string, error) {
	retType := g.goReturnType(n.Ret, ctx.typeParams)
	params := make([]*ast.Field, len(n.Params))
	for i, p := range n.Params {
		params[i] = &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(sanitizeIdent(p.Name))},
			Type:  goastTypeExpr(p.Type),
		}
	}
	var results []*ast.Field
	if retType != "" {
		results = []*ast.Field{{Type: ast.NewIdent(retType)}}
	}
	child := ctx.child()
	child.retType = retType
	for _, p := range n.Params {
		child.locals[p.Name] = g.goType(p.Type, ctx.typeParams)
		child.bindings[p.Name] = p.Name
	}
	if block, ok := n.Body.(*BlockExpr); ok {
		stmts, _ := g.translateBlockStmts(block, child, retType, nil)
		return &ast.FuncLit{
			Type: &ast.FuncType{
				Params:  &ast.FieldList{List: params},
				Results: fieldListIfNonEmptyGoast(results),
			},
			Body: &ast.BlockStmt{List: stmts},
		}, retType, nil
	}
	bodyCode, _, _ := g.translateExpr(n.Body, child, retType)
	return &ast.FuncLit{
		Type: &ast.FuncType{
			Params:  &ast.FieldList{List: params},
			Results: fieldListIfNonEmptyGoast(results),
		},
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{bodyCode}}}},
	}, retType, nil
}
