package compiler

import (
	"fmt"
	"strings"

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

func (g *generator) translateBlock(n *BlockExpr, ctx *exprCtx, expected string) (string, string, error) {
	var b strings.Builder
	b.WriteString("func()")
	if expected != "" {
		b.WriteString(" ")
		b.WriteString(expected)
	}
	b.WriteString(" {\n")
	child := ctx.child()
	var lastWasExprStmt bool
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
				return "", "", err
			}
			if isLast && expected != "" {
				if typ == "" {
					line, col := common.NodePos(s)
					return "", "", common.ErrorAtPos(line, col, "block must end with an expression returning %s", expected)
				}
				b.WriteString("\treturn ")
				b.WriteString(code)
				b.WriteString("\n")
				continue
			}
			b.WriteString("\t")
			if stmtIsStatementSafe(s.Expr) {
				b.WriteString(code)
			} else {
				b.WriteString("_ = ")
				b.WriteString(code)
			}
			b.WriteString("\n")
		case *LetStmt:
			lastWasExprStmt = false
			code, typ, err := g.translateExpr(s.Value, child, g.goType(s.Type, child.typeParams))
			if err != nil {
				return "", "", err
			}
			b.WriteString("\t")
			if s.Name == "_" {
				if stmtIsStatementSafe(s.Value) {
					b.WriteString(code)
				} else {
					b.WriteString("_ = ")
					b.WriteString(code)
				}
			} else {
				actualName := g.bindLocal(child, s.Name, typ, s.Mutable)
				bindType := typ
				if s.Type != nil {
					bindType = g.goType(s.Type, child.typeParams)
					b.WriteString("var ")
					b.WriteString(actualName)
					b.WriteString(" ")
					b.WriteString(bindType)
					b.WriteString(" = ")
					b.WriteString(code)
				} else {
					b.WriteString(actualName)
					b.WriteString(" := ")
					b.WriteString(code)
				}
				child.locals[s.Name] = bindType
				child.sourceTypes[s.Name] = bindType
				child.bindings[s.Name] = actualName
			}
			b.WriteString("\n")
		case *AssignStmt:
			lastWasExprStmt = false
			actualName, ok := child.bindings[s.Name]
			if !ok {
				return "", "", common.ErrorAtPos(s.Line, s.Column, "unknown binding %q", s.Name)
			}
			if !child.mutable[actualName] {
				return "", "", common.ErrorAtPos(s.Line, s.Column, "cannot assign to immutable binding %q", s.Name)
			}
			targetType := child.locals[s.Name]
			code, _, err := g.translateExpr(s.Value, child, targetType)
			if err != nil {
				return "", "", err
			}
			b.WriteString("\t")
			b.WriteString(actualName)
			b.WriteString(" = ")
			b.WriteString(code)
			b.WriteString("\n")
		default:
			lastWasExprStmt = false
			line, col := common.NodePos(stmt)
			return "", "", common.ErrorAtPos(line, col, "unsupported statement %#v", stmt)
		}
	}
	if expected != "" && !lastWasExprStmt {
		line, col := common.NodePos(n)
		return "", "", common.ErrorAtPos(line, col, "block must end with an expression returning %s", expected)
	}
	b.WriteString("}()")
	if expected != "" {
		return b.String(), expected, nil
	}
	return b.String(), "", nil
}

func (g *generator) translateFuncLit(n *FuncLitExpr, outer *exprCtx) (string, string, error) {
	retType := g.goReturnType(n.Ret, outer.typeParams)
	var b strings.Builder
	b.WriteString("func(")
	child := outer.child()
	child.retType = retType
	for i, p := range n.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		tp := g.goType(p.Type, outer.typeParams)
		child.locals[p.Name] = tp
		b.WriteString(p.Name)
		b.WriteString(" ")
		b.WriteString(tp)
	}
	b.WriteString(")")
	if retType != "" {
		b.WriteString(" ")
		b.WriteString(retType)
	}
	b.WriteString(" {\n")
	body, bodyType, err := g.translateExpr(n.Body, child, retType)
	if err != nil {
		return "", "", err
	}
	if retType == "" {
		g.writeUnitBody(&b, body, bodyType)
	} else {
		b.WriteString("\treturn ")
		b.WriteString(body)
		b.WriteString("\n")
	}
	b.WriteString("}")
	return b.String(), retType, nil
}

