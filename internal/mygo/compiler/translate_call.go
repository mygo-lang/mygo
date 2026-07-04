package compiler

import (
	"fmt"
	"strings"

	jen "github.com/dave/jennifer/jen"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func (g *generator) translateCall(n *CallExpr, ctx *exprCtx, expected string) (jen.Code, string, error) {
	if field, ok := n.Callee.(*FieldExpr); ok {
		if baseIdent, ok := field.Expr.(*IdentExpr); ok {
			if baseIdent.Name == "Ref" && field.Field == "new" {
				return g.translateRefNewCall(n.Args, ctx, expected)
			}
			if enumDecl := g.pkg.Enums[baseIdent.Name]; enumDecl != nil {
				if variant := g.findVariant(enumDecl, field.Field); variant != nil {
					return g.translateEnumConstructor(baseIdent.Name, field.Field, n.Args, ctx, expected)
				}
			}
			if code, typ, ok, err := g.translateGoSelectorCall(baseIdent.Name, field.Field, n.Args, ctx, expected); err != nil {
				return nil, "", err
			} else if ok {
				return code, typ, nil
			}
		}
		if code, typ, ok, err := g.translateGoMethodCall(field.Expr, field.Field, n.Args, ctx, expected); err != nil {
			return nil, "", err
		} else if ok {
			return code, typ, nil
		}
	}
	if id, ok := n.Callee.(*IdentExpr); ok {
		if st := g.pkg.Structs[id.Name]; st != nil && len(n.Args) == len(st.Fields) && len(st.Fields) > 0 && strings.HasPrefix(st.Fields[0].Name, "F") {
			typeName := sanitizeIdent(id.Name)
			var args []jen.Code
			for i, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, g.goType(st.Fields[i].Type, nil))
				if err != nil {
					return nil, "", err
				}
				args = append(args, code)
			}
			dict := jen.Dict{}
			for i, arg := range args {
				dict[jen.Id(fmt.Sprintf("F%d", i))] = arg
			}
			return jen.Id(typeName).Lit(dict), typeName, nil
		}
		if typ, ok := ctx.locals[id.Name]; ok && typ == "any" {
			if code, ret, ok, err := g.translateAnyFuncCall(id.Name, n.Args, ctx); err != nil {
				return nil, "", err
			} else if ok {
				return code, ret, nil
			}
			g.needsCallAny = true
			var args []jen.Code
			for _, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return nil, "", err
				}
				args = append(args, code)
			}
			actualName := id.Name
			if bound, ok := ctx.bindings[id.Name]; ok {
				actualName = bound
			}
			callArgs := append([]jen.Code{jen.Id(actualName)}, args...)
			return jen.Id("callAny").Call(callArgs...), "any", nil
		}
		if typ, ok := ctx.locals[id.Name]; ok && strings.HasPrefix(strings.TrimSpace(typ), "func(") {
			var args []jen.Code
			for _, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return nil, "", err
				}
				args = append(args, code)
			}
			return jen.Id(sanitizeIdent(id.Name)).Call(args...), funcReturnType(typ), nil
		}
		if g.pkg.Funcs[id.Name] != nil {
			fn := g.pkg.Funcs[id.Name]
			var args []jen.Code
			var argTypes []string
			for _, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return nil, "", err
				}
				args = append(args, code)
				_, typ, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return nil, "", err
				}
				argTypes = append(argTypes, typ)
			}
			subst := inferFuncTypeArgs(fn, argTypes, expected, ctx.typeParams)
			callee := jen.Id(sanitizeIdent(id.Name))
			if len(fn.TypeParams) > 0 && len(subst) == len(fn.TypeParams) {
				typeArgCodes := make([]jen.Code, 0, len(fn.TypeParams))
				for _, tp := range fn.TypeParams {
					typeArgCodes = append(typeArgCodes, jen.Id(subst[tp]))
				}
				callee = bracketArgs(callee, typeArgCodes)
			}
			// Auto-resolve using constraints at call site.
			// Resolution order: lexical scope → package-level impls.
			for _, c := range fn.Using {
				iface := g.pkg.Interfaces[c.Name]
				if iface == nil {
					return nil, "", common.ErrorAtPos(c.Line, c.Column, "call %s: missing interface %s", fn.Name, c.Name)
				}
				if len(iface.TypeParams) != len(c.Args) {
					return nil, "", common.ErrorAtPos(c.Line, c.Column, "call %s: type arity mismatch for %s", fn.Name, c.Name)
				}
				cTypeArgs := make([]string, 0, len(c.Args))
				for _, arg := range c.Args {
					cTypeArgs = append(cTypeArgs, typeString(arg, subst))
				}
				for _, m := range iface.Methods {
					resolvedType := ""
					if len(cTypeArgs) > 0 {
						resolvedType = cTypeArgs[0]
					}
					// 1. Lexical scope: check if caller has a matching constraint function.
					if helper, ok := ctx.constraintFuncs[m.Name]; ok {
						args = append(args, jen.Id(helper))
						continue
					}
					// 2. Lexical scope: check if caller's typeclassMethods provide a binding.
					if bindings, ok := ctx.typeclassMethods[m.Name]; ok && len(bindings) > 0 {
						best := typeclassBindingBest(bindings)
						if best.DictExpr != "" {
							args = append(args, jen.Id(best.DictExpr))
						} else {
							args = append(args, jen.Id(helperFuncName(m.Name, typeKeyFromType(resolvedType))))
						}
						continue
					}
					// 3. Package-level: use the helper function for this type.
					args = append(args, jen.Id(helperFuncName(m.Name, typeKeyFromType(resolvedType))))
				}
			}
			retType := g.goReturnType(fn.Ret, ctx.typeParams)
			if len(subst) > 0 {
				retType = typeStringReturn(fn.Ret, subst)
			}
			return callee.Call(args...), retType, nil
		}
		if enumName, ok := g.variantByName[id.Name]; ok {
			return g.translateEnumConstructor(enumName, id.Name, n.Args, ctx, expected)
		}
		if helper, typ, ok := g.translateTypeclassCall(id.Name, n.Args, ctx, expected); ok {
			return helper, typ, nil
		}
		if typ, ok := ctx.locals[id.Name]; ok && typ == "any" {
			if code, ret, ok, err := g.translateAnyFuncCall(id.Name, n.Args, ctx); err != nil {
				return nil, "", err
			} else if ok {
				return code, ret, nil
			}
			g.needsCallAny = true
			var args []jen.Code
			for _, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return nil, "", err
				}
				args = append(args, code)
			}
			actualName := id.Name
			if bound, ok := ctx.bindings[id.Name]; ok {
				actualName = bound
			}
			callArgs := append([]jen.Code{jen.Id(actualName)}, args...)
			return jen.Id("callAny").Call(callArgs...), "any", nil
		}
	}
	// Fallback: treat callee as an identifier
	return jen.Id("unknown").Call(), "", nil
}

