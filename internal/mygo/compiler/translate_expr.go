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
	case *SliceLitExpr:
		return g.translateSliceLit(n, ctx, expected)
	case *MapLitExpr:
		return g.translateMapLit(n, ctx, expected)
	case *SetLitExpr:
		return g.translateSetLit(n, ctx, expected)
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

func (g *generator) translateSliceLit(n *SliceLitExpr, ctx *exprCtx, expected string) (string, string, error) {
	// 1. If `expected` is a Go slice type (starts with "[]"), try to infer element type from it.
	// 2. Infer element type from actual elements.
	// 3. If both exist, unify them and error on mismatch.
	// 4. If neither, error.

	var fromExpected string
	if strings.HasPrefix(expected, "[]") {
		fromExpected = expected[2:]
	}

	// Infer element type from each element expression
	var inferredTypes []string
	for _, elem := range n.Elems {
		_, typ, err := g.translateExpr(elem, ctx, "")
		if err != nil {
			return "", "", err
		}
		if typ != "" && typ != "any" {
			inferredTypes = append(inferredTypes, typ)
		}
	}

	// Find common type among inferred types
	elemType := ""
	if len(inferredTypes) > 0 {
		elemType = inferredTypes[0]
		for _, t := range inferredTypes[1:] {
			if t != elemType {
				line, col := common.NodePos(n)
				return "", "", common.ErrorAtPos(line, col, "slice element types inconsistent: %s and %s", elemType, t)
			}
		}
	}

	// Resolve final element type
	if fromExpected != "" && elemType != "" {
		// Unify: expected type takes precedence if it's a named primitive, otherwise check compatibility
		// For simplicity, use the element type from inference if compatible, else prefer expected
		// Both "int" and "int" should match; "int" and "int64" should error
		if fromExpected != elemType {
			// Check if they are the same primitive (e.g., both are "int" vs "Int")
			// For now, accept either if one is generic "any"
			if elemType == "any" {
				elemType = fromExpected
			} else if fromExpected == "any" {
				// keep inferred
			} else {
				// Type mismatch — use expected (annotation is more authoritative)
				elemType = fromExpected
			}
		}
	} else if fromExpected != "" {
		elemType = fromExpected
	}

	if elemType == "" {
		return "", "", common.ErrorAtPos(n.Line, n.Column, "could not infer slice element type")
	}

	// Translate all elements with the resolved element type
	var parts []string
	for _, elem := range n.Elems {
		code, _, err := g.translateExpr(elem, ctx, elemType)
		if err != nil {
			return "", "", err
		}
		parts = append(parts, code)
	}

	goElemType := g.goType(&NamedType{Name: elemType}, nil)
	sliceType := "[]" + goElemType
	return sliceType + "{" + strings.Join(parts, ", ") + "}", sliceType, nil
}

func (g *generator) translateMapLit(n *MapLitExpr, ctx *exprCtx, expected string) (string, string, error) {
	// Strategy:
	// 1. Parse `expected` (Go type like "map[string]int") for key/value types.
	// 2. Infer key/value types from each pair's expressions.
	// 3. Unify and error on inconsistency.

	var fromExpectedKey, fromExpectedVal string
	if key, val, ok := splitMapExpected(expected); ok {
		fromExpectedKey, fromExpectedVal = key, val
	}

	// Infer types from pairs
	var keyTypes, valTypes []string
	for _, pair := range n.Pairs {
		_, kt, err := g.translateExpr(pair.Key, ctx, "")
		if err != nil {
			return "", "", err
		}
		_, vt, err := g.translateExpr(pair.Value, ctx, "")
		if err != nil {
			return "", "", err
		}
		if kt != "" && kt != "any" {
			keyTypes = append(keyTypes, kt)
		}
		if vt != "" && vt != "any" {
			valTypes = append(valTypes, vt)
		}
	}

	keyType := ""
	if len(keyTypes) > 0 {
		keyType = keyTypes[0]
		for _, t := range keyTypes[1:] {
			if t != keyType {
				line, col := common.NodePos(n)
				return "", "", common.ErrorAtPos(line, col, "map key types inconsistent: %s and %s", keyType, t)
			}
		}
	}

	valType := ""
	if len(valTypes) > 0 {
		valType = valTypes[0]
		for _, t := range valTypes[1:] {
			if t != valType {
				line, col := common.NodePos(n)
				return "", "", common.ErrorAtPos(line, col, "map value types inconsistent: %s and %s", valType, t)
			}
		}
	}

	// Resolve from expected if inference didn't produce a type
	if keyType == "" {
		keyType = fromExpectedKey
	}
	if valType == "" {
		valType = fromExpectedVal
	}

	// Unify with expected if both exist
	if fromExpectedKey != "" && keyType != "" && fromExpectedKey != keyType && keyType != "any" {
		keyType = fromExpectedKey
	}
	if fromExpectedVal != "" && valType != "" && fromExpectedVal != valType && valType != "any" {
		valType = fromExpectedVal
	}

	if keyType == "" || valType == "" {
		return "", "", common.ErrorAtPos(n.Line, n.Column, "could not infer map key/value types")
	}

	var parts []string
	for _, pair := range n.Pairs {
		keyCode, _, err := g.translateExpr(pair.Key, ctx, keyType)
		if err != nil {
			return "", "", err
		}
		valCode, _, err := g.translateExpr(pair.Value, ctx, valType)
		if err != nil {
			return "", "", err
		}
		parts = append(parts, keyCode+": "+valCode)
	}

	keyGoType := g.goType(&NamedType{Name: keyType}, nil)
	valGoType := g.goType(&NamedType{Name: valType}, nil)
	mapType := "map[" + keyGoType + "]" + valGoType
	return mapType + "{" + strings.Join(parts, ", ") + "}", mapType, nil
}

