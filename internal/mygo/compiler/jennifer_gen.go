package compiler

import (
	"strconv"
	"strings"

	jen "github.com/dave/jennifer/jen"
	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

// genJenIds converts string param names to properly-typed jen.Code items
// for use with Types(). Handles pointer types (e.g., "*Document" → Op("*").Id("Document")).
func genJenIds(params []string) []jen.Code {
	items := make([]jen.Code, len(params))
	for i, p := range params {
		trimmed := strings.TrimSpace(p)
		if strings.HasPrefix(trimmed, "*") {
			inner := strings.TrimPrefix(trimmed, "*")
			items[i] = jen.Op("*").Add(jen.Id(inner))
		} else {
			items[i] = jen.Id(trimmed)
		}
	}
	return items
}

// bracketArgs appends generic type arguments using Jennifer's native Types()
// rendering so we get valid Go syntax like `Foo[A, B]`.
func bracketArgs(stmt *jen.Statement, args []jen.Code) *jen.Statement {
	if len(args) == 0 {
		return stmt
	}
	return stmt.Types(args...)
}

func preludeJenTypeName(name string) *jen.Statement {
	if name == "" {
		return jen.Id(name)
	}
	switch name {
	case "Option", "Result", "IEnumerable", "Show", "Eq", "IOption", "List", "HKTType", "HKT1", "HKT2", "HKT":
		return jen.Id(name)
	default:
		return jen.Id(name)
	}
}

func typeWithArgs(base *jen.Statement, args ...jen.Code) *jen.Statement {
	if len(args) == 0 {
		return base
	}
	return base.Types(args...)
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
		case "Slice":
			if len(tt.Args) == 1 {
				return jen.Index().Add(jenTypeExpr(tt.Args[0]))
			}
		case "Map":
			if len(tt.Args) == 2 {
				return jen.Map(jenTypeExpr(tt.Args[0])).Add(jenTypeExpr(tt.Args[1]))
			}
		case "Set":
			if len(tt.Args) == 1 {
				return jen.Map(jenTypeExpr(tt.Args[0])).Struct()
			}
		case "Any":
			return jen.Id("any")
		}
		if len(tt.Args) == 0 {
			return preludeJenTypeName(tt.Name)
		}
		args := make([]jen.Code, 0, len(tt.Args))
		for _, a := range tt.Args {
			args = append(args, jenTypeExpr(a))
		}
		return bracketArgs(preludeJenTypeName(tt.Name), args)
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
	case *TupleType:
		fields := make(jen.Dict)
		for i, e := range tt.Elems {
			fields[jen.Id("F"+strconv.Itoa(i))] = jenTypeExpr(e)
		}
		return jen.Struct(fields)
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
		case "Slice":
			if len(tt.Args) == 1 {
				return jen.Index().Add(jenHKTTypeExpr(tt.Args[0], hktSet))
			}
		case "Map":
			if len(tt.Args) == 2 {
				return jen.Map(jenHKTTypeExpr(tt.Args[0], hktSet)).Add(jenHKTTypeExpr(tt.Args[1], hktSet))
			}
		case "Set":
			if len(tt.Args) == 1 {
				return jen.Map(jenHKTTypeExpr(tt.Args[0], hktSet)).Struct()
			}
		case "Any":
			return jen.Id("any")
		}
		if len(tt.Args) == 0 {
			return preludeJenTypeName(tt.Name)
		}
		args := make([]jen.Code, 0, len(tt.Args))
		for _, a := range tt.Args {
			args = append(args, jenHKTTypeExpr(a, hktSet))
		}
		return bracketArgs(preludeJenTypeName(tt.Name), args)
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
	case *TupleType:
		fields := make(jen.Dict)
		for i, e := range tt.Elems {
			fields[jen.Id("F"+strconv.Itoa(i))] = jenHKTTypeExpr(e, hktSet)
		}
		return jen.Struct(fields)
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
	items := make([]jen.Code, 0, len(params))
	for _, p := range params {
		items = append(items, jen.Id(p).Id("any"))
	}
	return stmt.Types(items...)
}

// typeParamJenItems returns a *jen.Statement for type parameter constraints.
// Prefer addTypeParams for direct chaining to avoid spacing issues.
// WARNING: Using .Add(typeParamJenItems(...)) adds spaces around brackets.
func typeParamJenItems(params []string) *jen.Statement {
	if len(params) == 0 {
		return nil
	}
	items := make([]jen.Code, 0, len(params))
	for _, p := range params {
		items = append(items, jen.Id(p).Id("any"))
	}
	stmt := &jen.Statement{}
	return stmt.Types(items...)
}

func jenTypeParams(params []string) *jen.Statement {
	return typeParamJenItems(params)
}

func jenTypeArgList(params []string) *jen.Statement {
	if len(params) == 0 {
		return nil
	}
	items := make([]jen.Code, 0, len(params))
	for _, p := range params {
		items = append(items, jen.Id(p))
	}
	stmt := &jen.Statement{}
	return stmt.Types(items...)
}

func jenFieldName(name string) string {
	if name == "embed" {
		return ""
	}
	return sanitizeIdent(name)
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
