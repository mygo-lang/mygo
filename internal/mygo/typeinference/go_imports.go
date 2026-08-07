package typeinference

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	parserpkg "github.com/mygo-lang/mygo/internal/mygo/parser"
)

type GoPackageInfo struct {
	Alias   string
	Path    string
	Funcs   map[string]TFunc
	Aliases map[string]string
	Types   map[string]*GoTypeInfo
	// Constants records exported package-level Go constants (e.g.
	// http.StatusOK). They are surfaced as values of their declared type so
	// selectors like `http.StatusOK` type-check without going through a funcall.
	Constants map[string]MonoType
	// RawReturns records the original Go multi-return shape for functions
	// whose FFI signature was wrapped into a Result[T, error] (or similar).
	// At call sites that expect a tuple (e.g. `let (a, b) = f(...)` where
	// f is a Go FFI function returning `(T, error)`), the tuple destructuring
	// code can recover the unwrapped element types from this map instead of
	// forcing callers to go through the Result wrapper.
	RawReturns map[string][]MonoType
}

// GoTypeInfo records the exported method surface of a named Go type loaded
// through the FFI. It mirrors the go/types introspection used by the
// bootstrapped compiler so the hand-written inference can resolve methods
// like `db.AutoMigrate(...)` on `*gorm.DB`.
type GoTypeInfo struct {
	TypeName string
	Methods  map[string]TFunc
}

func loadGoPackageInfo(alias, path, dir string) (*GoPackageInfo, error) {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedTypes,
		Dir:  dir,
	}, path)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("loaded %d packages for %q, want 1", len(pkgs), path)
	}
	pkg := pkgs[0]
	if len(pkg.Errors) != 0 {
		return nil, fmt.Errorf("%s", pkg.Errors[0])
	}
	if pkg.Types == nil {
		return nil, fmt.Errorf("package %q has no type information", path)
	}
	info := &GoPackageInfo{
		Alias:      alias,
		Path:       path,
		Funcs:      map[string]TFunc{},
		Aliases:    map[string]string{},
		Types:      map[string]*GoTypeInfo{},
		Constants:  map[string]MonoType{},
		RawReturns: map[string][]MonoType{},
	}
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		obj, ok := scope.Lookup(name).(*types.TypeName)
		if !ok || !obj.IsAlias() || !isExportedGoName(name) {
			continue
		}
		info.Aliases[monoTypeFromGoType(types.Unalias(obj.Type())).String()] = pkg.Types.Name() + "." + name
	}
	for _, name := range scope.Names() {
		if !isExportedGoName(name) {
			continue
		}
		obj := scope.Lookup(name)
		tn, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		named, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		typeInfo := &GoTypeInfo{TypeName: name, Methods: map[string]TFunc{}}
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
			typeInfo.Methods[fn.Name()] = replaceGoAliases(goSignatureType(sig), info.Aliases).(TFunc)
		}
		info.Types[name] = typeInfo
	}
	for _, name := range scope.Names() {
		if !isExportedGoName(name) {
			continue
		}
		obj := scope.Lookup(name)
		fn, ok := obj.(*types.Func)
		if !ok {
			continue
		}
		sig, ok := fn.Type().(*types.Signature)
		if !ok {
			continue
		}
		// Record the raw Go return value types before any MyGO-level wrapping
		// happens.  The FFI surfaces `(T, error)` as `Result[T, error]` for
		// pattern matching, but `let (a, b) = ...` needs the original tuple.
		if sig.Results() != nil && sig.Results().Len() > 0 {
			raw := make([]MonoType, sig.Results().Len())
			for i := 0; i < sig.Results().Len(); i++ {
				raw[i] = replaceGoAliases(monoTypeFromGoType(sig.Results().At(i).Type()), info.Aliases)
			}
			info.RawReturns[name] = raw
		}
		info.Funcs[name] = replaceGoAliases(goSignatureType(sig), info.Aliases).(TFunc)
	}
	// Surface exported package-level constants (e.g. http.StatusOK) as values of
	// their declared type so `pkg.CONST` selectors type-check like ordinary
	// fields instead of failing with "no function".
	for _, name := range scope.Names() {
		if !isExportedGoName(name) {
			continue
		}
		obj := scope.Lookup(name)
		cnst, ok := obj.(*types.Const)
		if !ok {
			continue
		}
		info.Constants[name] = replaceGoAliases(monoTypeFromGoType(cnst.Type()), info.Aliases)
	}
	return info, nil
}

