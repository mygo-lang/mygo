package compiler

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	jen "github.com/dave/jennifer/jen"
	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
	parserpkg "github.com/mygo-lang/mygo/internal/mygo/parser"
)

func loadPreludeDecls() ([]Decl, error) {
	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("prelude: unable to locate compiler source")
	}
	// helpers.go lives at internal/mygo/compiler/helpers.go
	// project root is at:  ../..  from compiler/
	candidate := filepath.Join(filepath.Dir(filePath), "..", "..", "..", "prelude", "prelude.mygo")
	src, err := os.ReadFile(candidate)
	if err != nil {
		return nil, fmt.Errorf("prelude: %w", err)
	}
	file, err := parserpkg.ParseFile(string(src))
	if err != nil {
		return nil, fmt.Errorf("prelude: %w", err)
	}
	return file.Decls, nil
}

func hasImportPath(imports []importSpec, path string) bool {
	for _, imp := range imports {
		if importPathForGo(imp.Path) == path {
			return true
		}
	}
	return false
}

func nodeLineFromExprSlice(exprs []Expr) int {
	for _, e := range exprs {
		if l, _ := common.NodePos(e); l != 0 {
			return l
		}
	}
	return 0
}

func nodeColFromExprSlice(exprs []Expr) int {
	for _, e := range exprs {
		_, c := common.NodePos(e)
		if c != 0 {
			return c
		}
	}
	return 0
}

func isAnyType(t types.Type) bool {
	if t == nil {
		return false
	}
	if iface, ok := t.Underlying().(*types.Interface); ok {
		return iface.NumMethods() == 0 && iface.NumEmbeddeds() == 0
	}
	return false
}

func goMethodReturnType(ret []string) string {
	if len(ret) == 0 {
		return ""
	}
	if len(ret) == 1 {
		return ret[0]
	}
	return "(" + strings.Join(ret, ", ") + ")"
}

func goMyGoTypeString(t types.Type) string {
	if t == nil {
		return "any"
	}
	switch tt := t.(type) {
	case *types.Pointer:
		return "Ref[" + goMyGoTypeString(tt.Elem()) + "]"
	case *types.Basic:
		switch tt.Kind() {
		case types.Int:
			return "Int"
		case types.Int8:
			return "Int8"
		case types.Int16:
			return "Int16"
		case types.Int32:
			return "Int32"
		case types.Int64:
			return "Int64"
		case types.Uint:
			return "UInt"
		case types.Uint8:
			return "UInt8"
		case types.Uint16:
			return "UInt16"
		case types.Uint32:
			return "UInt32"
		case types.Uint64:
			return "UInt64"
		case types.Float32:
			return "Float32"
		case types.Float64:
			return "Float64"
		case types.String:
			return "String"
		case types.Bool:
			return "Bool"
		}
	case *types.Named:
		if obj := tt.Obj(); obj != nil && obj.Pkg() != nil {
			return obj.Pkg().Name() + "." + obj.Name()
		}
		return tt.Obj().Name()
	}
	return t.String()
}

func importAliasForPath(path string) string {
	path = importPathForGo(path)
	if path == "" {
		return ""
	}
	path = strings.TrimSuffix(path, "/")
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return sanitizeIdent(path[idx+1:])
	}
	return sanitizeIdent(path)
}

func importPathForGo(path string) string {
	return strings.TrimPrefix(path, "go:")
}

func (g *generator) isImportAlias(name string) bool {
	if g == nil {
		return false
	}
	_, ok := g.importAliases[name]
	return ok
}

func (g *generator) translateGoPackageSelector(alias, name string) (jen.Code, string, bool, error) {
	path, ok := g.pkg.ImportAliases[alias]
	if !ok || !strings.HasPrefix(importPathForGo(path), "") {
		return nil, "", false, nil
	}
	if !isExportedIdent(name) {
		return nil, "", false, nil
	}
	goPath := importPathForGo(path)
	sigs, err := g.goPackageSigsFor(goPath)
	if err != nil {
		return nil, "", false, err
	}
	if sigs.pkg == nil {
		return nil, "", false, nil
	}
	obj := sigs.pkg.Scope().Lookup(name)
	if obj == nil {
		return nil, "", false, nil
	}
	switch o := obj.(type) {
	case *types.Var, *types.Const:
		return jen.Qual(goPath, name), goMyGoTypeString(o.Type()), true, nil
	case *types.TypeName:
		return jen.Qual(goPath, name), goMyGoTypeString(o.Type()), true, nil
	default:
		return nil, "", false, nil
	}
}

func (g *generator) translateMyGoPackageSelector(alias, name string) (jen.Code, string, bool, error) {
	path, ok := g.pkg.ImportAliases[alias]
	if !ok || strings.HasPrefix(path, "go:") {
		return nil, "", false, nil
	}
	if !isExportedIdent(name) {
		return nil, "", false, nil
	}
	pkg, err := loadImportedMyGoPackage(g.pkg.WorkspaceRoot, g.pkg.Dir, path, g.pkg.NoPrelude)
	if err != nil {
		return nil, "", false, err
	}
	if fn, ok := pkg.Funcs[name]; ok {
		return jen.Id(alias).Dot(exportName(name)), g.goType(fn.Ret, nil), true, nil
	}
	if pkg.Structs[name] != nil {
		return jen.Id(alias).Dot(exportName(name)), exportName(name), true, nil
	}
	if pkg.Enums[name] != nil {
		return jen.Id(alias).Dot(exportName(name)), exportName(name), true, nil
	}
	return nil, "", false, nil
}

func (g *generator) translateMyGoSelectorCall(alias, name string, args []Expr, ctx *exprCtx, expected string) (jen.Code, string, bool, error) {
	path, ok := g.pkg.ImportAliases[alias]
	if !ok || strings.HasPrefix(path, "go:") {
		return nil, "", false, nil
	}
	if !isExportedIdent(name) {
		return nil, "", false, nil
	}
	pkg, err := loadImportedMyGoPackage(g.pkg.WorkspaceRoot, g.pkg.Dir, path, g.pkg.NoPrelude)
	if err != nil {
		return nil, "", false, err
	}
	sig, ok := pkg.Funcs[name]
	if !ok {
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
	return jen.Id(alias).Dot(exportName(name)).Call(argCodes...), g.goType(sig.Ret, nil), true, nil
}
