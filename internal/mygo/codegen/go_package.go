package codegen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	goparser "go/parser"
	"go/token"
	gotypes "go/types"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

// loadGoPackageSigs loads function/method signatures from a Go package.
func loadGoPackageSigs(path string) (*GoPackageSigs, error) {
	cmd := exec.Command("go", "list", "-json", path)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go list %q: %w: %s", path, err, strings.TrimSpace(stderr.String()))
	}
	var meta struct {
		Dir     string
		Name    string
		GoFiles []string
	}
	if err := json.Unmarshal(stdout.Bytes(), &meta); err != nil {
		return nil, err
	}
	if meta.Dir == "" {
		return nil, fmt.Errorf("go list %q: missing package dir", path)
	}

	fset := token.NewFileSet()
	var parsed []*ast.File
	for _, name := range meta.GoFiles {
		fpath := filepath.Join(meta.Dir, name)
		f, err := goparser.ParseFile(fset, fpath, nil, goparser.SkipObjectResolution)
		if err != nil {
			continue
		}
		parsed = append(parsed, f)
	}
	if len(parsed) == 0 {
		return nil, fmt.Errorf("go package %q: no parsable Go files", path)
	}

	conf := gotypes.Config{Importer: importer.Default()}
	checked, err := conf.Check(path, fset, parsed, nil)
	if err != nil {
		return nil, err
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