func replaceGoAliases(t MonoType, aliases map[string]string) MonoType {
	switch t := t.(type) {
	case TCon:
		if name, ok := aliases[t.Name]; ok && len(t.Args) == 0 {
			return TCon{Name: name}
		}
		args := make([]MonoType, len(t.Args))
		for i, arg := range t.Args {
			args[i] = replaceGoAliases(arg, aliases)
		}
		return TCon{Name: canonicalTypeName(t.Name), Args: args}
	case TFunc:
		args := make([]MonoType, len(t.Args))
		for i, arg := range t.Args {
			args[i] = replaceGoAliases(arg, aliases)
		}
		return TFunc{Args: args, Ret: replaceGoAliases(t.Ret, aliases), Variadic: t.Variadic}
	default:
		return t
	}
}

func isExportedGoName(name string) bool {
	if name == "" {
		return false
	}
	r, _ := utf8DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

func utf8DecodeRuneInString(s string) (rune, int) {
	for _, r := range s {
		return r, 1
	}
	return 0, 0
}

func loadMyGoPackageInfo(workspaceRoot, baseDir, importPath, alias string, cache map[string]*MyGoPackageInfo) (*MyGoPackageInfo, error) {
	cacheKey := workspaceRoot + "\x00" + baseDir + "\x00" + importPath + "\x00" + alias
	if cache != nil {
		if cached, ok := cache[cacheKey]; ok {
			return cached, nil
		}
	}
	dir, err := resolveMyGoImportPath(workspaceRoot, baseDir, importPath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	info := &MyGoPackageInfo{
		Alias:       alias,
		Path:        importPath,
		Funcs:       map[string]*Scheme{},
		Types:       map[string]struct{}{},
		TypeAliases: map[string]*TypeAliasDecl{},
		Structs:     map[string]*StructDecl{},
		Enums:       map[string]*EnumDecl{},
		Interfaces:  map[string]*InterfaceDecl{},
		Impls:       []*ImplDecl{},
		Values:      map[string]*Scheme{},
	}
	var decls []Decl
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".mygo") || strings.HasSuffix(name, ".gen.go") {
			continue
		}
		sourcePath := filepath.Join(dir, name)
		src, err := os.ReadFile(sourcePath)
		if err != nil {
			return nil, err
		}
		file, err := parserpkg.ParseFile(displayPath(sourcePath), string(src))
		if err != nil {
			return nil, err
		}
		if info.Name == "" && file.PackageName != "" {
			info.Name = file.PackageName
		}
		decls = append(decls, file.Decls...)
	}
	for _, decl := range decls {
		switch d := decl.(type) {
		case *FuncDecl:
			if !isExportedGoName(d.Name) {
				continue
			}
			info.Funcs[d.Name] = funcDeclSignatureScheme(d, TypeEnv{}, NewInferState())
		case *StructDecl:
			if isExportedGoName(d.Name) {
				info.Types[d.Name] = struct{}{}
				info.Structs[d.Name] = d
			}
		case *TypeAliasDecl:
			if isExportedGoName(d.Name) {
				info.Types[d.Name] = struct{}{}
				info.TypeAliases[d.Name] = d
			}
		case *TypeDecl:
			if isExportedGoName(d.Name) {
				info.Types[d.Name] = struct{}{}
			}
		case *EnumDecl:
			if isExportedGoName(d.Name) {
				info.Types[d.Name] = struct{}{}
				info.Enums[d.Name] = d
				for _, v := range d.Variants {
					if isExportedGoName(v.Name) {
						typeParamVars := make(map[string]MonoType, len(d.TypeParams))
						var typeArgs []MonoType
						st := NewInferState()
						for _, tp := range d.TypeParams {
							tv := TVar{ID: st.Fresh()}
							typeParamVars[tp] = tv
							typeArgs = append(typeArgs, tv)
						}
						args := make([]MonoType, 0, len(v.Fields))
						for _, f := range v.Fields {
							args = append(args, typeFromASTWithParams(f.Type, typeParamVars))
						}
						ret := MonoType(TCon{Name: d.Name, Args: typeArgs})
						if len(args) == 0 {
							info.Funcs[v.Name] = Generalize(TypeEnv{}, TFunc{Args: nil, Ret: ret}, nil)
						} else {
							info.Funcs[v.Name] = Generalize(TypeEnv{}, TFunc{Args: args, Ret: ret}, nil)
						}
					}
				}
			}
		case *InterfaceDecl:
			if isExportedGoName(d.Name) {
				info.Types[d.Name] = struct{}{}
				info.Interfaces[d.Name] = d
			}
		case *LetStmt:
			// A top-level `let`/`var` binding is exposed to importers as an
			// exported value (e.g. `let DB: Result[...] = initDB()`).  Only
			// bindings carrying a type annotation can be surfaced without
			// running full inference on the imported package.
			if d.Name == "" || !isExportedGoName(d.Name) || d.Mutable || d.Type == nil {
				continue
			}
			info.Values[d.Name] = Generalize(TypeEnv{}, typeFromAST(d.Type), nil)
		case *ImplDecl:
			info.Impls = append(info.Impls, d)
		}
	}
	// Exported function schemes are built while declarations are scanned, so
	// their types still contain this package's bare aliases. Normalize them
	// after every alias has been collected; callers can then qualify the
	// remaining concrete package types with their import alias.
	for name, sch := range info.Funcs {
		info.Funcs[name] = &Scheme{
			Bound: sch.Bound,
			Body: QualifiedType{
				Predicates: sch.Body.Predicates,
				Body:       expandMyGoPackageTypeAliases(info, sch.Body.Body, map[string]bool{}),
			},
		}
	}
	if cache != nil {
		cache[cacheKey] = info
	}
	return info, nil
}

