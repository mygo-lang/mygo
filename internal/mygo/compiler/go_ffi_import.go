package compiler

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
	. "github.com/mygo-lang/mygo/prelude"
)

// goModuleImporter resolves Go packages for FFI signature collection.  The
// compiled-package importer (importer.Default) covers GOROOT and GOPATH
// installs, which handles the standard-library go: imports.  Module-aware
// packages — source directories inside the current module or dependencies in
// the module cache — are resolved through the MyGO module lookup
// (bootstrapGoImportDir, defined in go_ffi_import.mygo) and type-checked from
// source.  Everything stays inside the compiler binary: no go-command
// subprocess is spawned, so FFI resolution keeps working when the compiler
// builds packages in other modules or on machines without a Go toolchain.
//
// types.Importer demands a method set, which MyGO FFI cannot implement, so
// this adapter is toplevel Go; the resolution policy itself lives in
// go_ffi_import.mygo.
type goModuleImporter struct {
	dir      string
	fset     *token.FileSet
	compiled types.Importer
	cache    map[string]*types.Package
}

func newGoModuleImporter(dir string) *goModuleImporter {
	fset := token.NewFileSet()
	return &goModuleImporter{
		dir:  dir,
		fset: fset,
		// A single compiled importer instance keeps one package cache, so a
		// directly imported stdlib package and the same package referenced
		// transitively through another stdlib package resolve to the same
		// *types.Package; otherwise go/types would reject identical types as
		// mismatched (for example token.TYPE vs GenDecl.Tok).
		compiled: importer.ForCompiler(fset, "gc", nil),
		cache:    map[string]*types.Package{},
	}
}

// Import implements types.Importer.
func (m *goModuleImporter) Import(path string) (*types.Package, error) {
	if pkg, ok := m.cache[path]; ok {
		return pkg, nil
	}
	pkg, err := m.importOnce(path)
	if err != nil {
		return nil, err
	}
	m.cache[path] = pkg
	return pkg, nil
}

func (m *goModuleImporter) importOnce(path string) (*types.Package, error) {
	if pkg, err := m.compiled.Import(path); err == nil {
		return pkg, nil
	}
	var dir string
	switch d := bootstrapGoImportDir(m.dir, path).(type) {
	case Option__Some[string]:
		dir = d.F0
	case Option__None[string]:
		return nil, fmt.Errorf("can't find import %q from %q", path, m.dir)
	}
	return m.typeCheckDir(dir)
}

// typeCheckDir parses every non-test .go file in dir and type-checks them as
// a single package, resolving sub-imports through the same module-aware
// importer.
func (m *goModuleImporter) typeCheckDir(dir string) (*types.Package, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	pkgName := ""
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(m.fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, err
		}
		if pkgName == "" {
			pkgName = file.Name.Name
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no Go source files in %s", dir)
	}
	conf := types.Config{Importer: m}
	return conf.Check(pkgName, m.fset, files, nil)
}

