package codegen

import (
	"fmt"
	gotypes "go/types"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

// loadGoPackageSigs loads function/method signatures from a Go package.
//
// It uses golang.org/x/tools/go/packages so that dependencies are resolved
// through the Go module graph (including sub-packages living in the module
// cache).  Using go/importer.Default plus a hand-built []*ast.File only works
// for the standard library; third-party modules cannot be type-checked that
// way, which previously surfaced as un-wrapped (T, error) FFI calls whenever
// the signature table failed to load.
func loadGoPackageSigs(path string) (*GoPackageSigs, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports |
			packages.NeedDeps,
	}
	pkgs, err := packages.Load(cfg, path)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("go package %q: no such package", path)
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return nil, fmt.Errorf("load %q: %w", path, pkg.Errors[0])
	}
	checked := pkg.Types
	if checked == nil {
		return nil, fmt.Errorf("go package %q: no type information", path)
	}

	funcs := map[string]*GoFuncSig{}
	methods := map[string]map[string]*GoFuncSig{}

	scope := checked.Scope()
	for _, name := range scope.Names() {
		if !isExportedGoIdent(name) {
			continue
		}
		obj := scope.Lookup(name)
		fn, ok := obj.(*gotypes.Func)
		if !ok {
			continue
		}
		sig, ok := fn.Type().(*gotypes.Signature)
		if !ok {
			continue
		}
		if sig.Recv() == nil {
			funcs[name] = &GoFuncSig{
				params: goSignatureParams(sig),
				ret:    goSignatureResults(sig),
			}
		} else {
			recv := sig.Recv().Type().String()
			if methods[recv] == nil {
				methods[recv] = map[string]*GoFuncSig{}
			}
			methods[recv][name] = &GoFuncSig{
				params: goSignatureParams(sig),
				ret:    goSignatureResults(sig),
			}
		}
	}

	return &GoPackageSigs{funcs: funcs, methods: methods, pkg: checked}, nil
}

func goSignatureParams(sig *gotypes.Signature) []string {
	if sig == nil {
		return nil
	}
	params := sig.Params()
	var out []string
	for i := 0; i < params.Len(); i++ {
		typ := goTypeToMyGo(params.At(i).Type())
		if sig.Variadic() && i == params.Len()-1 {
			typ = "..." + variadicElem(typ)
		}
		out = append(out, typ)
	}
	return out
}

func variadicElem(typ string) string {
	typ = strings.TrimSpace(typ)
	if strings.HasPrefix(typ, "[]") {
		return typ[2:]
	}
	if strings.HasPrefix(typ, "Slice[") && strings.HasSuffix(typ, "]") {
		return typ[6 : len(typ)-1]
	}
	return strings.TrimPrefix(typ, "...")
}

func goSignatureResults(sig *gotypes.Signature) []string {
	if sig == nil || sig.Results() == nil {
		return nil
	}
	results := sig.Results()
	out := make([]string, results.Len())
	for i := 0; i < results.Len(); i++ {
		out[i] = goTypeToMyGo(results.At(i).Type())
	}
	return out
}

// goTypeToMyGo converts a go/types type to MyGO type notation.
func goTypeToMyGo(t gotypes.Type) string {
	if t == nil {
		return "any"
	}
	s := t.String()
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "[]") {
		return "Slice[" + goTypeToMyGoName(s[2:]) + "]"
	}
	if strings.HasPrefix(s, "*") {
		return "Ref[" + goTypeToMyGoName(s[1:]) + "]"
	}
	if strings.HasPrefix(s, "map[") {
		end := strings.Index(s, "]")
		if end > 0 {
			key := goTypeToMyGoName(s[4:end])
			val := goTypeToMyGoName(s[end+1:])
			if val == "struct{}" {
				return "Set[" + key + "]"
			}
			return "Map[" + key + ", " + val + "]"
		}
	}
	if strings.HasPrefix(s, "chan<- ") {
		return "SendChan[" + goTypeToMyGoName(s[7:]) + "]"
	}
	if strings.HasPrefix(s, "<-chan ") {
		return "RecvChan[" + goTypeToMyGoName(s[7:]) + "]"
	}
	if strings.HasPrefix(s, "chan ") {
		return "Chan[" + goTypeToMyGoName(s[5:]) + "]"
	}
	return goTypeToMyGoName(s)
}

func goTypeToMyGoName(s string) string {
	s = strings.TrimSpace(s)
	switch s {
	case "string":
		return "String"
	case "bool":
		return "Bool"
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
	case "any", "interface{}":
		return "Any"
	case "error":
		return "error"
	case "rune":
		return "rune"
	case "byte":
		return "Byte"
	}
	return s
}

func isExportedGoIdent(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}

func isExportedIdent(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		return unicode.IsUpper(r)
	}
	return false
}