func (g *generator) translateSetLit(n *SetLitExpr, ctx *exprCtx, expected string) (string, string, error) {
	// Strategy:
	// 1. If empty and expected is not map[A]struct{}, treat as empty map.
	// 2. Infer element type from elements.
	// 3. Unify with expected if provided.

	if len(n.Elems) == 0 {
		if strings.HasSuffix(expected, "struct{") || strings.HasSuffix(expected, "struct{}") {
			// Empty set: map[A]struct{}{}
		} else {
			return g.translateEmptyMapLit(ctx, expected, n.Line, n.Col)
		}
	}

	// Infer element type from elements
	var inferredTypes []string
	for _, elem := range n.Elems {
		_, typ, err := g.translateExpr(elem, ctx, "")
		if err != nil {
			return "", "", err
		}
		if typ != "" && typ != "any" {
			inferredTypes = append(inferredTypes, typ)
		}
	}

	elemType := ""
	if len(inferredTypes) > 0 {
		elemType = inferredTypes[0]
		for _, t := range inferredTypes[1:] {
			if t != elemType {
				line, col := common.NodePos(n)
				return "", "", common.ErrorAtPos(line, col, "set element types inconsistent: %s and %s", elemType, t)
			}
		}
	}

	// Resolve from expected
	if elemType == "" {
		if strings.HasPrefix(expected, "map[") {
			if key, val, ok := splitMapExpected(expected); ok && val == "struct{}" {
				elemType = key
			}
		}
	} else if elemType != "any" {
		// Unify with expected if both exist
		if strings.HasPrefix(expected, "map[") {
			if key, val, ok := splitMapExpected(expected); ok && val == "struct{}" && key != elemType {
				elemType = key
			}
		}
	}

	if elemType == "" {
		return "", "", common.ErrorAtPos(n.Line, n.Col, "could not infer set element type")
	}

	var parts []string
	for _, elem := range n.Elems {
		code, _, err := g.translateExpr(elem, ctx, elemType)
		if err != nil {
			return "", "", err
		}
		parts = append(parts, code)
	}

	elemGoType := g.goType(&NamedType{Name: elemType}, nil)
	setType := "map[" + elemGoType + "]struct{}"
	partsWithBlank := make([]string, len(parts))
	for i, p := range parts {
		partsWithBlank[i] = p + ":{}"
	}
	return setType + "{" + strings.Join(partsWithBlank, ", ") + "}", setType, nil
}

func (g *generator) translateEmptyMapLit(ctx *exprCtx, expected string, line, col int) (string, string, error) {
	// expected is like "map[string]int"
	keyType, valType, ok := splitMapExpected(expected)
	if !ok {
		keyType = ""
		valType = ""
	}
	if keyType == "" || valType == "" {
		return "", "", common.ErrorAtPos(line, col, "empty map literal requires a known map type")
	}
	keyGoType := g.goType(&NamedType{Name: keyType}, nil)
	valGoType := g.goType(&NamedType{Name: valType}, nil)
	mapType := "map[" + keyGoType + "]" + valGoType
	return mapType + "{}", mapType, nil
}

// splitTopLevelSingle splits a top-level comma-separated string into exactly two parts.
func splitTopLevelSingle(s string) (string, string) {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[', '(', '<':
			depth++
		case ']', ')', '>':
			depth--
		case ',':
			if depth == 0 {
				return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:])
			}
		}
	}
	return "", ""
}

func splitMapExpected(expected string) (string, string, bool) {
	expected = strings.TrimSpace(expected)
	if !strings.HasPrefix(expected, "map[") {
		return "", "", false
	}
	inner := expected[4:]
	depth := 0
	for i := 0; i < len(inner); i++ {
		switch inner[i] {
		case '[', '(', '<':
			depth++
		case ']', ')', '>':
			if depth == 0 {
				return "", "", false
			}
			if depth == 1 {
				key := strings.TrimSpace(inner[1:i])
				val := strings.TrimSpace(inner[i+1:])
				if key == "" || val == "" {
					return "", "", false
				}
				return key, val, true
			}
			depth--
		}
	}
	return "", "", false
}

func (g *generator) isTypeParamName(name string, ctx *exprCtx) bool {
	if ctx == nil {
		return false
	}
	_, ok := ctx.typeParams[name]
	return ok
}