// expandMyGoPackageTypeAliases expands aliases in a package's own exported
// signatures. It intentionally leaves non-alias type names bare; callers add
// their import qualifier later with qualifyMyGoType.
func expandMyGoPackageTypeAliases(info *MyGoPackageInfo, t MonoType, visiting map[string]bool) MonoType {
	switch t := t.(type) {
	case TCon:
		args := make([]MonoType, len(t.Args))
		for i, arg := range t.Args {
			args[i] = expandMyGoPackageTypeAliases(info, arg, visiting)
		}
		if decl := info.TypeAliases[t.Name]; decl != nil && len(decl.TypeParams) == len(args) && !visiting[t.Name] {
			visiting[t.Name] = true
			params := make(map[string]MonoType, len(decl.TypeParams))
			for i, name := range decl.TypeParams {
				params[name] = args[i]
			}
			expanded := typeFromASTWithParams(decl.Type, params)
			delete(visiting, t.Name)
			return expandMyGoPackageTypeAliases(info, expanded, visiting)
		}
		return TCon{Name: t.Name, Args: args}
	case TFunc:
		args := make([]MonoType, len(t.Args))
		for i, arg := range t.Args {
			args[i] = expandMyGoPackageTypeAliases(info, arg, visiting)
		}
		return TFunc{Args: args, Ret: expandMyGoPackageTypeAliases(info, t.Ret, visiting), Variadic: t.Variadic}
	default:
		return t
	}
}

func resolveMyGoImportPath(workspaceRoot, baseDir, importPath string) (string, error) {
	if filepath.IsAbs(importPath) {
		return importPath, nil
	}
	if strings.HasPrefix(importPath, ".") {
		return filepath.Clean(filepath.Join(baseDir, importPath)), nil
	}
	for _, start := range []string{baseDir, workspaceRoot} {
		if dir := resolveGoModuleImportDir(start, importPath); dir != "" {
			return dir, nil
		}
	}
	if workspaceRoot != "" {
		candidate := filepath.Clean(filepath.Join(workspaceRoot, importPath))
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	cur := baseDir
	for {
		candidate := filepath.Clean(filepath.Join(cur, importPath))
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return "", fmt.Errorf("cannot resolve MyGO import %q from %q", importPath, baseDir)
}

func displayPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	wd, err := os.Getwd()
	if err != nil {
		return abs
	}
	rel, err := filepath.Rel(wd, abs)
	if err != nil {
		return abs
	}
	return rel
}

func resolveGoModuleImportDir(start, importPath string) string {
	if start == "" {
		return ""
	}
	root := findGoModuleRoot(start)
	if root == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	if dir := moduleImportDir(root, modulePath(data), importPath); dir != "" {
		return dir
	}
	for _, repl := range goModReplaceEntries(data) {
		if dir := moduleImportDir(resolveReplaceRoot(root, repl.Path), repl.Module, importPath); dir != "" {
			return dir
		}
	}
	for _, req := range goModRequireEntries(data) {
		if suffix, ok := moduleImportSuffix(req.Module, importPath); ok {
			for _, cacheRoot := range goModCacheRoots() {
				modRoot := filepath.Join(cacheRoot, moduleCachePath(req.Module, req.Version))
				if dir := existingDir(filepath.Join(modRoot, filepath.FromSlash(suffix))); dir != "" {
					return dir
				}
			}
		}
	}
	return ""
}

type goModReplaceEntry struct {
	Module string
	Path   string
}

type goModRequireEntry struct {
	Module  string
	Version string
}

func moduleImportDir(root, module, importPath string) string {
	if root == "" || module == "" {
		return ""
	}
	suffix, ok := moduleImportSuffix(module, importPath)
	if !ok {
		return ""
	}
	return existingDir(filepath.Join(root, filepath.FromSlash(suffix)))
}

func moduleImportSuffix(module, importPath string) (string, bool) {
	if importPath == module {
		return "", true
	}
	prefix := module + "/"
	if strings.HasPrefix(importPath, prefix) {
		return strings.TrimPrefix(importPath, prefix), true
	}
	return "", false
}

func resolveReplaceRoot(moduleRoot, repl string) string {
	if repl == "" || strings.HasPrefix(repl, ".") || filepath.IsAbs(repl) {
		if !filepath.IsAbs(repl) {
			repl = filepath.Join(moduleRoot, repl)
		}
		return filepath.Clean(repl)
	}
	return ""
}

func existingDir(path string) string {
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		return filepath.Clean(path)
	}
	return ""
}

