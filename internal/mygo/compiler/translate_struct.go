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
	translated := make(map[string]jen.Code, len(n.Fields))
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
		fieldExpected := myGoTypeString(substituteTypeExpr(fieldDecl.Type, subst), nil)
		code, typ, err := g.translateExpr(f.Value, ctx, fieldExpected)
		if err != nil {
			return nil, "", err
		}
		unifyType(fieldDecl.Type, typ, typeParamSet(st.TypeParams), subst)
		translated[f.Name] = code
	}
	if len(st.TypeParams) > 0 {
		for _, tp := range st.TypeParams {
			if subst[tp] == "" {
				return nil, "", common.ErrorAtPos(n.Line, n.Column, "struct %s: could not infer type parameters", n.TypeName)
			}
		}
	}
	parts := make(jen.Dict, len(n.Fields))
	for _, f := range n.Fields {
		key := f.Name
		if f.Name == "embed" {
			for _, stField := range st.Fields {
				if stField.Name == "embed" {
					key = myGoTypeString(stField.Type, subst)
					break
				}
			}
		}
		fieldKey := jen.Id(key)
		parts[fieldKey] = translated[f.Name]
	}
	if len(n.TypeArgs) > 0 {
		var args []jen.Code
		for _, arg := range n.TypeArgs {
			args = append(args, jen.Id(g.goType(arg, ctx.typeParams)))
		}
		return jen.Id(typeName).Index(args...).Values(parts), typeString(&NamedType{Name: n.TypeName, Args: n.TypeArgs}, nil), nil
	} else if len(st.TypeParams) > 0 {
		var args []jen.Code
		for _, tp := range st.TypeParams {
			args = append(args, jen.Id(subst[tp]))
		}
		return jen.Id(typeName).Index(args...).Values(parts), typeString(&NamedType{Name: n.TypeName, Args: n.TypeArgs}, nil), nil
	}
	return jen.Id(typeName).Values(parts), typeString(&NamedType{Name: n.TypeName, Args: n.TypeArgs}, nil), nil
}

func substituteTypeExpr(t TypeExpr, subst map[string]string) TypeExpr {
	switch tt := t.(type) {
	case *NamedType:
		if subst != nil {
			if repl, ok := subst[tt.Name]; ok && len(tt.Args) == 0 {
				return &NamedType{Name: repl}
			}
		}
		args := make([]TypeExpr, 0, len(tt.Args))
		for _, a := range tt.Args {
			args = append(args, substituteTypeExpr(a, subst))
		}
		return &NamedType{Name: tt.Name, Args: args}
	case *FuncType:
		params := make([]TypeExpr, 0, len(tt.Params))
		for _, p := range tt.Params {
			params = append(params, substituteTypeExpr(p, subst))
		}
		return &FuncType{Params: params, Ret: substituteTypeExpr(tt.Ret, subst)}
	case *TupleType:
		elems := make([]TypeExpr, 0, len(tt.Elems))
		for _, e := range tt.Elems {
			elems = append(elems, substituteTypeExpr(e, subst))
		}
		return &TupleType{Elems: elems}
	default:
		return t
	}
}
