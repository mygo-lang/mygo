package codegen

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

// translateIf handles if expressions.
func (g *gen) translateIf(n *IfExpr, ctx *egCtx, expected string) (translatedExpr, error) {
	cond, _, err := g.translateExpr(n.Cond, ctx, "bool")
	if err != nil {
		return translatedExpr{}, err
	}
	if cond == nil {
		line, col := common.NodePos(n.Cond)
		return translatedExpr{}, common.ErrorAtPos(g.currentFile, line, col, "if condition produced nil Go AST")
	}
	thenCtx := ctx.child()
	elseCtx := ctx.child()
	thenResult, err := g.translateExprResult(n.Then, thenCtx, expected)
	if err != nil {
		return translatedExpr{}, err
	}
	thenCode, thenType := thenResult.Expr, thenResult.Type
	elseResult, err := g.translateExprResult(n.Else, elseCtx, expected)
	if err != nil {
		return translatedExpr{}, err
	}
	elseCode, elseType := elseResult.Expr, elseResult.Type

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
		thenStmts := append([]ast.Stmt{}, thenResult.Stmts...)
		if thenCode != nil {
			thenStmts = append(thenStmts, stmtForExpr(n.Then, thenCode, thenType))
		}
		thenStmt := &ast.BlockStmt{List: thenStmts}
		ifStmt := &ast.IfStmt{
			Cond: cond,
			Body: thenStmt,
		}
		if _, isUnitElse := n.Else.(*UnitLitExpr); elseCode != nil && !isUnitElse {
			elseStmts := append([]ast.Stmt{}, elseResult.Stmts...)
			if elseCode != nil {
				elseStmts = append(elseStmts, stmtForExpr(n.Else, elseCode, elseType))
			}
			elseStmt := &ast.BlockStmt{List: elseStmts}
			ifStmt.Else = elseStmt
		}
		return translatedExpr{Stmts: []ast.Stmt{ifStmt}}, nil
	}
	g.localSeq++
	tmp := "expr_" + strconv.Itoa(g.localSeq)
	ifStmt := &ast.IfStmt{Cond: cond,
		Body: &ast.BlockStmt{List: append(thenResult.Stmts, &ast.ReturnStmt{Results: []ast.Expr{thenCode}})},
		Else: &ast.BlockStmt{List: append(elseResult.Stmts, &ast.ReturnStmt{Results: []ast.Expr{elseCode}})},
	}
	return translatedExpr{Expr: ast.NewIdent(tmp), Type: resultType, Stmts: append([]ast.Stmt{&ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{Names: []*ast.Ident{ast.NewIdent(tmp)}, Type: g.goTypeExprFromString(resultType)}}}}}, renameIIFEReturns([]ast.Stmt{ifStmt}, tmp)...)}, nil
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
	if whileExpr, ok := e.(*WhileExpr); ok {
		forStmt, err := g.translateWhileFor(whileExpr, ctx)
		if err != nil {
			return nil, err
		}
		return &ast.BlockStmt{List: []ast.Stmt{forStmt}}, nil
	}
	if ifExpr, ok := e.(*IfExpr); ok {
		ifStmt, err := g.translateIfStmt(ifExpr, ctx, "", nil)
		if err != nil {
			return nil, err
		}
		return &ast.BlockStmt{List: []ast.Stmt{ifStmt}}, nil
	}
	if switchExpr, ok := e.(*SwitchExpr); ok {
		translated, err := g.translateExprResult(switchExpr, ctx, "")
		if err != nil {
			return nil, err
		}
		return &ast.BlockStmt{List: translated.Stmts}, nil
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
func (g *gen) translateSwitch(n *SwitchExpr, ctx *egCtx, expected string) (translatedExpr, error) {
	statementForm := isUnitGoType(expected)
	if statementForm {
		expected = ""
	}
	target, ttype, err := g.translateExpr(n.Target, ctx, "")
	if err != nil {
		return translatedExpr{}, err
	}
	if inferred := g.inferredType(n.Target); inferred != "" && !containsGeneratedTypeVar(inferred) {
		ttype = inferred
	}
	ttype = refineSwitchTargetTypeFromCases(ttype, n.Cases)
	if isNilASTExpr(target) {
		line, col := common.NodePos(n.Target)
		return translatedExpr{}, common.ErrorAtPos(g.currentFile, line, col, "switch target produced nil Go AST")
	}
	_, _ = target, ttype
	if !statementForm {
		for _, c := range n.Cases {
			if typ := switchBodyType(c.Body, g, ctx); typ != "" {
				if expected == "" || !sameSwitchResultType(expected, typ, g) {
					expected = typ
				}
				break
			}
		}
	}

	lastIsWildcard := false
	if len(n.Cases) > 0 {
		if _, ok := n.Cases[len(n.Cases)-1].Pattern.(*WildcardPattern); ok {
			lastIsWildcard = true
		}
	}

	var tail ast.Stmt
	caseBody := func(body Expr, bodyCtx *egCtx) (*ast.BlockStmt, ast.Expr, error) {
		translated, err := g.translateExprResult(body, bodyCtx, expected)
		if err != nil {
			return nil, nil, err
		}
		if expected == "" || isUnitGoType(expected) {
			if translated.Expr != nil && !isNilASTExpr(translated.Expr) {
				translated.Stmts = append(translated.Stmts, stmtForExpr(body, translated.Expr, translated.Type))
			}
			return &ast.BlockStmt{List: translated.Stmts}, nil, nil
		}
		if translated.Expr == nil || isNilASTExpr(translated.Expr) {
			return nil, nil, common.ErrorAtPos(g.currentFile, 0, 0, "switch case body produced nil Go AST")
		}
		stmts := append([]ast.Stmt{}, translated.Stmts...)
		stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{translated.Expr}})
		return &ast.BlockStmt{List: stmts}, nil, nil
	}
	for i := len(n.Cases) - 1; i >= 0; i-- {
		c := n.Cases[i]
		if _, ok := c.Pattern.(*WildcardPattern); ok {
			bodyBlock, code, err := caseBody(c.Body, ctx.child())
			if err != nil {
				return translatedExpr{}, err
			}
			tail = bodyBlock
			if code != nil {
				tail = stmtForExpr(c.Body, code, expected)
			}
			continue
		}
		if lit, ok := c.Pattern.(*LiteralPattern); ok {
			patExpr := litToExpr(lit)
			child := ctx.child()
			bodyBlock, code, err := caseBody(c.Body, child)
			if err != nil {
				return translatedExpr{}, err
			}
			if code != nil {
				bodyBlock = &ast.BlockStmt{List: []ast.Stmt{stmtForExpr(c.Body, code, expected)}}
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
						taExprs[i] = g.goTypeExprForAssertion(ta)
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
			bodyBlock, code, err := caseBody(c.Body, child)
			if err != nil {
				return translatedExpr{}, err
			}
			if code != nil {
				bodyBlock = &ast.BlockStmt{List: []ast.Stmt{stmtForExpr(c.Body, code, expected)}}
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
		return translatedExpr{Expr: ast.NewIdent("_")}, nil
	}
	if expected == "" || isUnitGoType(expected) {
		return translatedExpr{Stmts: []ast.Stmt{tail}}, nil
	}
	g.localSeq++
	tmp := "expr_" + strconv.Itoa(g.localSeq)
	return translatedExpr{
		Expr: ast.NewIdent(tmp), Type: expected,
		Stmts: append([]ast.Stmt{&ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
			Names: []*ast.Ident{ast.NewIdent(tmp)}, Type: g.goTypeExprFromString(expected),
		}}}}}, renameIIFEReturns([]ast.Stmt{tail}, tmp)...),
	}, nil
}