func goModReplaceEntries(goMod []byte) []goModReplaceEntry {
	var entries []goModReplaceEntry
	inReplaceBlock := false
	for _, line := range strings.Split(string(goMod), "\n") {
		line = cleanGoModLine(line)
		if line == "" {
			continue
		}
		if line == "replace (" {
			inReplaceBlock = true
			continue
		}
		if inReplaceBlock && line == ")" {
			inReplaceBlock = false
			continue
		}
		if strings.HasPrefix(line, "replace ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "replace "))
		} else if !inReplaceBlock {
			continue
		}
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "=>" && i > 0 && i+1 < len(fields) {
				entries = append(entries, goModReplaceEntry{Module: fields[0], Path: fields[i+1]})
				break
			}
		}
	}
	return entries
}

func goModRequireEntries(goMod []byte) []goModRequireEntry {
	var entries []goModRequireEntry
	inRequireBlock := false
	for _, line := range strings.Split(string(goMod), "\n") {
		line = cleanGoModLine(line)
		if line == "" {
			continue
		}
		if line == "require (" {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}
		if strings.HasPrefix(line, "require ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "require "))
		} else if !inRequireBlock {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			entries = append(entries, goModRequireEntry{Module: fields[0], Version: fields[1]})
		}
	}
	return entries
}

func cleanGoModLine(line string) string {
	line = strings.TrimSpace(line)
	if i := strings.Index(line, "//"); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	return line
}

func findGoModuleRoot(start string) string {
	absStart, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for cur := absStart; ; cur = filepath.Dir(cur) {
		if _, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
	}
}

