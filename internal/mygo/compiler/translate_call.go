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
			if path, ok := g.pkg.ImportAliases[baseIdent.Name]; ok && !strings.HasPrefix(path, "go:") {
				if code, typ, ok, err := g.translateMyGoSelectorCall(baseIdent.Name, field.Field, n.Args, ctx, expected); err != nil {
					return nil, "", err
				} else if ok {
					return code, typ, nil
				}
			}
			if code, typ, ok, err := g.translateGoSelectorCall(baseIdent.Name, field.Field, n.Args, ctx, expected); err != nil {
				return nil, "", err
			} else if ok {
				return code, typ, nil
			}
			if code, typ, ok, err := g.translateMyGoSelectorCall(baseIdent.Name, field.Field, n.Args, ctx, expected); err != nil {
				return nil, "", err
			} else if ok {
				return code, typ, nil
			}
		}
		if field.Field == "value" && len(n.Args) == 0 {
			baseCode, baseType, err := g.translateExpr(field.Expr, ctx, "")
			if err != nil {
				return nil, "", err
			}
			trimmed := strings.TrimSpace(baseType)
			if strings.HasPrefix(trimmed, "Ref[") && strings.HasSuffix(trimmed, "]") {
				return jen.Op("*").Add(baseCode), strings.TrimSuffix(strings.TrimPrefix(trimmed, "Ref["), "]"), nil
			}
			if strings.HasPrefix(trimmed, "*") {
				return jen.Op("*").Add(baseCode), strings.TrimPrefix(trimmed, "*"), nil
			}
		}
		if baseIdent, ok := field.Expr.(*IdentExpr); ok {
			if code, typ, ok, err := g.translateInherentTypeCall(baseIdent.Name, field.Field, n.Args, ctx, expected); err != nil {
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
		if code, typ, ok, err := g.translateInherentMethodCall(n, field, ctx); err != nil {
			return nil, "", err
		} else if ok {
			return code, typ, nil
		}
		if helper, typ, ok := g.translateTypeclassCall(field.Field, append([]Expr{field.Expr}, n.Args...), ctx, expected); ok {
			return helper, typ, nil
		}
		// Handle method calls on impl types (e.g., self.isSome() inside an impl block).
		if _, hasConstraintHelper := ctx.constraintFuncForMethod(field.Field); ctx.currentImpl != "" && ctx.implTypeKey != "" && !hasConstraintHelper {
			if iface := g.pkg.Interfaces[ctx.currentImpl]; iface != nil {
				for _, m := range iface.Methods {
					if m.Name == field.Field {
						baseCode, _, err := g.translateExpr(field.Expr, ctx, "")
						if err != nil {
							return nil, "", err
						}
						argCodes := []jen.Code{baseCode}
						for _, a := range n.Args {
							code, _, err := g.translateExpr(a, ctx, "")
							if err != nil {
								return nil, "", err
							}
							argCodes = append(argCodes, code)
						}
						retType := g.goReturnType(m.Ret, ctx.typeParams)
						fnName := helperFuncName(m.Name, ctx.implTypeKey)
						callee := jen.Id(fnName)
						if len(ctx.implTypeParams) > 0 {
							typeArgCodes := make([]jen.Code, 0, len(ctx.implTypeParams))
							for _, tp := range ctx.implTypeParams {
								typeArgCodes = append(typeArgCodes, jen.Id(tp))
							}
							callee = bracketArgs(callee, typeArgCodes)
						}
						return callee.Call(argCodes...), retType, nil
					}
				}
			}
		}
		// Handle method calls through typeclass constraints (e.g., value.show()
		// inside a function with using FancyShow[Int64]).
		if bindings, ok := ctx.typeclassMethods[field.Field]; ok && len(bindings) > 0 {
			receiverCode, _, err := g.translateExpr(field.Expr, ctx, "")
			if err != nil {
				return nil, "", err
			}
			argCodes := []jen.Code{receiverCode}
			for _, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return nil, "", err
				}
				argCodes = append(argCodes, code)
			}
			receiverType := g.hmExprType(field.Expr)
			best, ok := typeclassBindingForReceiver(bindings, receiverType)
			if !ok {
				best = typeclassBindingBest(bindings)
			}
			funcName, ok := ctx.constraintFuncForMethod(field.Field)
			if !ok {
				_, ok = g.interfaceByMethod[field.Field]
				if ok {
					funcName = helperFuncName(field.Field, typeKeyFromType(""))
				} else {
					funcName = helperFuncName(field.Field, typeKeyFromType(best.RetType))
				}
			}
			retType := best.RetType
			if retType == "" {
				retType = ctx.retType
			}
			return jen.Id(funcName).Call(argCodes...), retType, nil
		}
		if ifaceName, ok := g.interfaceByMethod[field.Field]; ok {
			helperArgs := append([]Expr{field.Expr}, n.Args...)
			if helper, ok := g.matchTypeclassHelper(ifaceName, field.Field, helperArgs, ctx); ok {
				iface := g.pkg.Interfaces[ifaceName]
				return helper, methodReturnType(iface, field.Field), nil
			}
		}
	}
	if id, ok := n.Callee.(*IdentExpr); ok {
		if code, typ, ok := g.translatePreludeCall(id.Name, n.Args, ctx, expected); ok {
			return code, typ, nil
		}
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
		if method, ok := g.inherentMethodByMangledName(id.Name); ok {
			var args []jen.Code
			for _, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return nil, "", err
				}
				args = append(args, code)
			}
			retType := g.hmExprType(n)
			if retType == "" {
				retType = g.goReturnType(method.Func.Ret, ctx.typeParams)
			}
			return jen.Id(sanitizeIdent(id.Name)).Call(args...), retType, nil
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
			for _, c := range fn.Using {
				namedImpl, iface, ok := g.resolveUsingConstraint(c)
				if !ok {
					return nil, "", common.ErrorAtPos(c.Line, c.Column, "call %s: missing implementation or interface %s", fn.Name, c.Name)
				}
				if len(iface.TypeParams) != len(c.Args) {
					return nil, "", common.ErrorAtPos(c.Line, c.Column, "call %s: type arity mismatch for %s", fn.Name, c.Name)
				}
				cTypeArgs := make([]string, 0, len(c.Args))
				for _, arg := range c.Args {
					cTypeArgs = append(cTypeArgs, typeString(arg, subst))
				}
				namedImplTypeKey := ""
				if namedImpl != nil && c.BindName != "" {
					implTypeArgs := append([]TypeExpr(nil), namedImpl.InterfaceArgs...)
					if len(implTypeArgs) == 0 {
						implTypeArgs = append([]TypeExpr(nil), namedImpl.TypeArgs...)
					}
					implSubst := map[string]string{}
					if len(namedImpl.TypeParams) > 0 {
						if len(namedImpl.TypeParams) != len(c.Args) {
							return nil, "", common.ErrorAtPos(c.Line, c.Column, "call %s: type arity mismatch for %s", fn.Name, c.BindName)
						}
						for i, tp := range namedImpl.TypeParams {
							implSubst[tp] = typeString(c.Args[i], subst)
						}
					}
					for i, arg := range implTypeArgs {
						implTypeArgs[i] = substituteTypeExpr(arg, implSubst)
					}
					namedImplTypeKey = g.implHelperKey(namedImpl, implTypeArgs)
				}
				for _, m := range iface.Methods {
					resolvedType := ""
					if len(cTypeArgs) > 0 {
						resolvedType = cTypeArgs[0]
					}
					if namedImplTypeKey != "" {
						args = append(args, jen.Id(helperFuncName(m.Name, namedImplTypeKey)))
						continue
					}
					if bindings, ok := ctx.typeclassMethods[m.Name]; ok && len(bindings) > 0 {
						best := typeclassBindingBest(bindings)
						if receiverType := g.hmExprType(n.Callee); receiverType != "" {
							if chosen, ok := typeclassBindingForReceiver(bindings, receiverType); ok {
								best = chosen
							}
						}
						if best.DictExpr != "" {
							args = append(args, jen.Id(best.DictExpr))
						} else {
							args = append(args, jen.Id(helperFuncName(m.Name, typeKeyFromType(resolvedType))))
						}
						continue
					}
					if helper, ok := ctx.constraintFuncForMethod(m.Name); ok {
						args = append(args, jen.Id(helper))
						continue
					}
					args = append(args, jen.Id(helperFuncName(m.Name, typeKeyFromType(resolvedType))))
				}
			}
			retType := myGoTypeString(fn.Ret, nil)
			if len(subst) > 0 {
				retType = typeStringReturn(fn.Ret, subst)
			}
			return callee.Call(args...), retType, nil
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
		if enumName, ok := g.variantByName[id.Name]; ok {
			return g.translateEnumConstructor(enumName, id.Name, n.Args, ctx, expected)
		}
		if helper, typ, ok := g.translateTypeclassCall(id.Name, n.Args, ctx, expected); ok {
			return helper, typ, nil
		}
		if id.Name != "" {
			var args []jen.Code
			for _, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return nil, "", err
				}
				args = append(args, code)
			}
			return jen.Id(sanitizeIdent(id.Name)).Call(args...), expected, nil
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
	// Fallback: do not generate a bogus call. Surface a hard error instead so
	// we can fix the missing translation path at the source.
	line, col := common.NodePos(n)
	return nil, "", common.ErrorAtPos(line, col, "unsupported call expression %#v", n.Callee)
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

	// If the expression is a non-addressable function call, use a temp variable
	if _, ok := args[0].(*CallExpr); ok {
		tmp := g.bindLocal(ctx, "__ref_tmp", typ, false)
		return jen.Func().Params().Id("*"+typ).Block(
			jen.Id(tmp).Op(":=").Add(code),
			jen.Return(jen.Op("&").Id(tmp)),
		).Call(), "*" + typ, nil
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
	call := jen.Qual(importPathForGo(path), name).Call(argCodes...)
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
	if len(sig.ret) == 1 {
		return call, sig.ret[0], true, nil
	}
	if len(sig.ret) > 1 {
		return call, "(" + strings.Join(sig.ret, ", ") + ")", true, nil
	}
	return call, "", true, nil
}

