package metadata

import (
	"fmt"
	"strings"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/prelude"
)

// ExportedDecls reconstructs the public MyGO declarations needed by the
// inference engine from package metadata. Function bodies and implementation
// bodies are intentionally absent; only their exported signatures are used
// when a package is imported.
func (m *PackageMetadata) ExportedDecls() ([]ast2.Decl, error) {
	if m == nil {
		return nil, fmt.Errorf("metadata: cannot reconstruct declarations from nil metadata")
	}
	ctx := astContext{packagePath: m.Package.ImportPath, aliases: make(map[string]string)}
	decls := make([]ast2.Decl, 0, len(m.Imports)+len(m.Exports.Types)+len(m.Exports.Aliases)+len(m.Exports.Functions)+len(m.Exports.Interfaces))
	for _, item := range m.Imports {
		if item.Alias == "" || item.Path == "" {
			return nil, fmt.Errorf("metadata: malformed import entry")
		}
		pathValue := item.Path
		if item.Go {
			pathValue = "go:" + pathValue
		}
		decls = append(decls, ast2.Decl__ImportDecl__Ctor(item.Alias, pathValue))
		ctx.aliases[item.Path] = item.Alias
	}
	for _, exported := range m.Exports.Types {
		name := exported.Name
		typeParams := typeParamNames(exported.TypeParams)
		switch exported.Kind {
		case "struct":
			fields := make([]ast2.Field, 0, len(exported.Fields))
			for _, field := range exported.Fields {
				typ, err := ctx.typeExpr(field.Type)
				if err != nil {
					return nil, fmt.Errorf("metadata: type %s field %s: %w", exported.Name, field.Name, err)
				}
				tag := prelude.Option[string](prelude.Option__None[string]{})
				if field.Tag != "" {
					tag = prelude.Option[string](prelude.Option__Some[string]{F0: field.Tag})
				}
				fields = append(fields, ast2.Field{Name: field.Name, Type: typ, Tag: tag})
			}
			decls = append(decls, ast2.Decl__StructDecl__Ctor(name, typeParams, fields))
		case "enum":
			variants := make([]ast2.Variant, 0, len(exported.Variants))
			for _, variant := range exported.Variants {
				fields := make([]ast2.TypeExpr, 0, len(variant.Fields))
				names := make([]string, 0, len(variant.Fields))
				for _, field := range variant.Fields {
					typ, err := ctx.typeExpr(field.Type)
					if err != nil {
						return nil, fmt.Errorf("metadata: enum %s variant %s: %w", exported.Name, variant.Name, err)
					}
					fields = append(fields, typ)
					names = append(names, field.Name)
				}
				variants = append(variants, ast2.Variant{Name: variant.Name, Fields: fields, Names: names, Named: fieldsHaveNames(variant.Fields)})
			}
			decls = append(decls, ast2.Decl__EnumDecl__Ctor(name, typeParams, variants))
		case "named", "defined":
			underlying := ast2.TypeExpr(ast2.TypeExpr__UnitType{})
			if exported.Underlying != nil {
				var err error
				underlying, err = ctx.typeExpr(*exported.Underlying)
				if err != nil {
					return nil, fmt.Errorf("metadata: type %s underlying type: %w", exported.Name, err)
				}
			}
			decls = append(decls, ast2.Decl__TypeDecl__Ctor(name, typeParams, underlying))
		default:
			return nil, fmt.Errorf("metadata: type %s has unsupported export kind %q", exported.Name, exported.Kind)
		}
	}
	for _, exported := range m.Exports.Aliases {
		target, err := ctx.typeExpr(exported.Target)
		if err != nil {
			return nil, fmt.Errorf("metadata: alias %s: %w", exported.Name, err)
		}
		decls = append(decls, ast2.Decl__TypeAliasDecl__Ctor(exported.Name, typeParamNames(exported.TypeParams), target))
	}
	for _, exported := range m.Exports.Functions {
		params, err := ctx.fieldsAsParams(exported.Parameters)
		if err != nil {
			return nil, fmt.Errorf("metadata: function %s parameters: %w", exported.Name, err)
		}
		ret, err := ctx.results(exported.Results)
		if err != nil {
			return nil, fmt.Errorf("metadata: function %s results: %w", exported.Name, err)
		}
		using, err := ctx.constraints(exported.Using)
		if err != nil {
			return nil, fmt.Errorf("metadata: function %s constraints: %w", exported.Name, err)
		}
		decls = append(decls, ast2.Decl__FuncDecl__Ctor(exported.Name, typeParamNames(exported.TypeParams), params, ret, ast2.Expr{Kind: ast2.ExprKind__UnitExpr{}}, using))
	}
	for _, exported := range m.Exports.Interfaces {
		methods := make([]ast2.FuncSig, 0, len(exported.Methods))
		for _, method := range exported.Methods {
			params, err := ctx.fieldsAsParams(method.Parameters)
			if err != nil {
				return nil, fmt.Errorf("metadata: interface %s method %s parameters: %w", exported.Name, method.Name, err)
			}
			ret, err := ctx.results(method.Results)
			if err != nil {
				return nil, fmt.Errorf("metadata: interface %s method %s results: %w", exported.Name, method.Name, err)
			}
			using, err := ctx.constraints(method.Using)
			if err != nil {
				return nil, fmt.Errorf("metadata: interface %s method %s constraints: %w", exported.Name, method.Name, err)
			}
			methods = append(methods, ast2.FuncSig{Name: method.Name, TypeParams: typeParamNames(method.TypeParams), Params: params, Ret: ret, Using: using})
		}
		decls = append(decls, ast2.Decl__InterfaceDecl__Ctor(exported.Name, typeParamNames(exported.TypeParams), methods))
	}
	return decls, nil
}