func modulePath(goMod []byte) string {
	for _, line := range strings.Split(string(goMod), "\n") {
		line = cleanGoModLine(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func goModCacheRoots() []string {
	var roots []string
	if gomodcache := os.Getenv("GOMODCACHE"); gomodcache != "" {
		roots = append(roots, gomodcache)
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		for _, p := range filepath.SplitList(gopath) {
			if p != "" {
				roots = append(roots, filepath.Join(p, "pkg", "mod"))
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		roots = append(roots, filepath.Join(home, "go", "pkg", "mod"))
	}
	seen := map[string]bool{}
	out := roots[:0]
	for _, root := range roots {
		root = filepath.Clean(root)
		if !seen[root] {
			out = append(out, root)
			seen[root] = true
		}
	}
	return out
}

func moduleCachePath(module, version string) string {
	parts := strings.Split(module, "/")
	for i, p := range parts {
		parts[i] = escapeModulePathElem(p)
	}
	return filepath.Join(strings.Join(parts, string(filepath.Separator)) + "@" + version)
}

func escapeModulePathElem(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('!')
			b.WriteRune(r + ('a' - 'A'))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func goSignatureType(sig *types.Signature) TFunc {
	params := sig.Params()
	args := make([]MonoType, 0, params.Len())
	for i := 0; i < params.Len(); i++ {
		t := params.At(i).Type()
		if sig.Variadic() && i == params.Len()-1 {
			if slice, ok := t.(*types.Slice); ok {
				t = slice.Elem()
			}
		}
		args = append(args, monoTypeFromGoType(t))
	}

	var ret MonoType = TUnit{}
	results := sig.Results()
	if results != nil {
		switch results.Len() {
		case 0:
			ret = TUnit{}
		case 1:
			ret = monoTypeFromGoType(results.At(0).Type())
		default:
			retArgs := make([]MonoType, results.Len())
			for i := 0; i < results.Len(); i++ {
				retArgs[i] = monoTypeFromGoType(results.At(i).Type())
			}
			if len(retArgs) == 2 && isErrorType(retArgs[1]) {
				ret = TCon{Name: "Result", Args: retArgs}
			} else {
				ret = TCon{Name: "Tuple", Args: retArgs}
			}
		}
	}

	return TFunc{Args: args, Ret: ret, Variadic: sig.Variadic()}
}

func isErrorType(t MonoType) bool {
	con, ok := t.(TCon)
	if !ok {
		return false
	}
	return sameTypeName(con.Name, "Error") || con.Name == "builtin.error" || con.Name == "errors.error"
}

func monoTypeFromGoType(t types.Type) MonoType {
	switch t := t.(type) {
	case *types.Basic:
		switch t.Kind() {
		case types.Bool, types.UntypedBool:
			return TCon{Name: "Bool"}
		case types.Int, types.UntypedInt:
			return TCon{Name: "Int"}
		case types.Int8:
			return TCon{Name: "Int8"}
		case types.Int16:
			return TCon{Name: "Int16"}
		case types.Int32, types.UntypedRune:
			return TCon{Name: "Int32"}
		case types.Int64:
			return TCon{Name: "Int64"}
		case types.Uint:
			return TCon{Name: "UInt"}
		case types.Uint8:
			return TCon{Name: "UInt8"}
		case types.Uint16:
			return TCon{Name: "UInt16"}
		case types.Uint32:
			return TCon{Name: "UInt32"}
		case types.Uint64:
			return TCon{Name: "UInt64"}
		case types.Float32:
			return TCon{Name: "Float32"}
		case types.Float64, types.UntypedFloat:
			return TCon{Name: "Float64"}
		case types.String, types.UntypedString:
			return TCon{Name: "String"}
		case types.UntypedNil:
			return TCon{Name: "Nil"}
		}
		return TCon{Name: goTypeName(t)}
	case *types.Interface:
		if t.Empty() {
			return TCon{Name: "Any"}
		}
		return TCon{Name: "interface"}
	case *types.Pointer:
		return TCon{Name: "Ref", Args: []MonoType{monoTypeFromGoType(t.Elem())}}
	case *types.Slice:
		return TCon{Name: "Slice", Args: []MonoType{monoTypeFromGoType(t.Elem())}}
	case *types.Map:
		return TCon{Name: "Map", Args: []MonoType{
			monoTypeFromGoType(t.Key()),
			monoTypeFromGoType(t.Elem()),
		}}
	case *types.Signature:
		tf := goSignatureType(t)
		return tf
	case *types.Named:
		name := t.Obj().Name()
		if pkg := t.Obj().Pkg(); pkg != nil && pkg.Name() != "" {
			name = pkg.Name() + "." + name
		}
		return TCon{Name: canonicalTypeName(name)}
	case *types.Alias:
		// Keep the exported alias name.  MyGO type annotations name the FFI
		// package's public surface (for example goast.Expr), so eagerly
		// unaliasing it to ast.Expr makes an otherwise identical annotation
		// fail unification.
		name := t.Obj().Name()
		if pkg := t.Obj().Pkg(); pkg != nil && pkg.Name() != "" {
			name = pkg.Name() + "." + name
		}
		return TCon{Name: canonicalTypeName(name)}
	case *types.TypeParam:
		// A generic Go function (e.g. maps.Clone[M ~map[K,V]]) is polymorphic
		// in its type parameters.  Rendering one as a concrete TCon("M") would
		// force the parameter to equal the argument type and break calls such
		// as `maps.Clone(src: Map[String, String])`.  Represent it as a type
		// variable keyed by its index so the same parameter in the parameters
		// and the result stays connected at every call site.
		return TVar{ID: t.Index()}
	}
	return TCon{Name: canonicalTypeName(goTypeName(t))}
}

func goTypeName(t types.Type) string {
	name := strings.TrimSpace(types.TypeString(t, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Name()
	}))
	if name == "" {
		return "Any"
	}
	return name
}

func importAlias(path, alias string) string {
	if alias != "" {
		return alias
	}
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	if path == "" {
		return "go"
	}
	return path
}
