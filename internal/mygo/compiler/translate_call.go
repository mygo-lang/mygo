package compiler

import (
	"fmt"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func (g *generator) translateCall(n *CallExpr, ctx *exprCtx, expected string) (string, string, error) {
	if field, ok := n.Callee.(*FieldExpr); ok {
		if baseIdent, ok := field.Expr.(*IdentExpr); ok {
			if enumDecl := g.pkg.Enums[baseIdent.Name]; enumDecl != nil {
				if variant := g.findVariant(enumDecl, field.Field); variant != nil {
					return g.translateEnumConstructor(baseIdent.Name, field.Field, n.Args, ctx, expected)
				}
			}
			if code, typ, ok, err := g.translateGoSelectorCall(baseIdent.Name, field.Field, n.Args, ctx, expected); err != nil {
				return "", "", err
			} else if ok {
				return code, typ, nil
			}
		}
		if code, typ, ok, err := g.translateGoMethodCall(field.Expr, field.Field, n.Args, ctx, expected); err != nil {
			return "", "", err
		} else if ok {
			return code, typ, nil
		}
	}
	if id, ok := n.Callee.(*IdentExpr); ok {
		if st := g.pkg.Structs[id.Name]; st != nil && len(n.Args) == len(st.Fields) && len(st.Fields) > 0 && strings.HasPrefix(st.Fields[0].Name, "F") {
			var args []string
			for i, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, g.goType(st.Fields[i].Type, nil))
				if err != nil {
					return "", "", err
				}
				args = append(args, code)
			}
			parts := make([]string, 0, len(args))
			for i, arg := range args {
				parts = append(parts, fmt.Sprintf("F%d: %s", i, arg))
			}
			return fmt.Sprintf("%s{%s}", id.Name, strings.Join(parts, ", ")), id.Name, nil
		}
		if typ, ok := ctx.locals[id.Name]; ok && typ == "any" {
			if code, ret, ok, err := g.translateAnyFuncCall(id.Name, n.Args, ctx); err != nil {
				return "", "", err
			} else if ok {
				return code, ret, nil
			}
			g.needsCallAny = true
			var args []string
			for _, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return "", "", err
				}
				args = append(args, code)
			}
			actualName := id.Name
			if bound, ok := ctx.bindings[id.Name]; ok {
				actualName = bound
			}
			return fmt.Sprintf("callAny(%s%s)", actualName, func() string {
				if len(args) == 0 {
					return ""
				}
				return ", " + strings.Join(args, ", ")
			}()), "any", nil
		}
		if typ, ok := ctx.locals[id.Name]; ok && strings.HasPrefix(strings.TrimSpace(typ), "func(") {
			var args []string
			for _, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return "", "", err
				}
				args = append(args, code)
			}
			return fmt.Sprintf("%s(%s)", id.Name, strings.Join(args, ", ")), funcReturnType(typ), nil
		}
		if g.pkg.Funcs[id.Name] != nil {
			fn := g.pkg.Funcs[id.Name]
			var args []string
			var argTypes []string
			for _, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return "", "", err
				}
				args = append(args, code)
				_, typ, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return "", "", err
				}
				argTypes = append(argTypes, typ)
			}
			subst := inferFuncTypeArgs(fn, argTypes, expected, ctx.typeParams)
			callee := id.Name
			if len(fn.TypeParams) > 0 && len(subst) == len(fn.TypeParams) {
				var typeArgs []string
				for _, tp := range fn.TypeParams {
					typeArgs = append(typeArgs, subst[tp])
				}
				callee += "[" + strings.Join(typeArgs, ", ") + "]"
			}
			for _, c := range fn.Where {
				iface := g.pkg.Interfaces[c.Name]
				if iface == nil {
					return "", "", common.ErrorAtPos(c.Line, c.Column, "call %s: missing interface %s", fn.Name, c.Name)
				}
				if len(iface.TypeParams) != len(c.Args) {
					return "", "", common.ErrorAtPos(c.Line, c.Column, "call %s: type arity mismatch for %s", fn.Name, c.Name)
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
					if _, ok := ctx.typeParams[resolvedType]; ok {
						if helper, ok := ctx.constraintFuncs[m.Name]; ok {
							args = append(args, helper)
							continue
						}
					}
					args = append(args, helperFuncName(m.Name, typeKeyFromType(resolvedType)))
				}
			}
			retType := g.goReturnType(fn.Ret, ctx.typeParams)
			if len(subst) > 0 {
				retType = typeStringReturn(fn.Ret, subst)
			}
			return fmt.Sprintf("%s(%s)", callee, strings.Join(args, ", ")), retType, nil
		}
		if enumName, ok := g.variantByName[id.Name]; ok {
			return g.translateEnumConstructor(enumName, id.Name, n.Args, ctx, expected)
		}
		if helper, typ, ok := g.translateTypeclassCall(id.Name, n.Args, ctx, expected); ok {
			return helper, typ, nil
		}
		if typ, ok := ctx.locals[id.Name]; ok && typ == "any" {
			if code, ret, ok, err := g.translateAnyFuncCall(id.Name, n.Args, ctx); err != nil {
				return "", "", err
			} else if ok {
				return code, ret, nil
			}
			g.needsCallAny = true
			var args []string
			for _, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return "", "", err
				}
				args = append(args, code)
			}
			actualName := id.Name
			if bound, ok := ctx.bindings[id.Name]; ok {
				actualName = bound
			}
			return fmt.Sprintf("callAny(%s%s)", actualName, func() string {
				if len(args) == 0 {
					return ""
				}
				return ", " + strings.Join(args, ", ")
			}()), "any", nil
		}
	}
	callee, calleeType, err := g.translateExpr(n.Callee, ctx, "")
	if err != nil {
		return "", "", err
	}
	var args []string
	for _, a := range n.Args {
		code, _, err := g.translateExpr(a, ctx, "")
		if err != nil {
			return "", "", err
		}
		args = append(args, code)
	}
	retType := expected
	if parsedRet := funcReturnType(calleeType); parsedRet != "" {
		retType = parsedRet
	}
	return fmt.Sprintf("%s(%s)", callee, strings.Join(args, ", ")), retType, nil
}