func (g *generator) translateIf(n *IfExpr, ctx *exprCtx, expected string) (string, string, error) {
	cond, _, err := g.translateExpr(n.Cond, ctx, "")
	if err != nil {
		return "", "", err
	}
	thenCtx := ctx.child()
	elseCtx := ctx.child()
	thenCode, thenType, err := g.translateExpr(n.Then, thenCtx, expected)
	if err != nil {
		return "", "", err
	}
	elseCode, elseType, err := g.translateExpr(n.Else, elseCtx, expected)
	if err != nil {
		return "", "", err
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
	var b strings.Builder
	b.WriteString("func()")
	if resultType != "" {
		b.WriteString(" ")
		b.WriteString(resultType)
	}
	b.WriteString(" {\n")
	b.WriteString("\tif ")
	b.WriteString(cond)
	b.WriteString(" {\n")
	if resultType == "" {
		g.writeUnitBody(&b, thenCode, thenType)
	} else {
		b.WriteString("\t\treturn ")
		b.WriteString(thenCode)
		b.WriteString("\n")
	}
	b.WriteString("\t} else {\n")
	if resultType == "" {
		g.writeUnitBody(&b, elseCode, elseType)
	} else {
		b.WriteString("\t\treturn ")
		b.WriteString(elseCode)
		b.WriteString("\n")
	}
	b.WriteString("\t}\n")
	b.WriteString("}()")
	return b.String(), resultType, nil
}

func (g *generator) translateSwitch(n *SwitchExpr, ctx *exprCtx, expected string) (string, string, error) {
	targetCode, targetType, err := g.translateExpr(n.Target, ctx, "")
	if err != nil {
		return "", "", err
	}
	enumName, enumArgs := splitTypeArgs(targetType)
	enumDecl := g.pkg.Enums[enumName]
	if enumDecl == nil {
		return "", "", common.ErrorAtPos(n.Line, n.Column, "switch target %q is not an enum", targetType)
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
	var b strings.Builder
	b.WriteString("func()")
	if expected != "" {
		b.WriteString(" ")
		b.WriteString(expected)
	}
	b.WriteString(" {\n")
	if needsSwitchVar {
		b.WriteString("\tswitch v := ")
		b.WriteString(targetCode)
		b.WriteString(".(type) {\n")
	} else {
		b.WriteString("\tswitch ")
		b.WriteString(targetCode)
		b.WriteString(".(type) {\n")
	}
	for _, c := range n.Cases {
		pat, bindings, err := g.translatePattern(c.Pattern, enumDecl, enumArgs, "v", c.Body)
		if err != nil {
			return "", "", err
		}
		b.WriteString("\tcase ")
		b.WriteString(pat)
		b.WriteString(":\n")
		child := ctx.child()
		for name, info := range bindings {
			child.locals[name] = info.Type
			child.bindings[name] = info.Expr
		}
		body, bodyType, err := g.translateExpr(c.Body, child, expected)
		if err != nil {
			return "", "", err
		}
		if expected == "" {
			b.WriteString("\t\t")
			if bodyType == "" {
				b.WriteString(body)
			} else {
				b.WriteString("_ = ")
				b.WriteString(body)
			}
			b.WriteString("\n")
		} else {
			b.WriteString("\t\treturn ")
			b.WriteString(body)
			b.WriteString("\n")
		}
	}
	if expected == "" {
		b.WriteString("\t}\n}()")
	} else {
		b.WriteString("\t}\n\tpanic(\"unreachable\")\n}()")
	}
	return b.String(), expected, nil
}

func (g *generator) translateWhile(n *WhileExpr, ctx *exprCtx) (string, string, error) {
	cond, _, err := g.translateExpr(n.Cond, ctx, "bool")
	if err != nil {
		return "", "", err
	}
	var b strings.Builder
	b.WriteString("func() {\n")
	b.WriteString("\tfor ")
	b.WriteString(cond)
	b.WriteString(" {\n")
	child := ctx.child()
	switch body := n.Body.(type) {
	case *BlockExpr:
		for _, stmt := range body.Stmts {
			switch s := stmt.(type) {
			case *ExprStmt:
				code, typ, err := g.translateExpr(s.Expr, child, "")
				if err != nil {
					return "", "", err
				}
				b.WriteString("\t\t")
				if stmtIsStatementSafe(s.Expr) || typ == "" {
					b.WriteString(code)
				} else {
					b.WriteString("_ = ")
					b.WriteString(code)
				}
				b.WriteString("\n")
			case *LetStmt:
				code, typ, err := g.translateExpr(s.Value, child, g.goType(s.Type, child.typeParams))
				if err != nil {
					return "", "", err
				}
				b.WriteString("\t\t")
				if s.Name == "_" {
					if stmtIsStatementSafe(s.Value) {
						b.WriteString(code)
					} else {
						b.WriteString("_ = ")
						b.WriteString(code)
					}
				} else {
					actualName := g.bindLocal(child, s.Name, typ, s.Mutable)
					if s.Type != nil {
						b.WriteString("var ")
						b.WriteString(actualName)
						b.WriteString(" ")
						b.WriteString(g.goType(s.Type, child.typeParams))
						b.WriteString(" = ")
						b.WriteString(code)
					} else {
						b.WriteString(actualName)
						b.WriteString(" := ")
						b.WriteString(code)
					}
				}
				b.WriteString("\n")
			case *AssignStmt:
				actualName, ok := child.bindings[s.Name]
				if !ok {
					return "", "", common.ErrorAtPos(s.Line, s.Column, "unknown binding %q", s.Name)
				}
				if !child.mutable[actualName] {
					return "", "", common.ErrorAtPos(s.Line, s.Column, "cannot assign to immutable binding %q", s.Name)
				}
				targetType := child.locals[s.Name]
				code, _, err := g.translateExpr(s.Value, child, targetType)
				if err != nil {
					return "", "", err
				}
				b.WriteString("\t\t")
				b.WriteString(actualName)
				b.WriteString(" = ")
				b.WriteString(code)
				b.WriteString("\n")
			default:
				line, col := common.NodePos(stmt)
				return "", "", common.ErrorAtPos(line, col, "unsupported statement %#v", stmt)
			}
		}
	default:
		code, typ, err := g.translateExpr(body, child, "")
		if err != nil {
			return "", "", err
		}
		b.WriteString("\t\t")
		if stmtIsStatementSafe(body) || typ == "" {
			b.WriteString(code)
		} else {
			b.WriteString("_ = ")
			b.WriteString(code)
		}
		b.WriteString("\n")
	}
	b.WriteString("\t}\n")
	b.WriteString("}()")
	return b.String(), "", nil
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