func (g *generator) translateGoMethodCall(base Expr, name string, args []Expr, ctx *exprCtx, expected string) (jen.Code, string, bool, error) {
	baseCode, baseType, err := g.translateExpr(base, ctx, "")
	if err != nil {
		return nil, "", false, err
	}
	if (name == "len" || name == "Len") && len(args) == 0 {
		trimmed := strings.TrimSpace(baseType)
		if strings.HasPrefix(trimmed, "Slice[") || strings.HasPrefix(trimmed, "Map[") || strings.HasPrefix(trimmed, "Set[") || strings.HasPrefix(trimmed, "[]") || trimmed == "String" || trimmed == "string" {
			return jen.Len(baseCode), "Int", true, nil
		}
	}

	// Built-in container type methods (Map, Slice)
	if code, typ, ok, err := g.translateContainerMethod(baseCode, baseType, name, args, ctx, expected); err != nil {
		return nil, "", false, err
	} else if ok {
		return code, typ, true, nil
	}

	methodSig, ok := g.findGoMethodSig(baseType, name)
	if !ok {
		baseTypeName := strings.TrimSpace(baseType)
		if idx := strings.Index(baseTypeName, "["); idx >= 0 {
			baseTypeName = baseTypeName[:idx]
		}
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

// translateContainerMethod handles built-in method calls on container types
// (Map, Slice) that are lowered to Go native types. These are not real Go methods
// but are syntactic sugar in MyGO.
func (g *generator) translateContainerMethod(baseCode jen.Code, baseType, name string, args []Expr, ctx *exprCtx, expected string) (jen.Code, string, bool, error) {
	trimmed := strings.TrimSpace(baseType)

	// --- Map[K, V] or map[K]V ---
	var mapValType string
	isMap := false
	if strings.HasPrefix(trimmed, "Map[") && strings.HasSuffix(trimmed, "]") {
		inner := strings.TrimSuffix(strings.TrimPrefix(trimmed, "Map["), "]")
		parts := splitTopLevelArgs(inner)
		if len(parts) == 2 {
			mapValType = parts[1]
			isMap = true
		}
	} else if strings.HasPrefix(trimmed, "map[") {
		endBracket := strings.Index(trimmed, "]")
		if endBracket > 4 {
			mapValType = strings.TrimSpace(trimmed[endBracket+1:])
			isMap = true
		}
	}
	if isMap {
		switch name {
		case "Set":
			if len(args) != 2 {
				return nil, "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "Map.Set expects 2 args, got %d", len(args))
			}
			keyCode, _, err := g.translateExpr(args[0], ctx, "")
			if err != nil {
				return nil, "", false, err
			}
			valCode, _, err := g.translateExpr(args[1], ctx, "")
			if err != nil {
				return nil, "", false, err
			}
			return baseCode.(*jen.Statement).Index(keyCode).Op("=").Add(valCode), "", true, nil

		case "Get":
			if len(args) != 1 {
				return nil, "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "Map.Get expects 1 arg, got %d", len(args))
			}
			keyCode, _, err := g.translateExpr(args[0], ctx, "")
			if err != nil {
				return nil, "", false, err
			}
			valGoType := lowerMyGoTypeString(mapValType)
			// IIFE: func() Option[V] { if v, ok := m[k]; ok { return Some(v) }; return None[V]() }()
			fn := jen.Func().Params().Add(jen.Id("Option").Types(jen.Id(valGoType)))
			fn = fn.Block(
				jen.If(
					jen.List(jen.Id("v"), jen.Id("ok")).Op(":=").Add(baseCode.(*jen.Statement).Index(keyCode)),
					jen.Id("ok"),
				).Block(
					jen.Return(jen.Id("Some").Types(jen.Id(valGoType)).Call(jen.Id("v"))),
				),
				jen.Return(jen.Id("None").Types(jen.Id(valGoType)).Call()),
			)
			return fn.Call(), "Option[" + mapValType + "]", true, nil

		case "Delete":
			if len(args) != 1 {
				return nil, "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "Map.Delete expects 1 arg, got %d", len(args))
			}
			keyCode, _, err := g.translateExpr(args[0], ctx, "")
			if err != nil {
				return nil, "", false, err
			}
			return jen.Id("delete").Call(baseCode, keyCode), "", true, nil
		}
	}

	// --- Slice[A] or []A ---
	var sliceElemType string
	isSlice := false
	if strings.HasPrefix(trimmed, "Slice[") && strings.HasSuffix(trimmed, "]") {
		sliceElemType = strings.TrimSuffix(strings.TrimPrefix(trimmed, "Slice["), "]")
		isSlice = true
	} else if strings.HasPrefix(trimmed, "[]") {
		sliceElemType = strings.TrimSpace(trimmed[2:])
		isSlice = true
	}
	if isSlice {
		switch name {
		case "Len":
			return jen.Len(baseCode), "Int", true, nil
		case "Get":
			if len(args) != 1 {
				return nil, "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "Slice.Get expects 1 arg, got %d", len(args))
			}
			idxCode, _, err := g.translateExpr(args[0], ctx, "")
			if err != nil {
				return nil, "", false, err
			}
			return baseCode.(*jen.Statement).Index(idxCode), sliceElemType, true, nil
		case "Set":
			if len(args) != 2 {
				return nil, "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "Slice.Set expects 2 args, got %d", len(args))
			}
			idxCode, _, err := g.translateExpr(args[0], ctx, "")
			if err != nil {
				return nil, "", false, err
			}
			valCode, _, err := g.translateExpr(args[1], ctx, "")
			if err != nil {
				return nil, "", false, err
			}
			return baseCode.(*jen.Statement).Index(idxCode).Op("=").Add(valCode), "", true, nil
		}
	}

	return nil, "", false, nil
}

func (g *generator) translateInherentMethodCall(n *CallExpr, field *FieldExpr, ctx *exprCtx) (jen.Code, string, bool, error) {
	receiverType := g.hmExprType(field.Expr)
	if receiverType == "" {
		_, typ, err := g.translateExpr(field.Expr, ctx, "")
		if err != nil {
			return nil, "", false, err
		}
		receiverType = typ
	}
	receiverName := baseNamedType(receiverType)
	if receiverName == "" {
		return nil, "", false, nil
	}
	methods := g.inherentMethods[receiverName]
	if len(methods) == 0 {
		return nil, "", false, nil
	}
	method := methods[field.Field]
	if method == nil || !method.HasReceiver {
		return nil, "", false, nil
	}
	receiverCode, _, err := g.translateExpr(field.Expr, ctx, "")
	if err != nil {
		return nil, "", false, err
	}
	argCodes := []jen.Code{receiverCode}
	for _, a := range n.Args {
		code, _, err := g.translateExpr(a, ctx, "")
		if err != nil {
			return nil, "", false, err
		}
		argCodes = append(argCodes, code)
	}
	retType := g.hmExprType(n)
	if retType == "" {
		retType = g.goReturnType(method.Func.Ret, ctx.typeParams)
	}
	return jen.Id(inherentMethodName(receiverName, method.Func.Name)).Call(argCodes...), retType, true, nil
}

func (g *generator) translateInherentTypeCall(typeName, methodName string, args []Expr, ctx *exprCtx, expected string) (jen.Code, string, bool, error) {
	if typeName == "" {
		return nil, "", false, nil
	}
	methods := g.inherentMethods[typeName]
	if len(methods) == 0 {
		return nil, "", false, nil
	}
	method := methods[methodName]
	if method == nil || method.HasReceiver {
		return nil, "", false, nil
	}
	var argCodes []jen.Code
	for _, a := range args {
		code, _, err := g.translateExpr(a, ctx, "")
		if err != nil {
			return nil, "", false, err
		}
		argCodes = append(argCodes, code)
	}
	retType := g.goReturnType(method.Func.Ret, ctx.typeParams)
	if retType == "" {
		retType = expected
	}
	return jen.Id(inherentMethodName(typeName, method.Func.Name)).Call(argCodes...), retType, true, nil
}

func (g *generator) inherentMethodByMangledName(name string) (*inherentMethod, bool) {
	for receiverName, methods := range g.inherentMethods {
		for methodName, method := range methods {
			if inherentMethodName(receiverName, methodName) == name {
				return method, true
			}
		}
	}
	return nil, false
}

func baseNamedType(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if strings.HasPrefix(typeName, "*") {
		typeName = strings.TrimSpace(strings.TrimPrefix(typeName, "*"))
	}
	if strings.HasPrefix(typeName, "Ref[") && strings.HasSuffix(typeName, "]") {
		typeName = strings.TrimSuffix(strings.TrimPrefix(typeName, "Ref["), "]")
	}
	if idx := strings.Index(typeName, "["); idx >= 0 {
		typeName = typeName[:idx]
	}
	if strings.Contains(typeName, "{") {
		return ""
	}
	return strings.TrimSpace(typeName)
}

func (g *generator) translatePreludeCall(name string, args []Expr, ctx *exprCtx, expected string) (jen.Code, string, bool) {
	if g == nil || g.pkg == nil {
		return nil, "", false
	}
	switch name {
	case "Some", "None", "Ok", "Err":
		var argCodes []jen.Code
		for _, a := range args {
			code, _, err := g.translateExpr(a, ctx, "")
			if err != nil {
				return nil, "", false
			}
			argCodes = append(argCodes, code)
		}
		candidate := strings.TrimSpace(expected)
		if candidate == "" {
			candidate = strings.TrimSpace(ctx.retType)
		}
		var target *jen.Statement
		switch name {
		case "Some", "None":
			if base, args := splitTypeArgs(candidate); base == "Option" && len(args) == 1 {
				target = jen.Id(name + "[" + lowerMyGoTypeString(args[0]) + "]")
			}
		case "Ok", "Err":
			if base, args := splitTypeArgs(candidate); base == "Result" && len(args) == 2 {
				target = jen.Id(name + "[" + lowerMyGoTypeString(args[0]) + ", " + lowerMyGoTypeString(args[1]) + "]")
			}
		}
		if target == nil {
			target = jen.Id(name)
		}
		if g.pkg.Name != "prelude" {
			switch name {
			case "Some", "None":
				if base, args := splitTypeArgs(candidate); base == "Option" && len(args) == 1 {
					target = jen.Id(name + "[" + lowerMyGoTypeString(args[0]) + "]")
				}
			case "Ok", "Err":
				if base, args := splitTypeArgs(candidate); base == "Result" && len(args) == 2 {
					target = jen.Id(name + "[" + lowerMyGoTypeString(args[0]) + ", " + lowerMyGoTypeString(args[1]) + "]")
				}
			}
		}
		return target.Call(argCodes...), expected, true
	}
	return nil, "", false
}
