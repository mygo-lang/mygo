package codegen

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

// translateIf handles if expressions.
func (g *gen) translateIf(n *IfExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	cond, _, err := g.translateExpr(n.Cond, ctx, "bool")
	if err != nil {
		return nil, "", err
	}
	if cond == nil {
		line, col := common.NodePos(n.Cond)
		return nil, "", common.ErrorAtPos(g.currentFile, line, col, "if condition produced nil Go AST")
	}
	thenCtx := ctx.child()
	elseCtx := ctx.child()
	thenCode, thenType, err := g.translateExpr(n.Then, thenCtx, expected)
	if err != nil {
		return nil, "", err
	}
	if isNilASTExpr(thenCode) {
		line, col := common.NodePos(n.Then)
		return nil, "", common.ErrorAtPos(g.currentFile, line, col, "if then branch produced nil Go AST")
	}
	elseCode, elseType, err := g.translateExpr(n.Else, elseCtx, expected)
	if err != nil {
		return nil, "", err
	}
	if isNilASTExpr(elseCode) {
		line, col := common.NodePos(n.Else)
		return nil, "", common.ErrorAtPos(g.currentFile, line, col, "if else branch produced nil Go AST")
	}

	resultType := expected
	if resultType == "" {
		if _, isUnitElse := n.Else.(*UnitLitExpr); isUnitElse {
			resultType = "Unit"
		} else if elseType == "" {
			resultType = "Unit"
		} else if thenType != "" {
			resultType = thenType
		} else {
			resultType = elseType
		}
	}
	if resultType == "" || resultType == "any" || isUnitGoType(resultType) {
		// Statement form: wrap in IIFE so both branches are expressions
		thenStmt := stmtForExpr(n.Then, thenCode, thenType)
		if exprStmt, ok := thenStmt.(*ast.ExprStmt); ok && isNilASTExpr(exprStmt.X) {
			line, col := common.NodePos(n.Then)
			return nil, "", common.ErrorAtPos(g.currentFile, line, col, "if then branch statement produced nil Go AST")
		}
		ifStmt := &ast.IfStmt{
			Cond: cond,
			Body: &ast.BlockStmt{List: []ast.Stmt{thenStmt}},
		}
		if _, isUnitElse := n.Else.(*UnitLitExpr); elseCode != nil && !isUnitElse {
			elseStmt := stmtForExpr(n.Else, elseCode, elseType)
			if exprStmt, ok := elseStmt.(*ast.ExprStmt); ok && isNilASTExpr(exprStmt.X) {
				line, col := common.NodePos(n.Else)
				return nil, "", common.ErrorAtPos(g.currentFile, line, col, "if else branch statement produced nil Go AST")
			}
			ifStmt.Else = &ast.BlockStmt{List: []ast.Stmt{elseStmt}}
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

func (g *gen) translateIfStmt(n *IfExpr, ctx *egCtx, returnExpected string, retTypes []string) (*ast.IfStmt, error) {
	cond, _, err := g.translateExpr(n.Cond, ctx, "bool")
	if err != nil {
		return nil, err
	}
	if isNilASTExpr(cond) {
		line, col := common.NodePos(n.Cond)
		return nil, common.ErrorAtPos(g.currentFile, line, col, "if condition produced nil Go AST")
	}
	thenBlock, err := g.exprStmtBlock(n.Then, ctx.child(), returnExpected, retTypes)
	if err != nil {
		return nil, err
	}
	ifStmt := &ast.IfStmt{Cond: cond, Body: thenBlock}
	if _, isUnitElse := n.Else.(*UnitLitExpr); n.Else != nil && !isUnitElse {
		elseBlock, err := g.exprStmtBlock(n.Else, ctx.child(), returnExpected, retTypes)
		if err != nil {
			return nil, err
		}
		ifStmt.Else = elseBlock
	}
	return ifStmt, nil
}

func (g *gen) exprStmtBlock(e Expr, ctx *egCtx, returnExpected string, retTypes []string) (*ast.BlockStmt, error) {
	if block, ok := e.(*BlockExpr); ok {
		stmts, err := g.translateBlockStmts(block, ctx, "", nil)
		if err != nil {
			return nil, err
		}
		return &ast.BlockStmt{List: stmts}, nil
	}
	code, typ, err := g.translateExpr(e, ctx, "")
	if err != nil {
		return nil, err
	}
	if isNilASTExpr(code) {
		line, col := common.NodePos(e)
		return nil, common.ErrorAtPos(g.currentFile, line, col, "if branch produced nil Go AST")
	}
	return &ast.BlockStmt{List: []ast.Stmt{stmtForExpr(e, code, typ)}}, nil
}

// translateSwitch handles switch expressions.
func (g *gen) translateSwitch(n *SwitchExpr, ctx *egCtx, expected string) (ast.Expr, string, error) {
	if isUnitGoType(expected) {
		expected = ""
	}
	target, ttype, err := g.translateExpr(n.Target, ctx, "")
	if err != nil {
		return nil, "", err
	}
	if inferred := g.inferredType(n.Target); inferred != "" && !containsGeneratedTypeVar(inferred) {
		ttype = inferred
	}
	ttype = refineSwitchTargetTypeFromCases(ttype, n.Cases)
	if isNilASTExpr(target) {
		line, col := common.NodePos(n.Target)
		return nil, "", common.ErrorAtPos(g.currentFile, line, col, "switch target produced nil Go AST")
	}
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
			code, _, err := g.translateExpr(c.Body, ctx.child(), expected)
			if err != nil {
				return nil, "", err
			}
			if isNilASTExpr(code) {
				line, col := common.NodePos(c.Body)
				return nil, "", common.ErrorAtPos(g.currentFile, line, col, "switch wildcard case body produced nil Go AST")
			}
			if expected == "" {
				tail = stmtForExpr(c.Body, code, "")
			} else {
				tail = &ast.ReturnStmt{Results: []ast.Expr{code}}
			}
			continue
		}
		if lit, ok := c.Pattern.(*LiteralPattern); ok {
			patExpr := litToExpr(lit)
			child := ctx.child()
			code, _, err := g.translateExpr(c.Body, child, expected)
			if err != nil {
				return nil, "", err
			}
			if isNilASTExpr(code) {
				line, col := common.NodePos(c.Body)
				return nil, "", common.ErrorAtPos(g.currentFile, line, col, "switch literal case body produced nil Go AST")
			}
			var bodyBlock *ast.BlockStmt
			if expected == "" {
				bodyBlock = &ast.BlockStmt{List: []ast.Stmt{stmtForExpr(c.Body, code, "")}}
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
			if qualifiedName, ok := qualifiedVariantNameFromTargetType(ttype, vp.Name); ok {
				assertTypeName = qualifiedName
				found = true
			}
			var assertType ast.Expr = ast.NewIdent(assertTypeName)
			if found {
				if _, typeArgs := splitTypeArgs(ttype); len(typeArgs) > 0 {
					taExprs := make([]ast.Expr, len(typeArgs))
					for i, ta := range typeArgs {
						// Type assertion types must use Go-level names (e.g. "any" not "Any").
						taExprs[i] = goTypeExprForAssertion(ta)
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
						// Compute the Go-level type for each pattern arg from the enum definition.
						if enumName != "" && found {
							if enum, ok := g.pkg.Enums[enumName]; ok {
								for _, variant := range enum.Variants {
									if variant.Name == vp.Name && i < len(variant.Fields) {
										_, typeArgs := splitTypeArgs(ttype)
										subst := map[string]string{}
										for j, tp := range enum.TypeParams {
											if j < len(typeArgs) {
												subst[tp] = typeArgs[j]
											}
										}
										fieldType := substituteTypeExpr(variant.Fields[i].Type, subst)
										tpSet := map[string]struct{}{}
										for _, tp := range enum.TypeParams {
											tpSet[tp] = struct{}{}
										}
										child.locals[arg] = g.goType(fieldType, tpSet)
										break
									}
								}
							}
						}
					}
				}
				refinePatternBindingTypesFromBody(child, vp.Args, c.Body)
			}
			code, _, err := g.translateExpr(c.Body, child, expected)
			if err != nil {
				return nil, "", err
			}
			if isNilASTExpr(code) {
				line, col := common.NodePos(c.Body)
				return nil, "", common.ErrorAtPos(g.currentFile, line, col, "switch variant case body produced nil Go AST")
			}
			var bodyBlock *ast.BlockStmt
			if expected == "" {
				bodyBlock = &ast.BlockStmt{List: []ast.Stmt{stmtForExpr(c.Body, code, "")}}
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
	if expected == "" || isUnitGoType(expected) {
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
	case "rune":
		return &ast.BasicLit{Kind: token.CHAR, Value: strconv.QuoteRune([]rune(l.Value)[0])}
	case "number":
		return &ast.BasicLit{Kind: token.INT, Value: l.Value}
	default:
		return ast.NewIdent(l.Value)
	}
}

func refinePatternBindingTypesFromBody(ctx *egCtx, names []string, body Expr) {
	if ctx == nil || body == nil {
		return
	}
	nameSet := map[string]struct{}{}
	for _, name := range names {
		if name != "_" {
			nameSet[name] = struct{}{}
		}
	}
	var visit func(Expr)
	visit = func(e Expr) {
		switch n := e.(type) {
		case *BinaryExpr:
			refinePatternBindingType(ctx, nameSet, n.Left, n.Right)
			refinePatternBindingType(ctx, nameSet, n.Right, n.Left)
			visit(n.Left)
			visit(n.Right)
		case *BlockExpr:
			for _, st := range n.Stmts {
				if es, ok := st.(*ExprStmt); ok {
					visit(es.Expr)
				}
			}
		case *IfExpr:
			visit(n.Cond)
			visit(n.Then)
			visit(n.Else)
		case *SwitchExpr:
			visit(n.Target)
			for _, c := range n.Cases {
				visit(c.Body)
			}
		case *CallExpr:
			visit(n.Callee)
			for _, arg := range n.Args {
				visit(arg)
			}
		case *FieldExpr:
			visit(n.Expr)
		case *PrefixExpr:
			visit(n.Expr)
		case *CastExpr:
			visit(n.Expr)
		case *TupleLitExpr:
			for _, elem := range n.Elems {
				visit(elem)
			}
		case *SliceLitExpr:
			for _, elem := range n.Elems {
				visit(elem)
			}
		}
	}
	visit(body)
}

func refineSwitchTargetTypeFromCases(targetType string, cases []SwitchCase) string {
	base, args := splitTypeArgs(targetType)
	if base == "" || len(args) == 0 {
		return targetType
	}
	refined := append([]string(nil), args...)
	for _, c := range cases {
		vp, ok := c.Pattern.(*VariantPattern)
		if !ok || len(vp.Args) == 0 {
			continue
		}
		for i, arg := range vp.Args {
			if arg == "_" || i >= len(refined) || !isGeneratedTypeVar(refined[i]) {
				continue
			}
			if typ := inferredPatternNameTypeFromBody(arg, c.Body); typ != "" {
				refined[i] = typ
			}
		}
	}
	if strings.Join(refined, ", ") == strings.Join(args, ", ") {
		return targetType
	}
	return base + "[" + strings.Join(refined, ", ") + "]"
}

func qualifiedVariantNameFromTargetType(targetType, variantName string) (string, bool) {
	baseName, _ := splitTypeArgs(targetType)
	dotIdx := strings.LastIndexByte(baseName, '.')
	if dotIdx <= 0 || dotIdx == len(baseName)-1 {
		return "", false
	}
	alias := baseName[:dotIdx]
	enumName := baseName[dotIdx+1:]
	return alias + "." + variantNameForEnum(enumName, variantName), true
}

func inferredPatternNameTypeFromBody(name string, body Expr) string {
	found := ""
	var visit func(Expr)
	visit = func(e Expr) {
		if found != "" || e == nil {
			return
		}
		switch n := e.(type) {
		case *BinaryExpr:
			if id, ok := n.Left.(*IdentExpr); ok && id.Name == name {
				found = literalGoType(n.Right)
				return
			}
			if id, ok := n.Right.(*IdentExpr); ok && id.Name == name {
				found = literalGoType(n.Left)
				return
			}
			visit(n.Left)
			visit(n.Right)
		case *BlockExpr:
			for _, st := range n.Stmts {
				if es, ok := st.(*ExprStmt); ok {
					visit(es.Expr)
				}
			}
		case *IfExpr:
			visit(n.Cond)
			visit(n.Then)
			visit(n.Else)
		case *CallExpr:
			visit(n.Callee)
			for _, arg := range n.Args {
				visit(arg)
			}
		case *FieldExpr:
			visit(n.Expr)
		case *PrefixExpr:
			visit(n.Expr)
		case *CastExpr:
			visit(n.Expr)
		}
	}
	visit(body)
	return found
}

func refinePatternBindingType(ctx *egCtx, names map[string]struct{}, maybeIdent Expr, other Expr) {
	id, ok := maybeIdent.(*IdentExpr)
	if !ok {
		return
	}
	if _, ok := names[id.Name]; !ok {
		return
	}
	current := ctx.locals[id.Name]
	if current != "" && !isGeneratedTypeVar(current) {
		return
	}
	if typ := literalGoType(other); typ != "" {
		ctx.locals[id.Name] = typ
	}
}

func literalGoType(e Expr) string {
	lit, ok := e.(*LiteralExpr)
	if !ok {
		return ""
	}
	switch lit.Kind {
	case "string":
		return "string"
	case "rune":
		return "rune"
	case "bool":
		return "bool"
	case "number":
		if strings.Contains(lit.Value, ".") {
			return "float64"
		}
		return "int"
	default:
		return ""
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

func isBreakOrContinue(e Expr) bool {
	if id, ok := e.(*IdentExpr); ok {
		return id.Name == "break" || id.Name == "continue"
	}
	return false
}

func (g *gen) translateWhileStmt(stmt Stmt, ctx *egCtx, body *ast.BlockStmt) {
	switch s := stmt.(type) {
	case *ExprStmt:
		// If this is an if-expression with break/continue branches, handle directly
		// to avoid wrapping break in an IIFE (which is invalid Go).
		if ifExpr, ok := s.Expr.(*IfExpr); ok {
			if isBreakOrContinue(ifExpr.Then) || isBreakOrContinue(ifExpr.Else) {
				cond, _, _ := g.translateExpr(ifExpr.Cond, ctx, "bool")
				// Translate branches directly (no IIFE) so break/continue work.
				thenCode, thenType, _ := g.translateExpr(ifExpr.Then, ctx, "")
				thenStmt := stmtForExpr(ifExpr.Then, thenCode, thenType)
				elseCode, elseType, _ := g.translateExpr(ifExpr.Else, ctx, "")
				elseStmt := stmtForExpr(ifExpr.Else, elseCode, elseType)
				ifStmt := &ast.IfStmt{Cond: cond, Body: &ast.BlockStmt{List: []ast.Stmt{thenStmt}}}
				if elseStmt != nil {
					ifStmt.Else = &ast.BlockStmt{List: []ast.Stmt{elseStmt}}
				}
				body.List = append(body.List, ifStmt)
				return
			}
		}
		code, _, _ := g.translateExpr(s.Expr, ctx, "")
		body.List = append(body.List, &ast.ExprStmt{X: code})
	case *LetStmt:
		code, vtype, _ := g.translateExpr(s.Value, ctx, "")
		if s.Name == "_" {
			body.List = append(body.List, &ast.ExprStmt{X: code})
		} else {
			actual := sanitizeIdent(s.Name)
			ctx.bindings[s.Name] = actual
			ctx.locals[s.Name] = vtype
			body.List = append(body.List, &ast.AssignStmt{Lhs: []ast.Expr{ast.NewIdent(actual)}, Rhs: []ast.Expr{code}, Tok: token.DEFINE})
		}
	case *AssignStmt:
		code, retType, _ := g.translateExpr(s.Value, ctx, "")
		actual := ctx.bindings[s.Name]
		if retType != "" {
			ctx.locals[actual] = retType
		}
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
		stmts, err := g.translateBlockStmts(block, child, retType, nil)
		if err != nil {
			return nil, "", err
		}
		return &ast.FuncLit{
			Type: &ast.FuncType{
				Params:  &ast.FieldList{List: params},
				Results: fieldListIfNonEmptyGoast(results),
			},
			Body: &ast.BlockStmt{List: stmts},
		}, retType, nil
	}
	bodyCode, _, err := g.translateExpr(n.Body, child, retType)
	if err != nil {
		return nil, "", err
	}
	if bodyCode == nil {
		line, col := common.NodePos(n.Body)
		return nil, "", common.ErrorAtPos(g.currentFile, line, col, "function literal body produced nil Go AST")
	}
	body := &ast.BlockStmt{List: []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{bodyCode}}}}
	if retType == "" || isUnitGoType(retType) {
		body = &ast.BlockStmt{List: []ast.Stmt{stmtForExpr(n.Body, bodyCode, "")}}
	}
	return &ast.FuncLit{
		Type: &ast.FuncType{
			Params:  &ast.FieldList{List: params},
			Results: fieldListIfNonEmptyGoast(results),
		},
		Body: body,
	}, retType, nil
}
