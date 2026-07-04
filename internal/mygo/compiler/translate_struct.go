package compiler

import (
	jen "github.com/dave/jennifer/jen"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

func (g *generator) translateStructLit(n *StructLitExpr, ctx *exprCtx, expected string) (jen.Code, string, error) {
	st := g.pkg.Structs[n.TypeName]
	if st == nil {
		return nil, "", common.ErrorAtPos(n.Line, n.Column, "unknown struct type %s", n.TypeName)
	}
	typeName := sanitizeIdent(n.TypeName)
	subst := map[string]string{}
	if len(n.TypeArgs) > 0 {
		if len(st.TypeParams) != len(n.TypeArgs) {
			return nil, "", common.ErrorAtPos(n.Line, n.Column, "struct %s: type arity mismatch", n.TypeName)
		}
		for i, tp := range st.TypeParams {
			subst[tp] = g.goType(n.TypeArgs[i], ctx.typeParams)
		}
	} else if len(st.TypeParams) > 0 {
		if base, args := splitTypeArgs(expected); base == typeName && len(args) == len(st.TypeParams) {
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
			return nil, "", common.ErrorAtPos(f.Line, f.Column, "unknown field %s on struct %s", f.Name, n.TypeName)
		}
		fieldExpected := typeString(fieldDecl.Type, subst)
		code, typ, err := g.translateExpr(f.Value, ctx, fieldExpected)
		if err != nil {
			return nil, "", err
		}
		_ = code
		unifyType(fieldDecl.Type, typ, typeParamSet(st.TypeParams), subst)
	}
	if len(st.TypeParams) > 0 {
		for _, tp := range st.TypeParams {
			if subst[tp] == "" {
				return nil, "", common.ErrorAtPos(n.Line, n.Column, "struct %s: could not infer type parameters", n.TypeName)
			}
		}
	}
	fieldTypes := map[string]string{}
	for _, f := range st.Fields {
		fieldTypes[f.Name] = typeString(f.Type, subst)
	}
	parts := make(jen.Dict, len(n.Fields))
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
			return nil, "", common.ErrorAtPos(f.Line, f.Column, "unknown field %s on struct %s", f.Name, n.TypeName)
		}
		code, _, err := g.translateExpr(f.Value, ctx, fieldType)
		if err != nil {
			return nil, "", err
		}
		key := exportName(f.Name)
		if f.Name == "embed" {
			key = fieldType
		}
		fieldKey := jen.Id(key)
		parts[fieldKey] = code
	}
	if len(n.TypeArgs) > 0 {
		var args []jen.Code
		for _, arg := range n.TypeArgs {
			args = append(args, jen.Id(g.goType(arg, ctx.typeParams)))
		}
		return jen.Lit(jen.DictFunc(func(d jen.Dict) {
			for k, v := range parts {
				d[k] = v
			}
		})).Index(args...), typeString(&NamedType{Name: n.TypeName, Args: n.TypeArgs}, nil), nil
	} else if len(st.TypeParams) > 0 {
		var args []jen.Code
		for _, tp := range st.TypeParams {
			args = append(args, jen.Id(subst[tp]))
		}
		return jen.Lit(jen.DictFunc(func(d jen.Dict) {
			for k, v := range parts {
				d[k] = v
			}
		})).Index(args...), typeString(&NamedType{Name: n.TypeName, Args: n.TypeArgs}, nil), nil
	}
	return jen.Lit(jen.DictFunc(func(d jen.Dict) {
		for k, v := range parts {
			d[k] = v
		}
	})).Index(jen.Id(typeName)), typeString(&NamedType{Name: n.TypeName, Args: n.TypeArgs}, nil), nil
}