func switchBodyType(body Expr, g *gen, ctx *egCtx) string {
	// A case block is an expression in its own right. Its result is its final
	// expression, rather than the return type expected by an enclosing callback.
	if block, ok := body.(*BlockExpr); ok {
		if len(block.Stmts) == 0 {
			return ""
		}
		if last, ok := block.Stmts[len(block.Stmts)-1].(*ExprStmt); ok {
			return switchBodyType(last.Expr, g, ctx)
		}
		return ""
	}
	if inferred := g.inferredType(body); inferred != "" {
		return inferred
	}
	if ifExpr, ok := body.(*IfExpr); ok {
		if typ := switchBodyType(ifExpr.Then, g, ctx.child()); typ != "" {
			return typ
		}
	}
	_, typ, err := g.translateExpr(body, ctx.child(), "")
	if err != nil {
		return ""
	}
	return typ
}

func sameSwitchResultType(a, b string, g *gen) bool {
	return reflect.DeepEqual(g.goTypeExprFromString(a), g.goTypeExprFromString(b))
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
func (g *gen) translateWhile(n *WhileExpr, ctx *egCtx) (translatedExpr, error) {
	forStmt, err := g.translateWhileFor(n, ctx)
	if err != nil {
		return translatedExpr{}, err
	}
	return translatedExpr{Stmts: []ast.Stmt{forStmt}}, nil
}

func (g *gen) translateWhileFor(n *WhileExpr, ctx *egCtx) (*ast.ForStmt, error) {
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
	return forStmt, nil
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
		translated, err := g.translateExprResult(s.Expr, ctx, "")
		if err != nil {
			return
		}
		body.List = append(body.List, translated.Stmts...)
		if translated.Expr != nil {
			body.List = append(body.List, &ast.ExprStmt{X: translated.Expr})
		}
	case *LetStmt:
		code, vtype, _ := g.translateExpr(s.Value, ctx, "")
		if code == nil {
			return
		}
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
		if code == nil {
			return
		}
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
	bodyResult, err := g.translateExprResult(n.Body, child, retType)
	if err != nil {
		return nil, "", err
	}
	bodyCode := bodyResult.Expr
	if bodyCode == nil && retType != "" && !isUnitGoType(retType) {
		line, col := common.NodePos(n.Body)
		return nil, "", common.ErrorAtPos(g.currentFile, line, col, "function literal body produced nil Go AST")
	}
	body := &ast.BlockStmt{List: append(bodyResult.Stmts, &ast.ReturnStmt{Results: []ast.Expr{bodyCode}})}
	if retType == "" || isUnitGoType(retType) {
		body = &ast.BlockStmt{List: bodyResult.Stmts}
		if bodyCode != nil {
			body.List = append(body.List, stmtForExpr(n.Body, bodyCode, ""))
		}
	}
	return &ast.FuncLit{
		Type: &ast.FuncType{
			Params:  &ast.FieldList{List: params},
			Results: fieldListIfNonEmptyGoast(results),
		},
		Body: body,
	}, retType, nil
}