type astContext struct {
	packagePath string
	aliases     map[string]string
}

func (c astContext) typeExpr(value Type) (ast2.TypeExpr, error) {
	switch value.Kind {
	case "builtin":
		return ast2.TypeExpr__NamedType__Ctor(mygoBuiltin(value.Name), nil), nil
	case "typeParam":
		return ast2.TypeExpr__NamedType__Ctor(value.Name, nil), nil
	case "named":
		name := value.Name
		if value.Package != "" && value.Package != c.packagePath {
			alias, ok := c.aliases[value.Package]
			if !ok {
				return nil, fmt.Errorf("no package import alias for %q", value.Package)
			}
			name = alias + "." + name
		}
		args := make([]ast2.TypeExpr, 0, len(value.Arguments))
		for _, arg := range value.Arguments {
			typ, err := c.typeExpr(arg)
			if err != nil {
				return nil, err
			}
			args = append(args, typ)
		}
		return ast2.TypeExpr__NamedType__Ctor(name, args), nil
	case "slice", "reference":
		if value.Element == nil {
			return nil, fmt.Errorf("%s descriptor has no element", value.Kind)
		}
		item, err := c.typeExpr(*value.Element)
		if err != nil {
			return nil, err
		}
		name := "Slice"
		if value.Kind == "reference" {
			name = "Ref"
		}
		return ast2.TypeExpr__NamedType__Ctor(name, []ast2.TypeExpr{item}), nil
	case "map":
		if value.Key == nil || value.Value == nil {
			return nil, fmt.Errorf("map descriptor is incomplete")
		}
		key, err := c.typeExpr(*value.Key)
		if err != nil {
			return nil, err
		}
		item, err := c.typeExpr(*value.Value)
		if err != nil {
			return nil, err
		}
		return ast2.TypeExpr__NamedType__Ctor("Map", []ast2.TypeExpr{key, item}), nil
	case "tuple":
		items := make([]ast2.TypeExpr, 0, len(value.Arguments))
		for _, arg := range value.Arguments {
			typ, err := c.typeExpr(arg)
			if err != nil {
				return nil, err
			}
			items = append(items, typ)
		}
		if len(items) == 0 {
			return ast2.TypeExpr__UnitType__Ctor(), nil
		}
		return ast2.TypeExpr__TupleType__Ctor(items), nil
	case "function":
		params := make([]ast2.TypeExpr, 0, len(value.Parameters))
		for _, param := range value.Parameters {
			typ, err := c.typeExpr(param)
			if err != nil {
				return nil, err
			}
			params = append(params, typ)
		}
		var result ast2.TypeExpr = ast2.TypeExpr__UnitType__Ctor()
		if len(value.Results) == 1 {
			var err error
			result, err = c.typeExpr(value.Results[0])
			if err != nil {
				return nil, err
			}
		} else if len(value.Results) > 1 {
			items := make([]ast2.TypeExpr, 0, len(value.Results))
			for _, item := range value.Results {
				typ, err := c.typeExpr(item)
				if err != nil {
					return nil, err
				}
				items = append(items, typ)
			}
			result = ast2.TypeExpr__TupleType__Ctor(items)
		}
		return ast2.TypeExpr__FuncType__Ctor(params, &result), nil
	case "applied":
		args := make([]ast2.TypeExpr, 0, len(value.Arguments))
		for _, arg := range value.Arguments {
			typ, err := c.typeExpr(arg)
			if err != nil {
				return nil, err
			}
			args = append(args, typ)
		}
		return ast2.TypeExpr__NamedType__Ctor(value.Name, args), nil
	default:
		return nil, fmt.Errorf("unknown type descriptor kind %q", value.Kind)
	}
}

