package compiler

import (
	"fmt"
	"strconv"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func (g *generator) translateExpr(e Expr, ctx *exprCtx, expected string) (string, string, error) {
	switch n := e.(type) {
	case *IdentExpr:
		return g.translateIdent(n.Name, ctx, expected)
	case *LiteralExpr:
		switch n.Kind {
		case "number":
			switch expected {
			case "int", "int64", "float64":
				return n.Value, expected, nil
			}
			if strings.Contains(n.Value, ".") {
				return n.Value, "float64", nil
			}
			return n.Value, "int", nil
		case "string":
			return strconv.Quote(n.Value), "string", nil
		}
	case *BinaryExpr:
		if n.Op == "|>" {
			left, _, err := g.translateExpr(n.Left, ctx, "")
			if err != nil {
				return "", "", err
			}
			switch right := n.Right.(type) {
			case *CallExpr:
				callee, _, err := g.translateExpr(right.Callee, ctx, "")
				if err != nil {
					return "", "", err
				}
				args := make([]string, 0, len(right.Args)+1)
				for _, a := range right.Args {
					code, _, err := g.translateExpr(a, ctx, "")
					if err != nil {
						return "", "", err
					}
					args = append(args, code)
				}
				args = append(args, left)
				_, rt, err := g.translateExpr(right, ctx, "")
				if err != nil {
					return "", "", err
				}
				return fmt.Sprintf("%s(%s)", callee, strings.Join(args, ", ")), rt, nil
			default:
				rhs, rt, err := g.translateExpr(n.Right, ctx, "")
				if err != nil {
					return "", "", err
				}
				return fmt.Sprintf("%s(%s)", rhs, left), rt, nil
			}
		}
		if n.Op == "<|" {
			right, _, err := g.translateExpr(n.Right, ctx, "")
			if err != nil {
				return "", "", err
			}
			switch left := n.Left.(type) {
			case *CallExpr:
				callee, _, err := g.translateExpr(left.Callee, ctx, "")
				if err != nil {
					return "", "", err
				}
				args := make([]string, 0, len(left.Args)+1)
				for _, a := range left.Args {
					code, _, err := g.translateExpr(a, ctx, "")
					if err != nil {
						return "", "", err
					}
					args = append(args, code)
				}
				args = append(args, right)
				_, lt, err := g.translateExpr(left, ctx, "")
				if err != nil {
					return "", "", err
				}
				return fmt.Sprintf("%s(%s)", callee, strings.Join(args, ", ")), lt, nil
			default:
				lhs, lt, err := g.translateExpr(n.Left, ctx, "")
				if err != nil {
					return "", "", err
				}
				return fmt.Sprintf("%s(%s)", lhs, right), lt, nil
			}
		}
		left, lt, err := g.translateExpr(n.Left, ctx, "")
		if err != nil {
			return "", "", err
		}
		rightExpected := ""
		if lt != "" && lt != "any" {
			rightExpected = lt
		}
		right, rt, err := g.translateExpr(n.Right, ctx, rightExpected)
		if err != nil {
			return "", "", err
		}
		switch n.Op {
		case "+", "-", "*", "/":
			resType := lt
			if resType == "" || resType == "any" {
				resType = rt
			}
			return fmt.Sprintf("(%s %s %s)", left, n.Op, right), resType, nil
		case "&&", "||":
			if lt != "" && lt != "bool" {
				line, col := common.NodePos(n.Left)
				return "", "", common.ErrorAtPos(line, col, "logical operator %q requires Bool operands, got %s", n.Op, lt)
			}
			if rt != "" && rt != "bool" {
				line, col := common.NodePos(n.Right)
				return "", "", common.ErrorAtPos(line, col, "logical operator %q requires Bool operands, got %s", n.Op, rt)
			}
			return fmt.Sprintf("(%s %s %s)", left, n.Op, right), "bool", nil
		case "==", "!=", "<", ">", "<=", ">=":
			if err := g.ensureRelationAllowed(n, lt, rt, ctx); err != nil {
				return "", "", err
			}
			if eqExpr, ok := g.translateEqRelation(n.Op, left, right, lt, rt, ctx, expected); ok {
				return eqExpr, "bool", nil
			}
			return "", "", common.ErrorAtPos(n.Line, n.Column, "relation operator %q requires Eq-constrained operands", n.Op)
		}
	case *PrefixExpr:
		expr, typ, err := g.translateExpr(n.Expr, ctx, "")
		if err != nil {
			return "", "", err
		}
		if n.Op == "not" {
			return fmt.Sprintf("(!%s)", expr), "bool", nil
		}
		return expr, typ, nil
	case *FieldExpr:
		if baseIdent, ok := n.Expr.(*IdentExpr); ok {
			if enumDecl := g.pkg.Enums[baseIdent.Name]; enumDecl != nil {
				if variant := g.findVariant(enumDecl, n.Field); variant != nil {
					return g.translateEnumConstructor(baseIdent.Name, n.Field, nil, ctx, expected)
				}
			}
			if code, typ, ok, err := g.translateGoPackageSelector(baseIdent.Name, n.Field); err != nil {
				return "", "", err
			} else if ok {
				return code, typ, nil
			}
		}
		base, baseType, err := g.translateExpr(n.Expr, ctx, "")
		if err != nil {
			return "", "", err
		}
		if id, ok := n.Expr.(*IdentExpr); ok && g.isImportAlias(id.Name) {
			return fmt.Sprintf("%s.%s", base, n.Field), "any", nil
		}
		fieldType := g.lookupFieldType(baseType, n.Field)
		return fmt.Sprintf("%s.%s", base, exportName(n.Field)), fieldType, nil
	case *CallExpr:
		return g.translateCall(n, ctx, expected)
	case *StructLitExpr:
		return g.translateStructLit(n, ctx, expected)
	case *FuncLitExpr:
		return g.translateFuncLit(n, ctx)
	case *IfExpr:
		return g.translateIf(n, ctx, expected)
	case *SwitchExpr:
		return g.translateSwitch(n, ctx, expected)
	case *WhileExpr:
		return g.translateWhile(n, ctx)
	case *BlockExpr:
		return g.translateBlock(n, ctx, expected)
	}
	line, col := common.NodePos(e)
	return "", "", common.ErrorAtPos(line, col, "unsupported expression %#v", e)
}

func (g *generator) ensureRelationAllowed(n *BinaryExpr, leftType, rightType string, ctx *exprCtx) error {
	if leftType != "" && rightType != "" && leftType != rightType {
		return common.ErrorAtPos(n.Line, n.Column, "relation operator %q requires matching operand types, got %s and %s", n.Op, leftType, rightType)
	}
	typ := leftType
	if typ == "" {
		typ = rightType
	}
	if typ == "" {
		return common.ErrorAtPos(n.Line, n.Column, "relation operator %q requires typed operands", n.Op)
	}
	if typ == "any" {
		if _, ok := ctx.constraintFuncs["equals"]; ok {
			return nil
		}
		return common.ErrorAtPos(n.Line, n.Column, "relation operator %q requires Eq-constrained operands", n.Op)
	}
	if g.hasEqSupport(typ, ctx) {
		return nil
	}
	return common.ErrorAtPos(n.Line, n.Column, "relation operator %q requires Eq[%s]", n.Op, typ)
}

func (g *generator) translateEqRelation(op, left, right, leftType, rightType string, ctx *exprCtx, expected string) (string, bool) {
	_ = expected
	typ := leftType
	if typ == "" {
		typ = rightType
	}
	if typ == "any" || g.isTypeParamName(typ, ctx) {
		if fn := ctx.constraintFuncs["equals"]; fn != "" {
			return fmt.Sprintf("%s(%s, %s)", fn, left, right), true
		}
		return "", false
	}
	switch op {
	case "==":
		return fmt.Sprintf("(%s == %s)", left, right), true
	case "!=":
		return fmt.Sprintf("(%s != %s)", left, right), true
	case "<":
		return fmt.Sprintf("(%s < %s)", left, right), true
	case ">":
		return fmt.Sprintf("(%s > %s)", left, right), true
	case "<=":
		return fmt.Sprintf("(%s <= %s)", left, right), true
	case ">=":
		return fmt.Sprintf("(%s >= %s)", left, right), true
	default:
		return "", false
	}
}

func (g *generator) hasEqSupport(typ string, ctx *exprCtx) bool {
	if typ == "" {
		return false
	}
	if g.isTypeParamName(typ, ctx) {
		return ctx != nil && ctx.constraintFuncs["equals"] != ""
	}
	base := typ
	if idx := strings.Index(base, "["); idx >= 0 {
		base = base[:idx]
	}
	switch base {
	case "Int", "Int64", "Float64", "String", "Bool", "int", "int64", "float64", "string", "bool":
		return true
	}
	for _, impl := range g.pkg.Impls {
		if impl.Name != "Eq" {
			continue
		}
		if len(impl.TypeArgs) != 1 {
			continue
		}
		if g.goType(impl.TypeArgs[0], nil) == typ {
			return true
		}
	}
	return false
}

func (g *generator) isTypeParamName(name string, ctx *exprCtx) bool {
	if ctx == nil {
		return false
	}
	_, ok := ctx.typeParams[name]
	return ok
}
