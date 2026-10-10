package codegen2

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/metadata"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
	"github.com/mygo-lang/mygo/prelude"
)

// dynamicDispatchDecls returns the source level signatures of the ordinary Go
// APIs generated from dependency metadata. Bootstrap adds these declarations
// to external inference context before checking caller source, so the APIs are
// usable from MyGO without introducing new syntax.
func dynamicDispatchDecls(packageName string, files []SourceFileInput, dependencies []typeinference2.MyGoPackageInfo) ([]ast2.Decl, error) {
	return dynamicDispatchDeclsWithLocal(packageName, "", files, dependencies)
}

func dynamicDispatchDeclsWithLocal(packageName, packageImportPath string, files []SourceFileInput, dependencies []typeinference2.MyGoPackageInfo) ([]ast2.Decl, error) {
	source, err := dynamicDispatchSourceWithLocal(packageName, packageImportPath, files, dependencies)
	if err != nil {
		return nil, err
	}
	if err := diagnoseDynamicAPIRequests(files, dependencies, source); err != nil {
		return nil, err
	}
	if source == "" {
		return nil, nil
	}
	file, err := parser.ParseFile(token.NewFileSet(), "zz_mygo_dynamic.gen.go", source, parser.AllErrors)
	if err != nil {
		return nil, fmt.Errorf("parse generated dynamic API: %w", err)
	}
	decls := make([]ast2.Decl, 0, len(file.Decls))
	for _, imported := range file.Imports {
		alias := ""
		if imported.Name != nil {
			alias = imported.Name.Name
		}
		importPath, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("dynamic API import path: %w", err)
		}
		inferencePath := dynamicInferenceImportPath(importPath, files, dependencies)
		decls = append(decls, ast2.Decl__ImportDecl__Ctor(alias, inferencePath))
	}
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.FuncDecl:
			converted, err := dynamicGoFuncToMygo(value)
			if err != nil {
				return nil, fmt.Errorf("dynamic API %s: %w", value.Name.Name, err)
			}
			decls = append(decls, converted)
		case *ast.GenDecl:
			if value.Tok != token.TYPE {
				continue
			}
			for _, specification := range value.Specs {
				alias, ok := specification.(*ast.TypeSpec)
				if !ok || !alias.Assign.IsValid() {
					continue
				}
				target, err := dynamicGoTypeExprToMygo(alias.Type)
				if err != nil {
					return nil, fmt.Errorf("dynamic API type alias %s: %w", alias.Name.Name, err)
				}
				decls = append(decls, ast2.Decl__TypeAliasDecl__Ctor(alias.Name.Name, nil, target))
			}
		}
	}
	return decls, nil
}

// The generated Go source uses ordinary import paths, while bootstrap
// inference distinguishes Go FFI imports with the `go:` marker. Recover that
// marker from the producer package metadata when exposing generated dynamic
// API declarations to inference.
func dynamicInferenceImportPath(importPath string, files []SourceFileInput, dependencies []typeinference2.MyGoPackageInfo) string {
	for _, file := range files {
		for _, declaration := range file.File.Decls {
			importDecl, ok := declaration.(ast2.Decl__ImportDecl)
			if ok && strings.HasPrefix(importDecl.F1, "go:") && strings.TrimPrefix(importDecl.F1, "go:") == importPath {
				return "go:" + importPath
			}
		}
	}
	for _, dependency := range dependencies {
		if dependency.DynamicMetadataJSON != "" {
			var value metadata.PackageMetadata
			if json.Unmarshal([]byte(dependency.DynamicMetadataJSON), &value) == nil {
				for _, imported := range value.Imports {
					if imported.Go && imported.Path == importPath {
						return "go:" + importPath
					}
				}
			}
		}
		for _, declaration := range dependency.Decls {
			importDecl, ok := declaration.(ast2.Decl__ImportDecl)
			if ok && strings.TrimPrefix(importDecl.F1, "go:") == importPath && strings.HasPrefix(importDecl.F1, "go:") {
				return "go:" + importPath
			}
		}
	}
	return importPath
}

func dynamicGoFuncToMygo(function *ast.FuncDecl) (ast2.Decl, error) {
	typeParams := make([]string, 0)
	if function.Type.TypeParams != nil {
		for _, field := range function.Type.TypeParams.List {
			for _, name := range field.Names {
				typeParams = append(typeParams, name.Name)
			}
		}
	}
	params := make([]ast2.Param, 0)
	if function.Type.Params != nil {
		for _, field := range function.Type.Params.List {
			typ, err := dynamicGoTypeExprToMygo(field.Type)
			if err != nil {
				return nil, err
			}
			for _, name := range field.Names {
				params = append(params, ast2.Param{Name: name.Name, Type: typ})
			}
		}
	}
	var result prelude.Option[ast2.TypeExpr] = prelude.None[ast2.TypeExpr]()
	if function.Type.Results != nil && len(function.Type.Results.List) > 0 {
		results := make([]ast2.TypeExpr, 0)
		for _, field := range function.Type.Results.List {
			typ, err := dynamicGoTypeExprToMygo(field.Type)
			if err != nil {
				return nil, err
			}
			count := len(field.Names)
			if count == 0 {
				count = 1
			}
			for range count {
				results = append(results, typ)
			}
		}
		var resultType ast2.TypeExpr
		if len(results) == 1 {
			resultType = results[0]
		} else {
			resultType = ast2.TypeExpr__TupleType__Ctor(results)
		}
		result = prelude.Some[ast2.TypeExpr](resultType)
	}
	body := ast2.Expr{Kind: ast2.ExprKind__UnitExpr{}}
	if resultType, ok := result.(prelude.Option__Some[ast2.TypeExpr]); ok {
		body = ast2.Expr{Kind: ast2.ExprKind__InlineGoExpr__Ctor(&resultType.F0, "panic(\"unreachable dynamic API declaration\")", nil, nil)}
	}
	return ast2.Decl__FuncDecl__Ctor(function.Name.Name, typeParams, params, result, body, nil), nil
}

