package typeinference

import (
	"fmt"
	"go/importer"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	parserpkg "github.com/mygo-lang/mygo/internal/mygo/parser"
)

type GoPackageInfo struct {
	Alias string
	Path  string
	Funcs map[string]TFunc
}

func loadGoPackageInfo(alias, path string) (*GoPackageInfo, error) {
	pkg, err := importer.Default().Import(path)
	if err != nil {
		return nil, err
	}
	info := &GoPackageInfo{
		Alias: alias,
		Path:  path,
		Funcs: map[string]TFunc{},
	}
	scope := pkg.Scope()
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
		info.Funcs[name] = goSignatureType(sig)
	}
	return info, nil
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
		Alias:      alias,
		Path:       importPath,
		Funcs:      map[string]*Scheme{},
		Types:      map[string]struct{}{},
		Structs:    map[string]*StructDecl{},
		Enums:      map[string]*EnumDecl{},
		Interfaces: map[string]*InterfaceDecl{},
		Impls:      []*ImplDecl{},
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
		case *ImplDecl:
			info.Impls = append(info.Impls, d)
		}
	}
	if cache != nil {
		cache[cacheKey] = info
	}
	return info, nil
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
	return con.Name == "error" || con.Name == "builtin.error" || con.Name == "errors.error"
}

func monoTypeFromGoType(t types.Type) MonoType {
	switch t := t.(type) {
	case *types.Basic:
		switch t.Kind() {
		case types.Bool:
			return TCon{Name: "Bool"}
		case types.Int:
			return TCon{Name: "Int"}
		case types.Int8:
			return TCon{Name: "Int8"}
		case types.Int16:
			return TCon{Name: "Int16"}
		case types.Int32:
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
		case types.Float64:
			return TCon{Name: "Float64"}
		case types.String:
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
		return TCon{Name: name}
	case *types.Alias:
		return monoTypeFromGoType(types.Unalias(t))
	}
	return TCon{Name: goTypeName(t)}
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
