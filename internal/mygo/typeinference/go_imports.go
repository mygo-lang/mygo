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
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		file, err := parserpkg.ParseFile("<go-import>", string(src))
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
			ret = TCon{Name: "Tuple", Args: retArgs}
		}
	}

	return TFunc{Args: args, Ret: ret, Variadic: sig.Variadic()}
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