func (g *generator) translateRefNewCall(args []Expr, ctx *exprCtx, expected string) (jen.Code, string, error) {
	if len(args) != 1 {
		return nil, "", common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "Ref.new expects exactly 1 arg, got %d", len(args))
	}

	innerExpected := ""
	if strings.HasPrefix(strings.TrimSpace(expected), "Ref[") && strings.HasSuffix(strings.TrimSpace(expected), "]") {
		innerExpected = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(expected), "Ref["), "]")
	} else if strings.HasPrefix(strings.TrimSpace(expected), "*") {
		innerExpected = strings.TrimPrefix(strings.TrimSpace(expected), "*")
	}

	code, typ, err := g.translateExpr(args[0], ctx, innerExpected)
	if err != nil {
		return nil, "", err
	}

	if typ != "" {
		if strings.HasPrefix(strings.TrimSpace(typ), "*") {
			return code, typ, nil
		}
		if strings.HasPrefix(strings.TrimSpace(typ), "Ref[") && strings.HasSuffix(strings.TrimSpace(typ), "]") {
			return code, typ, nil
		}
		return jen.Op("&").Add(code), "*" + typ, nil
	}

	if innerExpected != "" {
		return jen.Op("&").Add(code), "*" + innerExpected, nil
	}
	return jen.Op("&").Add(code), "", nil
}

