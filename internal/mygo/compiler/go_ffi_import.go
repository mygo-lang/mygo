package compiler

import (
	"fmt"
	"go/types"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"

	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
)

// bootstrapLoadGoPackageGo loads a Go package's exported functions and types
// for FFI signature collection.  It delegates to go/packages so module
// resolution, build constraints, and cgo preprocessing match the Go command
// line exactly.
func bootstrapLoadGoPackageGo(dir, importPath string) (BootstrapGoPackageInfo, error) {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedTypes,
		Dir:  dir,
	}, importPath)
	if err != nil {
		return BootstrapGoPackageInfo{}, err
	}
	if len(pkgs) != 1 {
		return BootstrapGoPackageInfo{}, fmt.Errorf("loaded %d packages for %q, want 1", len(pkgs), importPath)
	}
	pkg := pkgs[0]
	if len(pkg.Errors) != 0 {
		return BootstrapGoPackageInfo{}, fmt.Errorf("%s", pkg.Errors[0])
	}
	if pkg.Types == nil {
		return BootstrapGoPackageInfo{}, fmt.Errorf("package %q has no type information", importPath)
	}
	return bootstrapGoPackageInfoFromTypes(pkg.Types), nil
}

// bootstrapGoPackageInfoFromTypes collects the FFI signature surface from a
// type-checked Go package.
func bootstrapGoPackageInfoFromTypes(pkg *types.Package) BootstrapGoPackageInfo {
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
		fields := []typeinference2.GoFieldSignature{}
		underlying := ""
		if named, ok := types.Unalias(alias).(*types.Named); ok {
			methods = goTypeMethods(named, goTupleTypes)
			fields = goTypeFields(named, typeString)
			underlying = typeString(named.Underlying())
		}
		typeSigs = append(typeSigs, typeinference2.GoTypeSignature{TypeName: name, TypeParams: []string{}, Methods: methods, Fields: fields, Underlying: underlying})
	}
	for _, name := range scope.Names() {
		switch obj := scope.Lookup(name).(type) {
		case *types.Func:
			sig, ok := obj.Type().(*types.Signature)
			if !ok {
				continue
			}
			funcs = append(funcs, typeinference2.GoFuncSignature{Name: name, Params: rewriteGoAliasStrings(goTupleTypes(sig.Params()), aliasStrings), Results: rewriteGoAliasStrings(goTupleTypes(sig.Results()), aliasStrings), Variadic: sig.Variadic(), TypeParams: goFuncTypeParams(sig, nil)})
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
			typeSigs = append(typeSigs, typeinference2.GoTypeSignature{TypeName: name, TypeParams: paramNames, Methods: goTypeMethods(named, goTupleTypes), Fields: goTypeFields(named, typeString), Underlying: typeString(named.Underlying())})
		}
	}
	return BootstrapGoPackageInfo{Funcs: funcs, Types: typeSigs}
}

// goTypeFields collects the exported struct fields of a named type as FFI
// field signatures.  Unexported and embedded anonymous fields are skipped;
// embedded exported fields are included so promoted selectors that the source
// spells as ordinary field access still type-check.
func goTypeFields(named *types.Named, typeString func(types.Type) string) []typeinference2.GoFieldSignature {
	underlying, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil
	}
	fields := []typeinference2.GoFieldSignature{}
	for i := 0; i < underlying.NumFields(); i++ {
		f := underlying.Field(i)
		if !f.Exported() {
			continue
		}
		fields = append(fields, typeinference2.GoFieldSignature{Name: f.Name(), Type: typeString(f.Type())})
	}
	return fields
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
		methods = append(methods, typeinference2.GoFuncSignature{Name: fn.Name(), Params: tupleTypes(sig.Params()), Results: tupleTypes(sig.Results()), Variadic: sig.Variadic(), TypeParams: goFuncTypeParams(sig, named.TypeParams())})
	}
	return methods
}

// goFuncTypeParams extracts the generic type-parameter names that may appear
// bare inside a rendered signature, in declaration order.  A package-level
// generic function contributes its own parameters (`maps.Clone`'s M, K, V);
// a method on a generic type contributes the receiver type's parameters
// (`(*List[T]).Set`'s T), since go/types renders those bare in the method
// signature while the method itself declares no parameters.  The ids these
// names map to are positional, so declaration order must be preserved.
func goFuncTypeParams(sig *types.Signature, recvTypeParams *types.TypeParamList) []string {
	names := []string{}
	if recvTypeParams != nil {
		for i := 0; i < recvTypeParams.Len(); i++ {
			names = append(names, recvTypeParams.At(i).Obj().Name())
		}
	}
	if sig.TypeParams() != nil {
		for i := 0; i < sig.TypeParams().Len(); i++ {
			names = append(names, sig.TypeParams().At(i).Obj().Name())
		}
	}
	return names
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
