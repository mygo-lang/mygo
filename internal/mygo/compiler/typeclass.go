package compiler

import (
	"fmt"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func typeclassMatchScore(args []TypeExpr, scopeTypes map[string]struct{}) matchScore {
	var score matchScore
	for _, arg := range args {
		score = score.add(typeMatchScore(arg, scopeTypes))
	}
	return score
}

func typeMatchScore(t TypeExpr, scopeTypes map[string]struct{}) matchScore {
	switch tt := t.(type) {
	case *NamedType:
		if scopeTypes != nil {
			if _, ok := scopeTypes[tt.Name]; ok && len(tt.Args) == 0 {
				return matchScore{TypeParams: 1}
			}
		}
		score := matchScore{ConcreteTypes: 1}
		for _, a := range tt.Args {
			score = score.add(typeMatchScore(a, scopeTypes))
		}
		return score
	case *FuncType:
		score := matchScore{ConcreteTypes: 1}
		for _, p := range tt.Params {
			score = score.add(typeMatchScore(p, scopeTypes))
		}
		score = score.add(typeMatchScore(tt.Ret, scopeTypes))
		return score
	default:
		return matchScore{AnyTypes: 1}
	}
}

func (m matchScore) add(other matchScore) matchScore {
	m.ConcreteTypes += other.ConcreteTypes
	m.TypeParams += other.TypeParams
	m.AnyTypes += other.AnyTypes
	return m
}

func betterMatch(a, b matchScore) bool {
	if a.ConcreteTypes != b.ConcreteTypes {
		return a.ConcreteTypes > b.ConcreteTypes
	}
	if a.TypeParams != b.TypeParams {
		return a.TypeParams < b.TypeParams
	}
	return a.AnyTypes < b.AnyTypes
}

func sameMatch(a, b matchScore) bool {
	return a.ConcreteTypes == b.ConcreteTypes && a.TypeParams == b.TypeParams && a.AnyTypes == b.AnyTypes
}

func typeclassBindingBest(bindings []typeclassBinding) typeclassBinding {
	best := bindings[0]
	for _, b := range bindings[1:] {
		if betterMatch(b.Score, best.Score) {
			best = b
		}
	}
	return best
}

func typeclassFuncType(paramTypes []string, retType string) string {
	if len(paramTypes) == 0 {
		if retType == "" {
			return "func()"
		}
		return "func() " + retType
	}
	fn := "func(" + strings.Join(paramTypes, ", ") + ")"
	if retType != "" {
		fn += " " + retType
	}
	return fn
}

func (g *generator) findVariant(enum *EnumDecl, name string) *EnumVariant {
	for i := range enum.Variants {
		if enum.Variants[i].Name == name {
			return &enum.Variants[i]
		}
	}
	return nil
}

func (g *generator) translateAnyFuncCall(name string, args []Expr, ctx *exprCtx) (string, string, bool, error) {
	sourceType, ok := ctx.sourceTypes[name]
	if !ok || !strings.HasPrefix(strings.TrimSpace(sourceType), "func(") {
		return "", "", false, nil
	}
	_, ret := splitFuncType(sourceType)
	var argCodes []string
	for _, a := range args {
		code, _, err := g.translateExpr(a, ctx, "")
		if err != nil {
			return "", "", false, err
		}
		argCodes = append(argCodes, codeString(code))
	}
	actualName := name
	if bound, ok := ctx.bindings[name]; ok {
		actualName = bound
	}
	return fmt.Sprintf("%s.(%s)(%s)", actualName, sourceType, strings.Join(argCodes, ", ")), ret, true, nil
}

func (g *generator) translateEnumConstructor(enumName, name string, args []Expr, ctx *exprCtx, expected string) (string, string, error) {
	expectedEnum, enumArgs := splitTypeArgs(expected)
	if enumName == "" {
		enumName = expectedEnum
	}
	if enumName == "" {
		enumName = name
	}
	var typeArgs []string
	if len(enumArgs) > 0 {
		typeArgs = enumArgs
	}
	if len(typeArgs) == 0 {
		if _, ok := g.pkg.Enums[enumName]; ok && expected != "" {
			_, typeArgs = splitTypeArgs(expected)
		}
	}
	var variant *EnumVariant
	if enumDecl := g.pkg.Enums[enumName]; enumDecl != nil {
		variant = g.findVariant(enumDecl, name)
	}
	subst := map[string]string{}
	if enumDecl := g.pkg.Enums[enumName]; enumDecl != nil {
		for i, tp := range enumDecl.TypeParams {
			if i < len(typeArgs) {
				subst[tp] = typeArgs[i]
			}
		}
	}
	var argCodes []string
	for i, a := range args {
		argExpected := ""
		if variant != nil && i < len(variant.Fields) {
			argExpected = typeString(variant.Fields[i].Type, subst)
		}
		code, _, err := g.translateExpr(a, ctx, argExpected)
		if err != nil {
			return "", "", err
		}
		argCodes = append(argCodes, codeString(code))
	}
	typeArgStr := ""
	if len(typeArgs) > 0 {
		typeArgStr = "[" + strings.Join(typeArgs, ", ") + "]"
	}
	return fmt.Sprintf("%s%s(%s)", sanitizeIdent(name), typeArgStr, strings.Join(argCodes, ", ")), expected, nil
}

func (g *generator) translateTypeclassCall(name string, args []Expr, ctx *exprCtx, expected string) (string, string, bool) {
	if ifaceName, ok := g.resolveTypeclassInterface(name, ctx); ok {
		methodIface := g.pkg.Interfaces[ifaceName]
		if methodIface == nil {
			return "", "", false
		}
		if funcName, ok := ctx.constraintFuncs[name]; ok {
			var argCodes []string
			for _, a := range args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return "", "", false
				}
				argCodes = append(argCodes, codeString(code))
			}
			return fmt.Sprintf("%s(%s)", funcName, strings.Join(argCodes, ", ")), methodReturnType(methodIface, name), true
		}
		if len(args) == 0 {
			return "", "", false
		}
		var argCodes []string
		for _, a := range args {
			code, _, err := g.translateExpr(a, ctx, "")
			if err != nil {
				return "", "", false
			}
				argCodes = append(argCodes, codeString(code))
		}
		return fmt.Sprintf("%s(%s)", dispatchFuncName(ifaceName, name), strings.Join(argCodes, ", ")), methodReturnType(methodIface, name), true
	}
	return "", "", false
}