func (g *generator) translateGoSelectorCall(alias, name string, args []Expr, ctx *exprCtx, expected string) (jen.Code, string, bool, error) {
	path, ok := g.pkg.ImportAliases[alias]
	if !ok {
		return nil, "", false, nil
	}
	sigs, err := g.goPackageSigsFor(importPathForGo(path))
	if err != nil {
		return nil, "", false, err
	}
	sig, ok := sigs.funcs[name]
	if !ok {
		return nil, "", false, nil
	}
	argCodes := make([]jen.Code, 0, len(args))
	argTypes := make([]string, 0, len(args))
	for _, a := range args {
		code, typ, err := g.translateExpr(a, ctx, "")
		if err != nil {
			return nil, "", false, err
		}
		argCodes = append(argCodes, code)
		argTypes = append(argTypes, typ)
	}
	variadic := len(sig.params) > 0 && strings.HasPrefix(sig.params[len(sig.params)-1], "...")
	fixed := len(sig.params)
	if variadic {
		fixed--
	}
	if (!variadic && len(sig.params) != len(argTypes)) || (variadic && len(argTypes) < fixed) {
		want := len(sig.params)
		if variadic {
			want = fixed
		}
		return nil, "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "call %s.%s: expected %d args, got %d", alias, name, want, len(argTypes))
	}
	for i := 0; i < fixed; i++ {
		if !g.goTypeCompatible(sig.params[i], argTypes[i]) {
			return nil, "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "call %s.%s: arg %d has type %s, want %s", alias, name, i+1, argTypes[i], sig.params[i])
		}
	}
	if variadic {
		want := strings.TrimPrefix(sig.params[len(sig.params)-1], "...")
		for i := fixed; i < len(argTypes); i++ {
			if !g.goTypeCompatible(want, argTypes[i]) {
				return nil, "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "call %s.%s: arg %d has type %s, want %s", alias, name, i+1, argTypes[i], want)
			}
		}
	}
	call := jen.Id(alias).Dot(name).Call(argCodes...)
	if len(sig.ret) == 2 && isGoErrorType(sig.ret[1]) {
		base, args := splitTypeArgs(expected)
		if base != "Result" || len(args) != 2 {
			return call, "", true, nil
		}
		valueType := args[0]
		okType := args[1]
		retType := fmt.Sprintf("Result[%s, %s]", valueType, okType)
		return call, retType, true, nil
	}
	return call, "", true, nil
}

func (g *generator) translateGoMethodCall(base Expr, name string, args []Expr, ctx *exprCtx, expected string) (jen.Code, string, bool, error) {
	baseCode, baseType, err := g.translateExpr(base, ctx, "")
	if err != nil {
		return nil, "", false, err
	}
	methodSig, ok := g.findGoMethodSig(baseType, name)
	if !ok {
		return nil, "", false, nil
	}
	argCodes := make([]jen.Code, 0, len(args))
	argTypes := make([]string, 0, len(args))
	for _, a := range args {
		code, typ, err := g.translateExpr(a, ctx, "")
		if err != nil {
			return nil, "", false, err
		}
		argCodes = append(argCodes, code)
		argTypes = append(argTypes, typ)
	}
	variadic := len(methodSig.params) > 0 && strings.HasPrefix(methodSig.params[len(methodSig.params)-1], "...")
	fixed := len(methodSig.params)
	if variadic {
		fixed--
	}
	if (!variadic && len(methodSig.params) != len(argTypes)) || (variadic && len(argTypes) < fixed) {
		want := len(methodSig.params)
		if variadic {
			want = fixed
		}
		return nil, "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "call %s.%s: expected %d args, got %d", baseType, name, want, len(argTypes))
	}
	for i := 0; i < fixed; i++ {
		if !g.goTypeCompatible(methodSig.params[i], argTypes[i]) {
			return nil, "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "call %s.%s: arg %d has type %s, want %s", baseType, name, i+1, argTypes[i], methodSig.params[i])
		}
	}
	if variadic {
		want := strings.TrimPrefix(methodSig.params[len(methodSig.params)-1], "...")
		for i := fixed; i < len(argTypes); i++ {
			if !g.goTypeCompatible(want, argTypes[i]) {
				return nil, "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "call %s.%s: arg %d has type %s, want %s", baseType, name, i+1, argTypes[i], want)
			}
		}
	}
	call := baseCode.(*jen.Statement).Dot(name).Call(argCodes...)
	if len(methodSig.ret) == 2 && isGoErrorType(methodSig.ret[1]) {
		base, args := splitTypeArgs(expected)
		if base != "Result" || len(args) != 2 {
			return call, "", true, nil
		}
		valueType := args[0]
		okType := args[1]
		retType := fmt.Sprintf("Result[%s, %s]", valueType, okType)
		return call, retType, true, nil
	}
	return call, goMethodReturnType(methodSig.ret), true, nil
}
