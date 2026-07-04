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
	b := jen.Func().Params()
	if expected != "" {
		b.Add(jenTypeExpr(&NamedType{Name: expected}))
	}
	child := ctx.child()
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
					return nil, "", common.ErrorAtPos(line, col, "block must end with an expression returning %s", expected)
				}
				b = b.Block(jen.Return(code))
				continue
			}
			if stmtIsStatementSafe(s.Expr) || typ == "" {
				b = b.Block(code)
			} else {
				b = b.Block(jen.Id("_"), jen.Op("=").Add(code))
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
				b = b.Block(jen.Return(code))
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
					b = b.Block(code)
				} else {
					b = b.Block(jen.Id("_"), jen.Op("=").Add(code))
				}
			} else {
				actualName := g.bindLocal(child, s.Name, typ, s.Mutable)
				bindType := typ
				if s.Type != nil {
					bindType = g.goType(s.Type, child.typeParams)
					b = b.Block(jen.Var().Id(actualName).Add(jenTypeExpr(&NamedType{Name: bindType})).Op("=").Add(code))
				} else {
					b = b.Block(jen.Id(actualName), jen.Op(":=").Add(code))
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
			b = b.Block(jen.Id(actualName), jen.Op("=").Add(code))
		default:
			lastWasExprStmt = false
			line, col := common.NodePos(stmt)
			return nil, "", common.ErrorAtPos(line, col, "unsupported statement %#v", stmt)
		}
	}
	if expected != "" && !lastWasExprStmt && !sawReturn {
		line, col := common.NodePos(n)
		return nil, "", common.ErrorAtPos(line, col, "block must end with an expression returning %s", expected)
	}
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
	b := jen.Func().Params()
	if resultType != "" {
		b.Add(jenTypeExpr(&NamedType{Name: resultType}))
	}
	if resultType == "" {
		b = b.Block(jen.If(cond).Block(thenCode))
	} else {
		b = b.Block(jen.If(cond).Block(jen.Return(thenCode)))
	}
	if resultType == "" {
		b = b.Block(jen.If(cond).Block(elseCode))
	} else {
		b = b.Block(jen.If(cond).Block(jen.Return(elseCode)))
	}
	_, _, _ = thenType, elseType, cond
	return b, resultType, nil
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
	needsSwitchVar := false
	for _, c := range n.Cases {
		if pat, ok := c.Pattern.(*VariantPattern); ok {
			for _, arg := range pat.Args {
				if exprUsesIdent(c.Body, arg) {
					needsSwitchVar = true
					break
				}
			}
			if needsSwitchVar {
				break
			}
		}
	}

	// Build switch statement body using Jennifer
	switchBlock := jen.BlockFunc(func(b *jen.Group) {
		if needsSwitchVar {
			b.Add(jen.Id("v").Op(":=").Add(targetCode).Op(".").Id("type"))
		} else {
			b.Add(jen.Switch().Add(targetCode).Op(".").Id("type"))
		}
		for _, c := range n.Cases {
			pat, bindings, err := g.translatePattern(c.Pattern, enumDecl, enumArgs, "v", c.Body)
			if err != nil {
				return
			}
			child := ctx.child()
			for name, info := range bindings {
				child.locals[name] = info.Type
				child.bindings[name] = info.Expr
			}
			body, bodyType, err := g.translateExpr(c.Body, child, expected)
			if err != nil {
				return
			}
			caseBlock := jen.BlockFunc(func(cb *jen.Group) {
				if expected == "" {
					if bodyType == "" {
						cb.Add(body)
					} else {
						cb.Add(jen.Id("_").Op("=").Add(body))
					}
				} else {
					cb.Add(jen.Return(body))
				}
			})
			b.Add(jen.Case(jen.Id(pat)).Add(caseBlock))
		}
		if expected != "" {
			b.Add(jen.Id("panic").Call(jen.Lit("unreachable")))
		}
	})

	// Build the full switch expression
	var switchExpr jen.Code
	if needsSwitchVar {
		switchExpr = jen.Func().Params().Block(
			jen.Switch().Id("v").Block(switchBlock),
		)
	} else {
		switchExpr = jen.Func().Params().Block(
			jen.Switch().Add(targetCode).Block(switchBlock),
		)
	}

	return switchExpr, expected, nil
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

	// Build the for loop
	forLoop := jen.Func().Params().Block(
		jen.For(jen.Id("_").Op(":=").Add(cond)).Block(forBlock),
	)

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
