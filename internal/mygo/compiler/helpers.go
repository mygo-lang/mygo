package compiler

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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
		case types.Int64:
			return "Int64"
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

func (g *generator) translateGoPackageSelector(alias, name string) (string, string, bool, error) {
	path, ok := g.pkg.ImportAliases[alias]
	if !ok || !strings.HasPrefix(importPathForGo(path), "") {
		return "", "", false, nil
	}
	sigs, err := g.goPackageSigsFor(importPathForGo(path))
	if err != nil {
		return "", "", false, err
	}
	if sigs.pkg == nil {
		return "", "", false, nil
	}
	obj := sigs.pkg.Scope().Lookup(name)
	if obj == nil {
		return "", "", false, nil
	}
	switch o := obj.(type) {
	case *types.Var, *types.Const:
		return fmt.Sprintf("%s.%s", alias, name), goMyGoTypeString(o.Type()), true, nil
	case *types.TypeName:
		return fmt.Sprintf("%s.%s", alias, name), goMyGoTypeString(o.Type()), true, nil
	default:
		return "", "", false, nil
	}
}
