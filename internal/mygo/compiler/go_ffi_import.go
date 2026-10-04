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
	loaded, err := bootstrapLoadGoPackagesGo(dir, []string{importPath})
	if err != nil {
		return BootstrapGoPackageInfo{}, err
	}
	info, ok := loaded[importPath]
	if !ok {
		return BootstrapGoPackageInfo{}, fmt.Errorf("package %q was not returned by go/packages", importPath)
	}
	return info, nil
}

// bootstrapLoadGoPackagesGo loads all FFI roots in one go/packages operation.
// Shared dependencies must use the same *types.Package objects for Go type
// identity checks across interfaces.
func bootstrapLoadGoPackagesGo(dir string, importPaths []string) (map[string]BootstrapGoPackageInfo, error) {
	if len(importPaths) == 0 {
		return map[string]BootstrapGoPackageInfo{}, nil
	}
	uniquePaths := make([]string, 0, len(importPaths))
	seen := make(map[string]bool, len(importPaths))
	for _, path := range importPaths {
		if !seen[path] {
			seen[path] = true
			uniquePaths = append(uniquePaths, path)
		}
	}
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedTypes,
		Dir:  dir,
	}, uniquePaths...)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != len(uniquePaths) {
		return nil, fmt.Errorf("loaded %d packages for %d Go FFI imports", len(pkgs), len(uniquePaths))
	}
	loaded := make(map[string]BootstrapGoPackageInfo, len(pkgs))
	for _, pkg := range pkgs {
		if len(pkg.Errors) != 0 {
			return nil, fmt.Errorf("%s", pkg.Errors[0])
		}
		if pkg.Types == nil {
			return nil, fmt.Errorf("package %q has no type information", pkg.PkgPath)
		}
		typeinference2.RegisterGoPackage(pkg.Types)
		loaded[pkg.Types.Path()] = bootstrapGoPackageInfoFromTypes(pkg.Types)
	}
	return loaded, nil
}

// bootstrapGoPackageInfoFromTypes collects the FFI signature surface from a
// type-checked Go package.
func bootstrapGoPackageInfoFromTypes(pkg *types.Package) BootstrapGoPackageInfo {
	typeString := func(t types.Type) string {
		return types.TypeString(t, func(p *types.Package) string {
			if p.Path() == pkg.Path() {
				return p.Name()
			}
			// Keep the import path in FFI signatures.  A package name such as
			// "mail" is not a stable identity: source annotations resolve through
			// the actual import path (go:net/mail), and two different packages
			// can share the same package name.  Escape dots because the bootstrap
			// signature parser uses the final dot to split qualifier from type.
			return "go:" + strings.ReplaceAll(p.Path(), ".", "%2E")
		})
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
	consts := []typeinference2.GoConstSignature{}
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
		interfaceMethods := []typeinference2.GoInterfaceMethodSignature{}
		fields := []typeinference2.GoFieldSignature{}
		embeds := []typeinference2.GoEmbedSignature{}
		underlying := ""
		if named, ok := types.Unalias(alias).(*types.Named); ok {
			methods = goTypeMethods(named, goTupleTypes)
			interfaceMethods = goInterfaceMethodSet(named)
			fields = goTypeFields(named, typeString)
			embeds = goTypeEmbeds(named, typeString)
			underlying = typeString(named.Underlying())
		} else if signature, ok := types.Unalias(alias).(*types.Signature); ok {
			// A direct alias to a function type (for example
			// `type SendFunc = func(...) (struct{}, error)`) has no *types.Named
			// target, but its signature is still required to type-check and adapt
			// callback arguments at the FFI boundary.
			underlying = typeString(signature)
		}
		typeSigs = append(typeSigs, typeinference2.GoTypeSignature{TypeName: name, TypeParams: []string{}, Methods: methods, InterfaceMethods: interfaceMethods, Fields: fields, Embeds: embeds, Underlying: underlying})
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
			typeSigs = append(typeSigs, typeinference2.GoTypeSignature{TypeName: name, TypeParams: paramNames, Methods: goTypeMethods(named, goTupleTypes), InterfaceMethods: goInterfaceMethodSet(named), Fields: goTypeFields(named, typeString), Embeds: goTypeEmbeds(named, typeString), Underlying: typeString(named.Underlying())})
		}
	}
	// Third pass: exported package-level constants (e.g. time.Second,
	// http.StatusOK).  They are surfaced as values of their declared type so
	// the self-hosted inference can seed `pkg.CONST` selectors into the env,
	// mirroring how the hand-written bootstrap compiler imports them.
	// Exported package-level `var`s ride the same channel: many libraries
	// publish their sentinel values as vars rather than consts (GORM's
	// ErrRecordNotFound, os.ErrNotExist), and a selector over one must
	// type-check the same way.  A `var` always carries a fully typed
	// types.Type, so types.Default is a no-op for it and the rendering below
	// is unchanged.
	for _, name := range scope.Names() {
		if !isExportedGoName(name) {
			continue
		}
		obj := scope.Lookup(name)
		switch value := obj.(type) {
		case *types.Const:
			// Untyped constants (e.g. `const StatusOK = 200`) report their type as
			// the untyped basic (`untyped int`).  types.Default resolves that to
			// the constant's default type (`int`) so the constant can be typed as a
			// value in MyGO.  Typed constants (e.g. time.Second) pass through
			// unchanged.
			consts = append(consts, typeinference2.GoConstSignature{Name: name, Type: typeString(types.Default(value.Type()))})
		case *types.Var:
			consts = append(consts, typeinference2.GoConstSignature{Name: name, Type: typeString(value.Type())})
		}
	}
	return BootstrapGoPackageInfo{GoName: pkg.Name(), Funcs: funcs, Types: typeSigs, Constants: consts}
}