func dynamicGoTypeExprToMygo(value ast.Expr) (ast2.TypeExpr, error) {
	switch typ := value.(type) {
	case *ast.Ident:
		return ast2.TypeExpr__NamedType__Ctor(dynamicMygoBuiltin(typ.Name), nil), nil
	case *ast.SelectorExpr:
		base, ok := typ.X.(*ast.Ident)
		if !ok {
			return nil, fmt.Errorf("unsupported qualified Go type %T", typ.X)
		}
		return ast2.TypeExpr__NamedType__Ctor(base.Name+"."+typ.Sel.Name, nil), nil
	case *ast.IndexExpr:
		return dynamicGoAppliedType(typ.X, []ast.Expr{typ.Index})
	case *ast.IndexListExpr:
		return dynamicGoAppliedType(typ.X, typ.Indices)
	case *ast.ArrayType:
		if typ.Len != nil {
			return nil, fmt.Errorf("fixed Go arrays are not representable in a MyGO dynamic API signature")
		}
		element, err := dynamicGoTypeExprToMygo(typ.Elt)
		if err != nil {
			return nil, err
		}
		return ast2.TypeExpr__NamedType__Ctor("Slice", []ast2.TypeExpr{element}), nil
	case *ast.StarExpr:
		element, err := dynamicGoTypeExprToMygo(typ.X)
		if err != nil {
			return nil, err
		}
		return ast2.TypeExpr__NamedType__Ctor("Ref", []ast2.TypeExpr{element}), nil
	case *ast.ChanType:
		element, err := dynamicGoTypeExprToMygo(typ.Value)
		if err != nil {
			return nil, err
		}
		name := "Chan"
		switch typ.Dir {
		case ast.SEND:
			name = "SendChan"
		case ast.RECV:
			name = "RecvChan"
		}
		return ast2.TypeExpr__NamedType__Ctor(name, []ast2.TypeExpr{element}), nil
	case *ast.MapType:
		key, err := dynamicGoTypeExprToMygo(typ.Key)
		if err != nil {
			return nil, err
		}
		value, err := dynamicGoTypeExprToMygo(typ.Value)
		if err != nil {
			return nil, err
		}
		return ast2.TypeExpr__NamedType__Ctor("Map", []ast2.TypeExpr{key, value}), nil
	case *ast.StructType:
		if typ.Fields == nil || len(typ.Fields.List) == 0 {
			return ast2.TypeExpr__UnitType__Ctor(), nil
		}
		return nil, fmt.Errorf("non-empty Go structs are not representable in a MyGO dynamic API signature")
	case *ast.FuncType:
		params, err := dynamicGoFieldTypes(typ.Params)
		if err != nil {
			return nil, err
		}
		results, err := dynamicGoFieldTypes(typ.Results)
		if err != nil {
			return nil, err
		}
		var result ast2.TypeExpr = ast2.TypeExpr__UnitType__Ctor()
		if len(results) == 1 {
			result = results[0]
		} else if len(results) > 1 {
			result = ast2.TypeExpr__TupleType__Ctor(results)
		}
		return ast2.TypeExpr__FuncType__Ctor(params, &result), nil
	case *ast.ParenExpr:
		return dynamicGoTypeExprToMygo(typ.X)
	default:
		return nil, fmt.Errorf("unsupported Go type expression %T", value)
	}
}

func dynamicGoAppliedType(base ast.Expr, arguments []ast.Expr) (ast2.TypeExpr, error) {
	name, err := dynamicGoTypeName(base)
	if err != nil {
		return nil, err
	}
	args := make([]ast2.TypeExpr, 0, len(arguments))
	for _, argument := range arguments {
		typ, err := dynamicGoTypeExprToMygo(argument)
		if err != nil {
			return nil, err
		}
		args = append(args, typ)
	}
	return ast2.TypeExpr__NamedType__Ctor(name, args), nil
}

func dynamicGoTypeName(value ast.Expr) (string, error) {
	switch typ := value.(type) {
	case *ast.Ident:
		return dynamicMygoBuiltin(typ.Name), nil
	case *ast.SelectorExpr:
		base, ok := typ.X.(*ast.Ident)
		if !ok {
			return "", fmt.Errorf("unsupported qualified generic type %T", typ.X)
		}
		return base.Name + "." + typ.Sel.Name, nil
	default:
		return "", fmt.Errorf("unsupported generic type base %T", value)
	}
}

func dynamicGoFieldTypes(fields *ast.FieldList) ([]ast2.TypeExpr, error) {
	if fields == nil {
		return nil, nil
	}
	var output []ast2.TypeExpr
	for _, field := range fields.List {
		typ, err := dynamicGoTypeExprToMygo(field.Type)
		if err != nil {
			return nil, err
		}
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for range count {
			output = append(output, typ)
		}
	}
	return output, nil
}

func dynamicMygoBuiltin(name string) string {
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
	case "error":
		return "Error"
	case "any":
		return "Any"
	default:
		return name
	}
}