func (g *generator) translateGoSelectorCall(alias, name string, args []Expr, ctx *exprCtx, expected string) (string, string, bool, error) {
	path, ok := g.pkg.ImportAliases[alias]
	if !ok {
		return "", "", false, nil
	}
	sigs, err := g.goPackageSigsFor(importPathForGo(path))
	if err != nil {
		return "", "", false, err
	}
	sig, ok := sigs.funcs[name]
	if !ok {
		return "", "", false, nil
	}
	argCodes := make([]string, 0, len(args))
	argTypes := make([]string, 0, len(args))
	for _, a := range args {
		code, typ, err := g.translateExpr(a, ctx, "")
		if err != nil {
			return "", "", false, err
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
		return "", "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "call %s.%s: expected %d args, got %d", alias, name, want, len(argTypes))
	}
	for i := 0; i < fixed; i++ {
		if !g.goTypeCompatible(sig.params[i], argTypes[i]) {
			return "", "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "call %s.%s: arg %d has type %s, want %s", alias, name, i+1, argTypes[i], sig.params[i])
		}
	}
	if variadic {
		want := strings.TrimPrefix(sig.params[len(sig.params)-1], "...")
		for i := fixed; i < len(argTypes); i++ {
			if !g.goTypeCompatible(want, argTypes[i]) {
				return "", "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "call %s.%s: arg %d has type %s, want %s", alias, name, i+1, argTypes[i], want)
			}
		}
	}
	call := fmt.Sprintf("%s.%s(%s)", alias, name, strings.Join(argCodes, ", "))
	if len(sig.ret) == 2 && isGoErrorType(sig.ret[1]) {
		base, args := splitTypeArgs(expected)
		if base != "Result" || len(args) != 2 {
			return call, "", true, nil
		}
		valueType := args[0]
		okType := args[1]
		retType := fmt.Sprintf("Result[%s, %s]", valueType, okType)
		var b strings.Builder
		b.WriteString("func() ")
		b.WriteString(retType)
		b.WriteString(" {\n")
		b.WriteString("\tvalue, err := ")
		b.WriteString(call)
		b.WriteString("\n")
		b.WriteString("\tif err != nil {\n")
		b.WriteString("\t\treturn Err[")
		b.WriteString(valueType)
		b.WriteString(", ")
		b.WriteString(okType)
		b.WriteString("](err.Error())\n")
		b.WriteString("\t}\n")
		b.WriteString("\treturn Ok[")
		b.WriteString(valueType)
		b.WriteString(", ")
		b.WriteString(okType)
		b.WriteString("](value)\n")
		b.WriteString("}()")
		return b.String(), retType, true, nil
	}
	return call, "", true, nil
}

func (g *generator) translateGoMethodCall(base Expr, name string, args []Expr, ctx *exprCtx, expected string) (string, string, bool, error) {
	baseCode, baseType, err := g.translateExpr(base, ctx, "")
	if err != nil {
		return "", "", false, err
	}
	methodSig, ok := g.findGoMethodSig(baseType, name)
	if !ok {
		return "", "", false, nil
	}
	argCodes := make([]string, 0, len(args))
	argTypes := make([]string, 0, len(args))
	for _, a := range args {
		code, typ, err := g.translateExpr(a, ctx, "")
		if err != nil {
			return "", "", false, err
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
		return "", "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "call %s.%s: expected %d args, got %d", baseType, name, want, len(argTypes))
	}
	for i := 0; i < fixed; i++ {
		if !g.goTypeCompatible(methodSig.params[i], argTypes[i]) {
			return "", "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "call %s.%s: arg %d has type %s, want %s", baseType, name, i+1, argTypes[i], methodSig.params[i])
		}
	}
	if variadic {
		want := strings.TrimPrefix(methodSig.params[len(methodSig.params)-1], "...")
		for i := fixed; i < len(argTypes); i++ {
			if !g.goTypeCompatible(want, argTypes[i]) {
				return "", "", false, common.ErrorAtPos(nodeLineFromExprSlice(args), nodeColFromExprSlice(args), "call %s.%s: arg %d has type %s, want %s", baseType, name, i+1, argTypes[i], want)
			}
		}
	}
	call := fmt.Sprintf("%s.%s(%s)", baseCode, name, strings.Join(argCodes, ", "))
	if len(methodSig.ret) == 2 && isGoErrorType(methodSig.ret[1]) {
		base, args := splitTypeArgs(expected)
		if base != "Result" || len(args) != 2 {
			return call, "", true, nil
		}
		valueType := args[0]
		okType := args[1]
		retType := fmt.Sprintf("Result[%s, %s]", valueType, okType)
		var b strings.Builder
		b.WriteString("func() ")
		b.WriteString(retType)
		b.WriteString(" {\n")
		b.WriteString("\tvalue, err := ")
		b.WriteString(call)
		b.WriteString("\n")
		b.WriteString("\tif err != nil {\n")
		b.WriteString("\t\treturn Err[")
		b.WriteString(valueType)
		b.WriteString(", ")
		b.WriteString(okType)
		b.WriteString("](err.Error())\n")
		b.WriteString("\t}\n")
		b.WriteString("\treturn Ok[")
		b.WriteString(valueType)
		b.WriteString(", ")
		b.WriteString(okType)
		b.WriteString("](value)\n")
		b.WriteString("}()")
		return b.String(), retType, true, nil
	}
	return call, goMethodReturnType(methodSig.ret), true, nil
}
