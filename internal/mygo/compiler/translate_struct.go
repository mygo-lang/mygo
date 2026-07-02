package compiler

import (
	"fmt"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func (g *generator) translateStructLit(n *StructLitExpr, ctx *exprCtx, expected string) (string, string, error) {
	st := g.pkg.Structs[n.TypeName]
	if st == nil {
		return "", "", common.ErrorAtPos(n.Line, n.Column, "unknown struct type %s", n.TypeName)
	}
	subst := map[string]string{}
	if len(n.TypeArgs) > 0 {
		if len(st.TypeParams) != len(n.TypeArgs) {
			return "", "", common.ErrorAtPos(n.Line, n.Column, "struct %s: type arity mismatch", n.TypeName)
		}
		for i, tp := range st.TypeParams {
			subst[tp] = g.goType(n.TypeArgs[i], ctx.typeParams)
		}
	} else if len(st.TypeParams) > 0 {
		if base, args := splitTypeArgs(expected); base == n.TypeName && len(args) == len(st.TypeParams) {
			for i, tp := range st.TypeParams {
				subst[tp] = args[i]
			}
		}
	}
	for _, f := range n.Fields {
		var fieldDecl *Field
		for i := range st.Fields {
			if st.Fields[i].Name == f.Name {
				fieldDecl = &st.Fields[i]
				break
			}
		}
		if fieldDecl == nil && f.Name == "embed" {
			for i := range st.Fields {
				if st.Fields[i].Name == "embed" {
					fieldDecl = &st.Fields[i]
					break
				}
			}
		}
		if fieldDecl == nil {
			return "", "", common.ErrorAtPos(f.Line, f.Column, "unknown field %s on struct %s", f.Name, n.TypeName)
		}
		fieldExpected := typeString(fieldDecl.Type, subst)
		code, typ, err := g.translateExpr(f.Value, ctx, fieldExpected)
		if err != nil {
			return "", "", err
		}
		_ = code
		unifyType(fieldDecl.Type, typ, typeParamSet(st.TypeParams), subst)
	}
	if len(st.TypeParams) > 0 {
		for _, tp := range st.TypeParams {
			if subst[tp] == "" {
				return "", "", common.ErrorAtPos(n.Line, n.Column, "struct %s: could not infer type parameters", n.TypeName)
			}
		}
	}
	fieldTypes := map[string]string{}
	for _, f := range st.Fields {
		fieldTypes[f.Name] = typeString(f.Type, subst)
	}
	parts := make([]string, 0, len(n.Fields))
	for _, f := range n.Fields {
		fieldType := fieldTypes[f.Name]
		if fieldType == "" && f.Name == "embed" {
			for _, stField := range st.Fields {
				if stField.Name == "embed" {
					fieldType = typeString(stField.Type, subst)
					break
				}
			}
		}
		if fieldType == "" {
			return "", "", common.ErrorAtPos(f.Line, f.Column, "unknown field %s on struct %s", f.Name, n.TypeName)
		}
		code, _, err := g.translateExpr(f.Value, ctx, fieldType)
		if err != nil {
			return "", "", err
		}
		key := exportName(f.Name)
		if f.Name == "embed" {
			key = fieldType
		}
		parts = append(parts, fmt.Sprintf("%s: %s", key, code))
	}
	typeArgStr := ""
	if len(n.TypeArgs) > 0 {
		var args []string
		for _, arg := range n.TypeArgs {
			args = append(args, g.goType(arg, ctx.typeParams))
		}
		typeArgStr = "[" + strings.Join(args, ", ") + "]"
	} else if len(st.TypeParams) > 0 {
		var args []string
		for _, tp := range st.TypeParams {
			args = append(args, subst[tp])
		}
		typeArgStr = "[" + strings.Join(args, ", ") + "]"
	}
	typeArgs := n.TypeArgs
	if len(typeArgs) == 0 && len(st.TypeParams) > 0 {
		typeArgs = make([]TypeExpr, 0, len(st.TypeParams))
		for _, tp := range st.TypeParams {
			typeArgs = append(typeArgs, &NamedType{Name: subst[tp]})
		}
	}
	return fmt.Sprintf("%s%s{%s}", n.TypeName, typeArgStr, strings.Join(parts, ", ")), typeString(&NamedType{Name: n.TypeName, Args: typeArgs}, nil), nil
}
