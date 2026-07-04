package typeinference

import (
	"go/importer"
	"go/types"
	"strings"
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