// bootstrapLoadGoPackageGo loads a Go package's exported functions and types
// for FFI signature collection.  dir is the module context (workspace root or
// package dir) used when the import is resolved through go.mod rules.
func bootstrapLoadGoPackageGo(dir, importPath string) (BootstrapGoPackageInfo, error) {
	pkg, err := newGoModuleImporter(dir).Import(importPath)
	if err != nil {
		return BootstrapGoPackageInfo{}, err
	}
	typeString := func(t types.Type) string {
		return types.TypeString(t, func(p *types.Package) string { return p.Name() })
	}
	goTupleTypes := func(tuple *types.Tuple) []string {
		items := make([]string, tuple.Len())
		for i := range items {
			items[i] = typeString(tuple.At(i).Type())
		}
		return items
	}
	scope := pkg.Scope()
	funcs := []typeinference2.GoFuncSignature{}
	typeSigs := []typeinference2.GoTypeSignature{}
	// First pass: exported type aliases (`type Expr = ast.Expr`) are the FFI
	// surface MyGO names them by.  Register each alias as a type signature and
	// remember how its unaliased target renders so signature strings that use
	// the target (`ast.Expr`) can be rewritten to the alias name (`Expr`).
	// Without this the alias types never enter the Go FFI type table and
	// annotations such as `goast.Decl` fall back to a bare TCon that cannot
	// unify with the package's own function signatures.
	aliasStrings := map[string]string{}
	for _, name := range scope.Names() {
		if !isExportedGoName(name) {
			continue
		}
		obj := scope.Lookup(name)
		tn, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		alias, ok := tn.Type().(*types.Alias)
		if !ok {
			continue
		}
		aliasStrings[typeString(types.Unalias(alias))] = name
		methods := []typeinference2.GoFuncSignature{}
		if named, ok := types.Unalias(alias).(*types.Named); ok {
			methods = goTypeMethods(named, goTupleTypes)
		}
		typeSigs = append(typeSigs, typeinference2.GoTypeSignature{TypeName: name, TypeParams: []string{}, Methods: methods})
	}
	for _, name := range scope.Names() {
		switch obj := scope.Lookup(name).(type) {
		case *types.Func:
			sig, ok := obj.Type().(*types.Signature)
			if !ok {
				continue
			}
			funcs = append(funcs, typeinference2.GoFuncSignature{Name: name, Params: rewriteGoAliasStrings(goTupleTypes(sig.Params()), aliasStrings), Results: rewriteGoAliasStrings(goTupleTypes(sig.Results()), aliasStrings), Variadic: sig.Variadic()})
		case *types.TypeName:
			if _, isAlias := obj.Type().(*types.Alias); isAlias {
				// Registered in the first pass above.
				continue
			}
			named, ok := obj.Type().(*types.Named)
			if !ok {
				continue
			}
			params := named.TypeParams()
			paramNames := make([]string, 0, params.Len())
			for j := 0; j < params.Len(); j++ {
				paramNames = append(paramNames, params.At(j).Obj().Name())
			}
			typeSigs = append(typeSigs, typeinference2.GoTypeSignature{TypeName: name, TypeParams: paramNames, Methods: goTypeMethods(named, goTupleTypes)})
		}
	}
	return BootstrapGoPackageInfo{Funcs: funcs, Types: typeSigs}, nil
}

// goTypeMethods collects the exported pointer-method set of a named type as FFI
// signatures, mirroring the previous inline loop.
func goTypeMethods(named *types.Named, tupleTypes func(*types.Tuple) []string) []typeinference2.GoFuncSignature {
	methods := []typeinference2.GoFuncSignature{}
	methodSet := types.NewMethodSet(types.NewPointer(named))
	for j := 0; j < methodSet.Len(); j++ {
		fn, ok := methodSet.At(j).Obj().(*types.Func)
		if !ok || !fn.Exported() {
			continue
		}
		sig, ok := fn.Type().(*types.Signature)
		if !ok {
			continue
		}
		methods = append(methods, typeinference2.GoFuncSignature{Name: fn.Name(), Params: tupleTypes(sig.Params()), Results: tupleTypes(sig.Results()), Variadic: sig.Variadic()})
	}
	return methods
}

// rewriteGoAliasStrings replaces each alias target (`ast.Expr`) with the
// package's own alias name (`Expr`) so the rendered signature type matches the
// FFI-facing annotation `goast.Expr`.  Targets appear verbatim inside
// composites (`[]ast.Decl`, `map[ast.Expr]ast.Decl`), so a plain substring
// replacement of the fully qualified target is sufficient.
func rewriteGoAliasStrings(items []string, aliases map[string]string) []string {
	out := make([]string, len(items))
	copy(out, items)
	for target, alias := range aliases {
		for i := range out {
			out[i] = strings.ReplaceAll(out[i], target, alias)
		}
	}
	return out
}

func isExportedGoName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}
