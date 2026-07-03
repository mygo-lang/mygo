package compiler

import (
	"strings"

	jen "github.com/dave/jennifer/jen"
	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

func jenTypeExpr(t TypeExpr) jen.Code {
	switch tt := t.(type) {
	case *NamedType:
		switch tt.Name {
		case "Int":
			return jen.Int()
		case "String":
			return jen.String()
		case "Bool":
			return jen.Bool()
		case "Float64":
			return jen.Float64()
		case "Int64":
			return jen.Int64()
		case "Unit":
			return jen.Struct()
		}
		if len(tt.Args) == 0 {
			return jen.Id(tt.Name)
		}
		args := make([]jen.Code, 0, len(tt.Args))
		for _, a := range tt.Args {
			args = append(args, jenTypeExpr(a))
		}
		return jen.Id(tt.Name).Index(args...)
	case *FuncType:
		params := make([]jen.Code, 0, len(tt.Params))
		for _, p := range tt.Params {
			params = append(params, jenTypeExpr(p))
		}
		ret := jenTypeExpr(tt.Ret)
		if isUnitType(tt.Ret) {
			return jen.Func().Params(params...)
		}
		return jen.Func().Params(params...).Add(ret)
	default:
		return jen.Id("any")
	}
}

func jenTypeParams(params []string) *jen.Statement {
	if len(params) == 0 {
		return nil
	}
	items := make([]jen.Code, 0, len(params))
	for _, p := range params {
		items = append(items, jen.Id(p), jen.Id("any"))
	}
	return jen.IndexFunc(func(g *jen.Group) {
		for i, p := range params {
			if i > 0 {
				g.Add(jen.Op(","))
			}
			g.Add(jen.Id(p), jen.Id("any"))
		}
	})
}

func jenTypeArgList(params []string) *jen.Statement {
	if len(params) == 0 {
		return nil
	}
	return jen.IndexFunc(func(g *jen.Group) {
		for i, p := range params {
			if i > 0 {
				g.Add(jen.Op(","))
			}
			g.Add(jen.Id(p))
		}
	})
}

func jenFieldName(name string) string {
	if name == "embed" {
		return ""
	}
	return exportName(name)
}

func jenJoinParts(parts ...jen.Code) []jen.Code {
	out := make([]jen.Code, 0, len(parts))
	for _, p := range parts {
		if p != nil {
			out = append(out, p)
		}
	}
	return out
}

func jenReceiverName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "x"
	}
	return sanitizeIdent(name)
}
