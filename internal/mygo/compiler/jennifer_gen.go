package compiler

import (
	"strings"

	jen "github.com/dave/jennifer/jen"
	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

// genJenIds converts string param names to jen.Id(items) for use with bracketArgs.
func genJenIds(params []string) []jen.Code {
	items := make([]jen.Code, len(params))
	for i, p := range params {
		items[i] = jen.Id(p)
	}
	return items
}

// bracketArgs appends [item1, item2, ...] to stmt using Custom to avoid
// space-separated bracket tokens in the Statement.
func bracketArgs(stmt *jen.Statement, args []jen.Code) *jen.Statement {
	if len(args) == 0 {
		return stmt
	}
	opts := jen.Options{Open: "[", Close: "]", Separator: ", "}
	return stmt.Custom(opts, args...)
}

func jenTypeExpr(t TypeExpr) jen.Code {
	switch tt := t.(type) {
	case *NamedType:
		switch tt.Name {
		case "Int":
			return jen.Int()
		case "Int8":
			return jen.Int8()
		case "Int16":
			return jen.Int16()
		case "Int32":
			return jen.Int32()
		case "Int64":
			return jen.Int64()
		case "UInt":
			return jen.Uint()
		case "UInt8":
			return jen.Uint8()
		case "UInt16":
			return jen.Uint16()
		case "UInt32":
			return jen.Uint32()
		case "UInt64":
			return jen.Uint64()
		case "Float32":
			return jen.Float32()
		case "Float64":
			return jen.Float64()
		case "String":
			return jen.String()
		case "Bool":
			return jen.Bool()
		case "Unit":
			return jen.Struct()
		case "Ref":
			if len(tt.Args) == 1 {
				return jen.Op("*").Add(jenTypeExpr(tt.Args[0]))
			}
		}
		if len(tt.Args) == 0 {
			return jen.Id(tt.Name)
		}
		args := make([]jen.Code, 0, len(tt.Args))
		for _, a := range tt.Args {
			args = append(args, jenTypeExpr(a))
		}
		return bracketArgs(jen.Id(tt.Name), args)
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

// jenHKTTypeExpr renders a type expression, replacing HKT type parameters
// (e.g. C[A]) with HKT[C, A] encoding for use in interface method signatures.
// This is needed because Go does not allow indexing a type parameter (C[A] is invalid).
func jenHKTTypeExpr(t TypeExpr, hktSet map[string]struct{}) jen.Code {
	switch tt := t.(type) {
	case *NamedType:
		// Check if this named type is an HKT parameter in scope
		if len(tt.Args) > 0 && hktSet != nil {
			if _, ok := hktSet[tt.Name]; ok {
				// Emit HKT[C, A, ...] instead of C[A, ...].
				// Embed "HKT[" in Open so there's no Statement-imposed space.
				hktItems := make([]jen.Code, 0, len(tt.Args))
				for _, a := range tt.Args {
					hktItems = append(hktItems, jenHKTTypeExpr(a, hktSet))
				}
				hktItems = append([]jen.Code{jen.Id(tt.Name)}, hktItems...)
				opts := jen.Options{Open: "HKT[", Close: "]", Separator: ", "}
				return jen.Custom(opts, hktItems...)
			}
		}
		// Fall through to normal type handling
		switch tt.Name {
		case "Int":
			return jen.Int()
		case "Int8":
			return jen.Int8()
		case "Int16":
			return jen.Int16()
		case "Int32":
			return jen.Int32()
		case "Int64":
			return jen.Int64()
		case "UInt":
			return jen.Uint()
		case "UInt8":
			return jen.Uint8()
		case "UInt16":
			return jen.Uint16()
		case "UInt32":
			return jen.Uint32()
		case "UInt64":
			return jen.Uint64()
		case "Float32":
			return jen.Float32()
		case "Float64":
			return jen.Float64()
		case "String":
			return jen.String()
		case "Bool":
			return jen.Bool()
		case "Unit":
			return jen.Struct()
		case "Ref":
			if len(tt.Args) == 1 {
				return jen.Op("*").Add(jenHKTTypeExpr(tt.Args[0], hktSet))
			}
		}
		if len(tt.Args) == 0 {
			return jen.Id(tt.Name)
		}
		args := make([]jen.Code, 0, len(tt.Args))
		for _, a := range tt.Args {
			args = append(args, jenHKTTypeExpr(a, hktSet))
		}
		return bracketArgs(jen.Id(tt.Name), args)
	case *FuncType:
		params := make([]jen.Code, 0, len(tt.Params))
		for _, p := range tt.Params {
			params = append(params, jenHKTTypeExpr(p, hktSet))
		}
		ret := jenHKTTypeExpr(tt.Ret, hktSet)
		if isUnitType(tt.Ret) {
			return jen.Func().Params(params...)
		}
		return jen.Func().Params(params...).Add(ret)
	default:
		return jen.Id("any")
	}
}

// addTypeParams chains type parameter constraints [A any, B any] onto stmt.
// Uses Custom to avoid space-separated items in Statements.
func addTypeParams(stmt *jen.Statement, params []string) *jen.Statement {
	if len(params) == 0 {
		return stmt
	}
	opts := jen.Options{Open: "[", Close: "]", Separator: ", "}
	items := make([]jen.Code, 0, len(params))
	for _, p := range params {
		items = append(items, jen.Id(p).Id("any"))
	}
	return stmt.Custom(opts, items...)
}

// typeParamJenItems returns a *jen.Statement for type parameter constraints.
// Prefer addTypeParams for direct chaining to avoid spacing issues.
// WARNING: Using .Add(typeParamJenItems(...)) adds spaces around brackets.
func typeParamJenItems(params []string) *jen.Statement {
	if len(params) == 0 {
		return nil
	}
	opts := jen.Options{Open: "[", Close: "]", Separator: ", "}
	items := make([]jen.Code, 0, len(params))
	for _, p := range params {
		items = append(items, jen.Id(p).Id("any"))
	}
	return jen.Custom(opts, items...)
}

func jenTypeParams(params []string) *jen.Statement {
	return typeParamJenItems(params)
}

func jenTypeArgList(params []string) *jen.Statement {
	if len(params) == 0 {
		return nil
	}
	opts := jen.Options{Open: "[", Close: "]", Separator: ", "}
	items := make([]jen.Code, 0, len(params))
	for _, p := range params {
		items = append(items, jen.Id(p))
	}
	return jen.Custom(opts, items...)
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
