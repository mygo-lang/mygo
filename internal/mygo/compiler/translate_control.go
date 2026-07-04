package compiler

import (
	"fmt"
	"strings"

	jen "github.com/dave/jennifer/jen"
	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func stmtIsStatementSafe(expr Expr) bool {
	switch n := expr.(type) {
	case *CallExpr, *FuncLitExpr, *IfExpr, *SwitchExpr, *BlockExpr:
		return true
	case *BinaryExpr:
		return n.Op == "|>" || n.Op == "<|"
	default:
		return false
	}
}

func (g *generator) bindLocal(ctx *exprCtx, source, typ string, mutable bool) string {
	actual := sanitizeIdent(source)
	if actual == "" || actual == "_" {
		actual = "tmp"
	}
	g.localSeq++
	actual = fmt.Sprintf("%s_%d", actual, g.localSeq)
	ctx.bindings[source] = actual
	ctx.locals[source] = typ
	ctx.sourceTypes[source] = typ
	ctx.mutable[actual] = mutable
	return actual
}

func (g *generator) translateBlock(n *BlockExpr, ctx *exprCtx, expected string) (jen.Code, string, error) {
	returnExpected := expected
	if returnExpected == "" {
		returnExpected = ctx.retType
	}
	// Build a function literal: func() returnType { stmts }
	// We collect all statements into a single Block.
	b := jen.Func().Params()
	if expected != "" {
		b.Add(jenTypeExpr(&NamedType{Name: expected}))
	}
	child := ctx.child()
	var stmtCodes []jen.Code
	var lastWasExprStmt bool
	var sawReturn bool
	for i, stmt := range n.Stmts {
		isLast := i == len(n.Stmts)-1
		switch s := stmt.(type) {
		case *ExprStmt:
			lastWasExprStmt = isLast
			stmtExpected := ""
			if isLast {
				stmtExpected = expected
			}
			code, typ, err := g.translateExpr(s.Expr, child, stmtExpected)
			if err != nil {
				return nil, "", err
			}
			if isLast && expected != "" {
				if typ == "" {
					line, col := common.NodePos(s)
					return nil, "", common.ErrorAtPos(line, col, "block must end with an expression returning %s, but got empty type (expression was %T)", expected, s.Expr)
				}
				stmtCodes = append(stmtCodes, jen.Return(code))
				continue
			}
			if stmtIsStatementSafe(s.Expr) || typ == "" {
				stmtCodes = append(stmtCodes, code)
			} else {
				stmtCodes = append(stmtCodes, jen.Id("_").Op("=").Add(code))
			}
		case *ReturnStmt:
			sawReturn = true
			if s.Value != nil {
				code, typ, err := g.translateExpr(s.Value, child, returnExpected)
				if err != nil {
					return nil, "", err
				}
				if returnExpected == "" {
					line, col := common.NodePos(s)
					return nil, "", common.ErrorAtPos(line, col, "return with a value requires a non-unit function")
				}
				if typ == "" {
					line, col := common.NodePos(s)
					return nil, "", common.ErrorAtPos(line, col, "return value must have type %s", returnExpected)
				}
				stmtCodes = append(stmtCodes, jen.Return(code))
			} else if returnExpected != "" {
				line, col := common.NodePos(s)
				return nil, "", common.ErrorAtPos(line, col, "return requires a value of type %s", returnExpected)
			}
		case *LetStmt:
			lastWasExprStmt = false
			code, typ, err := g.translateExpr(s.Value, child, g.goType(s.Type, child.typeParams))
			if err != nil {
				return nil, "", err
			}
			if s.Name == "_" {
				if stmtIsStatementSafe(s.Value) {
					stmtCodes = append(stmtCodes, code)
				} else {
					stmtCodes = append(stmtCodes, jen.Id("_").Op("=").Add(code))
				}
			} else {
				actualName := g.bindLocal(child, s.Name, typ, s.Mutable)
				bindType := typ
				if s.Type != nil {
					bindType = g.goType(s.Type, child.typeParams)
					stmtCodes = append(stmtCodes, jen.Var().Id(actualName).Add(jenTypeExpr(&NamedType{Name: bindType})).Op("=").Add(code))
				} else {
					stmtCodes = append(stmtCodes, jen.Id(actualName).Op(":=").Add(code))
				}
				child.locals[s.Name] = bindType
				child.sourceTypes[s.Name] = bindType
				child.bindings[s.Name] = actualName
			}
		case *AssignStmt:
			lastWasExprStmt = false
			actualName, ok := child.bindings[s.Name]
			if !ok {
				return nil, "", common.ErrorAtPos(s.Line, s.Column, "unknown binding %q", s.Name)
			}
			if !child.mutable[actualName] {
				return nil, "", common.ErrorAtPos(s.Line, s.Column, "cannot assign to immutable binding %q", s.Name)
			}
			targetType := child.locals[s.Name]
			code, _, err := g.translateExpr(s.Value, child, targetType)
			if err != nil {
				return nil, "", err
			}
			stmtCodes = append(stmtCodes, jen.Id(actualName).Op("=").Add(code))
		default:
			lastWasExprStmt = false
			line, col := common.NodePos(stmt)
			return nil, "", common.ErrorAtPos(line, col, "unsupported statement %#v", stmt)
		}
	}
	// If the block does not end with a value-producing statement (ExprStmt or
	// ReturnStmt), treat it as a side-effect block. Don't wrap in a value-returning
	// function literal — just return the raw statements.
	if expected != "" && !lastWasExprStmt && !sawReturn {
		// This is a statement-only block; return the statements with no return type.
		rawBlock := jen.Block(stmtCodes...)
		return rawBlock, "", nil
	}
	// Wrap all collected statements in a single Block and immediately call it.
	b = b.Block(stmtCodes...).Call()
	return b, expected, nil
}

func (g *generator) translateFuncLit(n *FuncLitExpr, outer *exprCtx) (jen.Code, string, error) {
	retType := g.goReturnType(n.Ret, outer.typeParams)
	b := jen.Func().Params()
	child := outer.child()
	child.retType = retType
	for i, p := range n.Params {
		if i > 0 {
		}
		tp := g.goType(p.Type, outer.typeParams)
		child.locals[p.Name] = tp
		b = b.Params(jen.Id(sanitizeIdent(p.Name)).Add(jenTypeExpr(&NamedType{Name: tp})))
	}
	if retType != "" {
		b.Add(jenTypeExpr(&NamedType{Name: retType}))
	}
	body, bodyType, err := g.translateExpr(n.Body, child, retType)
	if err != nil {
		return nil, "", err
	}
	if retType == "" {
		b = b.Block(body)
	} else {
		b = b.Block(jen.Return(body))
	}
	_ = bodyType
	return b, retType, nil
}

func (g *generator) translateIf(n *IfExpr, ctx *exprCtx, expected string) (jen.Code, string, error) {
	cond, _, err := g.translateExpr(n.Cond, ctx, "")
	if err != nil {
		return nil, "", err
	}
	thenCtx := ctx.child()
	elseCtx := ctx.child()
	thenCode, thenType, err := g.translateExpr(n.Then, thenCtx, expected)
	if err != nil {
		return nil, "", err
	}
	elseCode, elseType, err := g.translateExpr(n.Else, elseCtx, expected)
	if err != nil {
		return nil, "", err
	}
	resultType := expected
	if resultType == "" {
		switch {
		case thenType != "" && thenType == elseType:
			resultType = thenType
		case thenType != "":
			resultType = thenType
		default:
			resultType = elseType
		}
	}
	// Build function literal wrapping the if/else
	fn := jen.Func().Params()
	if resultType != "" {
		fn.Add(jenTypeExpr(&NamedType{Name: resultType}))
	}
	// Construct single block with if-else chain
	if resultType == "" {
		fn = fn.Block(jen.If(cond).Block(thenCode).Else().Block(elseCode))
	} else {
		fn = fn.Block(jen.If(cond).Block(jen.Return(thenCode)).Else().Block(jen.Return(elseCode)))
	}
	return fn, resultType, nil
}

func (g *generator) translateSwitch(n *SwitchExpr, ctx *exprCtx, expected string) (jen.Code, string, error) {
	targetCode, targetType, err := g.translateExpr(n.Target, ctx, "")
	if err != nil {
		return nil, "", err
	}
	enumName, enumArgs := splitTypeArgs(targetType)
	enumDecl := g.pkg.Enums[enumName]
	if enumDecl == nil {
		return nil, "", common.ErrorAtPos(n.Line, n.Column, "switch target %q is not an enum", targetType)
	}

	// Classify: does the last case have a wildcard?
	lastIsWildcard := false
	if len(n.Cases) > 0 {
		if _, ok := n.Cases[len(n.Cases)-1].Pattern.(*WildcardPattern); ok {
			lastIsWildcard = true
		}
	}

	// Build each case into a translation function.
	// We'll chain them from last to first to produce nested if-else.
	type caseTranslation struct {
		isVariant bool
		ifStmt    *jen.Statement // non-nil for variant: if cond { body }
		elseBody  jen.Code       // non-nil for wildcard: plain body
	}

	var trans []caseTranslation

	for _, c := range n.Cases {
		switch pat := c.Pattern.(type) {
		case *WildcardPattern:
			child := ctx.child()
			body, bodyType, err := g.translateExpr(c.Body, child, expected)
			if err != nil {
				return nil, "", err
			}
			var bodyBlock jen.Code
			if expected == "" {
				if bodyType == "" {
					bodyBlock = body
				} else {
					bodyBlock = jen.Id("_").Op("=").Add(body)
				}
			} else {
				bodyBlock = jen.Return(body)
			}
			trans = append(trans, caseTranslation{isVariant: false, elseBody: bodyBlock})

		case *VariantPattern:
			variant := g.findVariant(enumDecl, pat.Name)
			if variant == nil {
				return nil, "", common.ErrorAtPos(pat.Line, pat.Column, "unknown variant %s of %s", pat.Name, enumDecl.Name)
			}
			tname := variantGoTypeName(enumDecl.Name, variant.Name)

			// Build the type assertion type: OptionSome or OptionSome[Int]
			assertType := jen.Id(tname)
			if len(enumArgs) > 0 {
				assertType = bracketArgs(assertType, genJenIds(enumArgs))
			}

			// Determine if any pattern args are used in the body
			hasBindings := false
			for _, arg := range pat.Args {
				if exprUsesIdent(c.Body, arg) {
					hasBindings = true
					break
				}
			}

			// Build condition: v, ok := target.(VariantType)  or just  _, ok
			// Use Op(".").Parens() instead of Assert() to avoid extra spaces.
			typeAssert := jen.Op(".").Parens(assertType)
			var cond *jen.Statement
			if hasBindings {
				cond = jen.List(jen.Id("v"), jen.Id("ok")).Op(":=").Add(targetCode).Add(typeAssert)
			} else {
				cond = jen.List(jen.Id("_"), jen.Id("ok")).Op(":=").Add(targetCode).Add(typeAssert)
			}

			// Build bindings for the case body
			child := ctx.child()
			for i, arg := range pat.Args {
				if i >= len(variant.Fields) {
					return nil, "", common.ErrorAtPos(pat.Line, pat.Column, "pattern %s arity mismatch", pat.Name)
				}
				if !exprUsesIdent(c.Body, arg) {
					continue
				}
				child.bindings[arg] = fmt.Sprintf("v.F%d", i)
				child.locals[arg] = g.goType(variant.Fields[i].Type, nil)
			}

			body, bodyType, err := g.translateExpr(c.Body, child, expected)
			if err != nil {
				return nil, "", err
			}
			var bodyBlock jen.Code
			if expected == "" {
				if bodyType == "" {
					bodyBlock = body
				} else {
					bodyBlock = jen.Id("_").Op("=").Add(body)
				}
			} else {
				bodyBlock = jen.Return(body)
			}

			trans = append(trans, caseTranslation{
				isVariant: true,
				ifStmt:    jen.If(cond, jen.Id("ok")).Block(bodyBlock),
			})

		default:
			line, col := common.NodePos(c.Pattern)
			return nil, "", common.ErrorAtPos(line, col, "unsupported pattern %#v", c.Pattern)
		}
	}

	// Chain from last to first to build nested if-else.
	//   if cond1 { body1 } else { if cond2 { body2 } else { wildcardBody } }
	var tail jen.Code
	for i := len(trans) - 1; i >= 0; i-- {
		t := trans[i]
		if t.isVariant {
			if tail == nil {
				// This variant case is the final branch in the chain.
				// If expression form and no wildcard follows, add panic default.
				if expected != "" && !lastIsWildcard {
					tail = t.ifStmt.Else().Block(jen.Panic(jen.Lit("unreachable")))
				} else {
					tail = t.ifStmt
				}
			} else {
				tail = t.ifStmt.Else().Block(tail)
			}
		} else {
			// Wildcard: the else body itself (innermost branch ends here)
			tail = t.elseBody
		}
	}

	// Wrap in an immediately-invoked function literal for expression form
	expr := jen.Func().Params()
	if expected != "" {
		expr.Add(jen.Id(expected))
	}
	expr = expr.Block(tail).Call()

	return expr, expected, nil
}

func (g *generator) translateWhile(n *WhileExpr, ctx *exprCtx) (jen.Code, string, error) {
	cond, _, err := g.translateExpr(n.Cond, ctx, "bool")
	if err != nil {
		return nil, "", err
	}

	// Build for loop body using Jennifer
	forBlock := jen.BlockFunc(func(b *jen.Group) {
		child := ctx.child()
		switch body := n.Body.(type) {
		case *BlockExpr:
			for _, stmt := range body.Stmts {
				switch s := stmt.(type) {
				case *ExprStmt:
					code, typ, err := g.translateExpr(s.Expr, child, "")
					if err != nil {
						return
					}
					if stmtIsStatementSafe(s.Expr) || typ == "" {
						b.Add(code)
					} else {
						b.Add(jen.Id("_").Op("=").Add(code))
					}
				case *LetStmt:
					code, typ, err := g.translateExpr(s.Value, child, g.goType(s.Type, child.typeParams))
					if err != nil {
						return
					}
					if s.Name == "_" {
						if stmtIsStatementSafe(s.Value) {
							b.Add(code)
						} else {
							b.Add(jen.Id("_").Op("=").Add(code))
						}
					} else {
						actualName := g.bindLocal(child, s.Name, typ, s.Mutable)
						if s.Type != nil {
							b.Add(jen.Var().Id(actualName).Add(jenTypeExpr(s.Type)).Op("=").Add(code))
						} else {
							b.Add(jen.Id(actualName).Op(":=").Add(code))
						}
					}
				case *AssignStmt:
					actualName, ok := child.bindings[s.Name]
					if !ok {
						return
					}
					if !child.mutable[actualName] {
						return
					}
					targetType := child.locals[s.Name]
					code, _, err := g.translateExpr(s.Value, child, targetType)
					if err != nil {
						return
					}
					b.Add(jen.Id(actualName).Op("=").Add(code))
				default:
					// Return error - this will be handled by the caller
					b.Add(jen.Id("panic").Call(jen.Lit("unsupported statement")))
				}
			}
		default:
			code, typ, err := g.translateExpr(body, child, "")
			if err != nil {
				return
			}
			if stmtIsStatementSafe(body) || typ == "" {
				b.Add(code)
			} else {
				b.Add(jen.Id("_").Op("=").Add(code))
			}
		}
	})

	// Build the for loop and immediately call it
	forLoop := jen.Func().Params().Block(
		jen.For(cond).Block(forBlock),
	).Call()

	return forLoop, "", nil
}

func (g *generator) translatePattern(p Pattern, enum *EnumDecl, enumArgs []string, switchVar string, body Expr) (string, map[string]bindingInfo, error) {
	switch pat := p.(type) {
	case *WildcardPattern:
		return "interface{}", nil, nil
	case *VariantPattern:
		variant := g.findVariant(enum, pat.Name)
		if variant == nil {
			return "", nil, common.ErrorAtPos(pat.Line, pat.Column, "unknown variant %s of %s", pat.Name, enum.Name)
		}
		tname := variantGoTypeName(enum.Name, variant.Name)
		if len(enumArgs) > 0 {
			tname += "[" + strings.Join(enumArgs, ", ") + "]"
		}
		bindings := map[string]bindingInfo{}
		for i, arg := range pat.Args {
			if i >= len(variant.Fields) {
				return "", nil, common.ErrorAtPos(pat.Line, pat.Column, "pattern %s arity mismatch", pat.Name)
			}
			if !exprUsesIdent(body, arg) {
				continue
			}
			bindings[arg] = bindingInfo{
				Expr: fmt.Sprintf("%s.F%d", switchVar, i),
				Type: g.goType(variant.Fields[i].Type, nil),
			}
		}
		return tname, bindings, nil
	default:
		line, col := common.NodePos(p)
		return "", nil, common.ErrorAtPos(line, col, "unsupported pattern %#v", p)
	}
}