func (c astContext) fieldsAsParams(fields []Field) ([]ast2.Param, error) {
	params := make([]ast2.Param, 0, len(fields))
	for _, field := range fields {
		typ, err := c.typeExpr(field.Type)
		if err != nil {
			return nil, err
		}
		params = append(params, ast2.Param{Name: field.Name, Type: typ})
	}
	return params, nil
}

func (c astContext) results(results []Type) (prelude.Option[ast2.TypeExpr], error) {
	if len(results) == 0 {
		return prelude.Option[ast2.TypeExpr](prelude.Option__None[ast2.TypeExpr]{}), nil
	}
	items := make([]ast2.TypeExpr, 0, len(results))
	for _, result := range results {
		typ, err := c.typeExpr(result)
		if err != nil {
			return nil, err
		}
		items = append(items, typ)
	}
	var result ast2.TypeExpr
	if len(items) == 1 {
		result = items[0]
	} else {
		result = ast2.TypeExpr__TupleType__Ctor(items)
	}
	return prelude.Option[ast2.TypeExpr](prelude.Option__Some[ast2.TypeExpr]{F0: result}), nil
}

func (c astContext) constraints(values []Constraint) ([]ast2.Constraint, error) {
	result := make([]ast2.Constraint, 0, len(values))
	for _, value := range values {
		args := make([]ast2.TypeExpr, 0, len(value.Arguments))
		for _, arg := range value.Arguments {
			typ, err := c.typeExpr(arg)
			if err != nil {
				return nil, err
			}
			args = append(args, typ)
		}
		result = append(result, ast2.Constraint{Name: value.Interface, BindName: prelude.Option[string](prelude.Option__None[string]{}), Args: args})
	}
	return result, nil
}

func typeParamNames(values []TypeParam) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.Name)
	}
	return result
}

func fieldsHaveNames(fields []Field) bool {
	if len(fields) == 0 {
		return false
	}
	for _, field := range fields {
		if field.Name == "" {
			return false
		}
	}
	return true
}

func mygoBuiltin(name string) string {
	switch name {
	case "int":
		return "Int"
	case "int8":
		return "Int8"
	case "int16":
		return "Int16"
	case "int32":
		return "Int32"
	case "int64":
		return "Int64"
	case "uint":
		return "UInt"
	case "uint8":
		return "UInt8"
	case "uint16":
		return "UInt16"
	case "uint32":
		return "UInt32"
	case "uint64":
		return "UInt64"
	case "float32":
		return "Float32"
	case "float64":
		return "Float64"
	case "bool":
		return "Bool"
	case "string":
		return "String"
	case "any":
		return "Any"
	case "error":
		return "Error"
	default:
		return strings.TrimSpace(name)
	}
}
