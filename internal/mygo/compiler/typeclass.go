package compiler

import (
	"fmt"
	"strings"

	jen "github.com/dave/jennifer/jen"
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

func (g *generator) translateAnyFuncCall(name string, args []Expr, ctx *exprCtx) (jen.Code, string, bool, error) {
	sourceType, ok := ctx.sourceTypes[name]
	if !ok || !strings.HasPrefix(strings.TrimSpace(sourceType), "func(") {
		return nil, "", false, nil
	}
	_, ret := splitFuncType(sourceType)
	var argCodes []jen.Code
	for _, a := range args {
		code, _, err := g.translateExpr(a, ctx, "")
		if err != nil {
			return nil, "", false, err
		}
		argCodes = append(argCodes, code)
	}
	actualName := name
	if bound, ok := ctx.bindings[name]; ok {
		actualName = bound
	}
	return jen.Id(actualName).Assert(jen.Id(sourceType)).Call(argCodes...), ret, true, nil
}

func (g *generator) translateEnumConstructor(enumName, name string, args []Expr, ctx *exprCtx, expected string) (jen.Code, string, error) {
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
	var argCodes []jen.Code
	for i, a := range args {
		argExpected := ""
		if variant != nil && i < len(variant.Fields) {
			argExpected = typeString(variant.Fields[i].Type, subst)
		}
		code, _, err := g.translateExpr(a, ctx, argExpected)
		if err != nil {
			return nil, "", err
		}
		argCodes = append(argCodes, code)
	}
	var callee jen.Code
	if len(typeArgs) > 0 {
		typeArgCodes := make([]jen.Code, 0, len(typeArgs))
		for _, ta := range typeArgs {
			typeArgCodes = append(typeArgCodes, jen.Id(ta))
		}
		callee = jen.Id(sanitizeIdent(name)).Index(typeArgCodes...)
	} else {
		callee = jen.Id(sanitizeIdent(name))
	}
	// jen.Code is an interface, need to use Statement's Call method
	stmt := callee.(*jen.Statement)
	return stmt.Call(argCodes...), expected, nil
}

func (g *generator) translateTypeclassCall(name string, args []Expr, ctx *exprCtx, expected string) (jen.Code, string, bool) {
	if ifaceName, ok := g.resolveTypeclassInterface(name, ctx); ok {
		methodIface := g.pkg.Interfaces[ifaceName]
		if methodIface == nil {
			return nil, "", false
		}
		var argCodes []jen.Code
		var argTypes []string
		for _, a := range args {
			code, typ, err := g.translateExpr(a, ctx, "")
			if err != nil {
				return nil, "", false
			}
			argCodes = append(argCodes, code)
			argTypes = append(argTypes, typ)
		}
		if funcName, ok := ctx.constraintFuncs[name]; ok {
			return jen.Id(funcName).Call(argCodes...), methodReturnType(methodIface, name), true
		}
		if len(argCodes) > 0 {
			lastType := argTypes[len(argTypes)-1]
			if lastType != "" && strings.HasPrefix(lastType, ifaceName) {
				return argCodes[len(argCodes)-1].(*jen.Statement).Dot(name).Call(argCodes[:len(argCodes)-1]...), methodReturnType(methodIface, name), true
			}
		}
		if helper, ok := g.matchTypeclassHelper(ifaceName, name, args, ctx); ok {
			return helper, methodReturnType(methodIface, name), true
		}
	}
	return nil, "", false
}

func (g *generator) translateIdent(name string, line, col int, ctx *exprCtx, expected string) (jen.Code, string, error) {
	if expr, ok := ctx.bindings[name]; ok {
		return jen.Id(expr), ctx.locals[name], nil
	}
	if typ, ok := ctx.locals[name]; ok {
		if typ == "any" {
			if sourceType, ok := ctx.sourceTypes[name]; ok && sourceType != "" && sourceType != "any" {
				return jen.Id(name).Assert(jen.Id(sourceType)), sourceType, nil
			}
		}
		return jen.Id(name), typ, nil
	}
	switch name {
	case "true", "false":
		return jen.Lit(name == "true"), "bool", nil
	case "None":
		base, args := splitTypeArgs(expected)
		if base != "" && len(args) > 0 {
			var typeArgs []jen.Code
			for _, a := range args {
				cleanArg := strings.TrimPrefix(strings.TrimSpace(a), "*")
				typeArgs = append(typeArgs, jen.Id(cleanArg))
			}
			return jen.Id("None").Index(typeArgs...).Call(), expected, nil
		}
		fmt.Printf("DEBUG None: expected=%q base=%q args=%v line=%d\n", expected, base, args, line)
		return nil, "", common.ErrorAtPos(line, col, "None requires type inference from context")
	case "Nil":
		return nil, "", common.ErrorAtPos(line, col, "Nil is not a valid value; use Option[Ref[T]] for nullable references")
	}
	if enumName, ok := g.variantByName[name]; ok {
		return g.translateEnumConstructor(enumName, name, nil, ctx, expected)
	}
	if typeclassHelper, typ, ok := g.translateTypeclassIdent(name, ctx, expected); ok {
		return typeclassHelper, typ, nil
	}
	return jen.Id(name), ctx.locals[name], nil
}

func (g *generator) translateTypeclassIdent(name string, ctx *exprCtx, expected string) (jen.Code, string, bool) {
	if ifaceName, ok := g.resolveTypeclassInterface(name, ctx); ok {
		if funcName, ok := ctx.constraintFuncs[name]; ok {
			return jen.Id(funcName), expected, true
		}
		if helper, ok := g.matchTypeclassHelper(ifaceName, name, nil, ctx); ok {
			return helper, expected, true
		}
	}
	return nil, "", false
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

func (g *generator) matchTypeclassHelper(ifaceName, method string, args []Expr, ctx *exprCtx) (jen.Code, bool) {
	var argCodes []jen.Code
	var argTypes []string
	for _, a := range args {
		code, typ, err := g.translateExpr(a, ctx, "")
		if err != nil {
			return nil, false
		}
		argCodes = append(argCodes, code)
		argTypes = append(argTypes, typ)
	}
	bestTypeKey := ""
	bestScore := matchScore{}
	found := false
	for _, impl := range g.pkg.Impls {
		if impl.InterfaceName != ifaceName && impl.Name != ifaceName {
			continue
		}
		implIfaceName := impl.InterfaceName
		if implIfaceName == "" {
			implIfaceName = impl.Name
		}
		if implIfaceName != ifaceName {
			continue
		}
		iface := g.pkg.Interfaces[ifaceName]
		if iface == nil {
			continue
		}
		methodDecl := (*FuncDecl)(nil)
		for _, m := range iface.Methods {
			if m.Name == method {
				methodDecl = m
				break
			}
		}
		if methodDecl == nil {
			continue
		}
		typeArgs := impl.InterfaceArgs
		if len(typeArgs) == 0 {
			typeArgs = impl.TypeArgs
		}
		if len(iface.TypeParams) != len(typeArgs) {
			continue
		}
		subst := map[string]string{}
		for i, tp := range iface.TypeParams {
			subst[tp] = g.goType(typeArgs[i], ctx.typeParams)
		}
		if len(methodDecl.Params) != len(argTypes) {
			continue
		}
		ok := true
		for i, p := range methodDecl.Params {
			want := typeString(p.Type, subst)
			if argTypes[i] != "" && want != argTypes[i] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		score := typeclassMatchScore(typeArgs, ctx.typeParams)
		if !found || betterMatch(score, bestScore) {
			bestScore = score
			bestTypeKey = g.implTypeKey(typeArgs)
			found = true
		} else if sameMatch(score, bestScore) {
			return nil, false
		}
	}
	if !found {
		return nil, false
	}
	_ = argCodes
	return jen.Id(helperFuncName(method, bestTypeKey)).Call(argCodes...), true
}

func (g *generator) hasHelper(method, typ string) bool {
	_ = method
	_ = typ
	return true
}