// goInterfaceMethodSet records the complete method set for named interface
// types. It is separate from Methods because unexported methods affect Go
// assignability but must not become callable FFI selectors.
func goInterfaceMethodSet(named *types.Named) []typeinference2.GoInterfaceMethodSignature {
	if _, ok := named.Underlying().(*types.Interface); !ok {
		return nil
	}
	methodSet := types.NewMethodSet(named)
	out := make([]typeinference2.GoInterfaceMethodSignature, 0, methodSet.Len())
	for i := 0; i < methodSet.Len(); i++ {
		fn, ok := methodSet.At(i).Obj().(*types.Func)
		if !ok {
			continue
		}
		sig, ok := fn.Type().(*types.Signature)
		if !ok {
			continue
		}
		pkgPath := ""
		if !fn.Exported() && fn.Pkg() != nil {
			pkgPath = fn.Pkg().Path()
		}
		out = append(out, typeinference2.GoInterfaceMethodSignature{
			Name:        fn.Name(),
			PackagePath: pkgPath,
			Signature:   goMethodSignatureIdentity(sig),
		})
	}
	return out
}

func goMethodSignatureIdentity(sig *types.Signature) string {
	qualifier := func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Path()
	}
	params := sig.Params()
	paramTypes := make([]string, params.Len())
	for i := 0; i < params.Len(); i++ {
		t := types.Unalias(params.At(i).Type())
		if sig.Variadic() && i == params.Len()-1 {
			if slice, ok := t.(*types.Slice); ok {
				t = slice.Elem()
			}
			paramTypes[i] = "..." + types.TypeString(t, qualifier)
		} else {
			paramTypes[i] = types.TypeString(t, qualifier)
		}
	}
	results := sig.Results()
	resultTypes := make([]string, results.Len())
	for i := 0; i < results.Len(); i++ {
		resultTypes[i] = types.TypeString(types.Unalias(results.At(i).Type()), qualifier)
	}
	resultSuffix := ""
	switch len(resultTypes) {
	case 0:
	case 1:
		resultSuffix = " " + resultTypes[0]
	default:
		resultSuffix = " (" + strings.Join(resultTypes, ",") + ")"
	}
	return "(" + strings.Join(paramTypes, ",") + ")" + resultSuffix
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

// goTypeEmbeds collects the exported embedded (anonymous) fields of a named
// struct type.  The promotion resolver needs to walk the embedding chain, and
// the flat GoFieldSignature table cannot say which of its entries are embedded
// rather than merely named.  Each entry repeats the corresponding Fields entry
// (Go names an anonymous field after its type), so the qualified path
// `recv.Model` continues to resolve through Fields exactly as before.
func goTypeEmbeds(named *types.Named, typeString func(types.Type) string) []typeinference2.GoEmbedSignature {
	underlying, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil
	}
	embeds := []typeinference2.GoEmbedSignature{}
	for i := 0; i < underlying.NumFields(); i++ {
		f := underlying.Field(i)
		if !f.Anonymous() || !f.Exported() {
			continue
		}
		embeds = append(embeds, typeinference2.GoEmbedSignature{Name: f.Name(), Type: typeString(f.Type())})
	}
	return embeds
}

// goTypeMethods collects the exported pointer-method set of a named type as FFI
// signatures, mirroring the previous inline loop.
func goTypeMethods(named *types.Named, tupleTypes func(*types.Tuple) []string) []typeinference2.GoFuncSignature {
	methods := []typeinference2.GoFuncSignature{}
	// An interface's own method set is only reachable through the interface
	// type itself; the pointer-to-interface method set is empty.  Concrete
	// types keep the pointer method set so value + pointer receivers (and
	// promoted embedded selectors) all surface.
	methodSet := types.NewMethodSet(types.NewPointer(named))
	if _, isInterface := named.Underlying().(*types.Interface); isInterface {
		methodSet = types.NewMethodSet(named)
	}
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