func (g *generator) translateIdent(name string, line, col int, ctx *exprCtx, expected string) (string, string, error) {
	if expr, ok := ctx.bindings[name]; ok {
		return expr, ctx.locals[name], nil
	}
	if typ, ok := ctx.locals[name]; ok {
		if typ == "any" {
			if sourceType, ok := ctx.sourceTypes[name]; ok && sourceType != "" && sourceType != "any" {
				return fmt.Sprintf("%s.(%s)", name, sourceType), sourceType, nil
			}
		}
		return name, typ, nil
	}
	switch name {
	case "true", "false":
		return name, "bool", nil
	case "None":
		base, args := splitTypeArgs(expected)
		if base != "" && len(args) > 0 {
			return fmt.Sprintf("None[%s]()", strings.Join(args, ", ")), expected, nil
		}
		return "", "", common.ErrorAtPos(line, col, "None requires type inference from context")
	case "Nil":
		return "", "", common.ErrorAtPos(line, col, "Nil is not a valid value; use Option[Ref[T]] for nullable references")
	}
	if enumName, ok := g.variantByName[name]; ok {
		return g.translateEnumConstructor(enumName, name, nil, ctx, expected)
	}
	if typeclassHelper, typ, ok := g.translateTypeclassIdent(name, ctx, expected); ok {
		return typeclassHelper, typ, nil
	}
	return name, ctx.locals[name], nil
}

func (g *generator) translateTypeclassIdent(name string, ctx *exprCtx, expected string) (string, string, bool) {
	if ifaceName, ok := g.resolveTypeclassInterface(name, ctx); ok {
		if funcName, ok := ctx.constraintFuncs[name]; ok {
			return funcName, expected, true
		}
		return dispatchFuncName(ifaceName, name), expected, true
	}
	return "", "", false
}

func (g *generator) resolveTypeclassInterface(name string, ctx *exprCtx) (string, bool) {
	if bindings, ok := ctx.typeclassMethods[name]; ok && len(bindings) > 0 {
		return typeclassBindingBest(bindings).Interface, true
	}
	if ifaceName, ok := g.interfaceByMethod[name]; ok {
		return ifaceName, true
	}
	return "", false
}

func (g *generator) hasHelper(method, typ string) bool {
	_ = method
	_ = typ
	return true
}
