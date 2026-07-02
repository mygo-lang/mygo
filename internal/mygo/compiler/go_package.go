package compiler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	goparser "go/parser"
	gotoken "go/token"
	"go/types"
	"os/exec"
	"path/filepath"
	"strings"
)

func (g *generator) goPackageSigsFor(path string) (*goPackageSigs, error) {
	if sigs, ok := g.goSigCache[path]; ok {
		return sigs, nil
	}
	sigs, err := loadGoPackageSigs(path)
	if err != nil {
		return nil, err
	}
	g.goSigCache[path] = sigs
	return sigs, nil
}

func loadGoPackageSigs(path string) (*goPackageSigs, error) {
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
	pkg, funcs, err := loadGoPackageTypeSigs(meta.Dir, meta.GoFiles)
	if err != nil {
		return nil, err
	}
	methods, err := loadGoPackageTypeMethods(meta.Dir, meta.GoFiles)
	if err != nil {
		return nil, err
	}
	return &goPackageSigs{funcs: funcs, methods: methods, pkg: pkg}, nil
}

func loadGoPackageTypeSigs(dir string, files []string) (*types.Package, map[string]*goFuncSig, error) {
	fset := gotoken.NewFileSet()
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("go package %q: no Go files", dir)
	}
	parsed := make([]*ast.File, 0, len(files))
	for _, name := range files {
		path := filepath.Join(dir, name)
		f, err := goparser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil, nil, err
		}
		parsed = append(parsed, f)
	}
	conf := types.Config{Importer: importer.Default()}
	checked, err := conf.Check(dir, fset, parsed, nil)
	if err != nil {
		return nil, nil, err
	}
	funcs := map[string]*goFuncSig{}
	scope := checked.Scope()
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
		funcs[name] = &goFuncSig{
			params: goSignatureParams(sig),
			ret:    goSignatureResults(sig),
		}
	}
	return checked, funcs, nil
}

func loadGoPackageTypeMethods(dir string, files []string) (map[string]map[string]*goFuncSig, error) {
	fset := gotoken.NewFileSet()
	if len(files) == 0 {
		return nil, fmt.Errorf("go package %q: no Go files", dir)
	}
	parsed := make([]*ast.File, 0, len(files))
	for _, name := range files {
		path := filepath.Join(dir, name)
		f, err := goparser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, f)
	}
	conf := types.Config{Importer: importer.Default()}
	checked, err := conf.Check(dir, fset, parsed, nil)
	if err != nil {
		return nil, err
	}
	methods := map[string]map[string]*goFuncSig{}
	scope := checked.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		fn, ok := obj.(*types.Func)
		if !ok {
			continue
		}
		sig, ok := fn.Type().(*types.Signature)
		if !ok || sig.Recv() == nil {
			continue
		}
		recv := sig.Recv().Type().String()
		if methods[recv] == nil {
			methods[recv] = map[string]*goFuncSig{}
		}
		methods[recv][name] = &goFuncSig{
			params: goSignatureParams(sig),
			ret:    goSignatureResults(sig),
		}
	}
	return methods, nil
}

func goSignatureParams(sig *types.Signature) []string {
	if sig == nil {
		return nil
	}
	params := sig.Params()
	var out []string
	for i := 0; i < params.Len(); i++ {
		typ := params.At(i).Type().String()
		if sig.Variadic() && i == params.Len()-1 {
			typ = "..." + strings.TrimPrefix(typ, "[]")
		}
		out = append(out, typ)
	}
	return out
}

func goSignatureResults(sig *types.Signature) []string {
	if sig == nil || sig.Results() == nil {
		return nil
	}
	results := sig.Results()
	out := make([]string, 0, results.Len())
	for i := 0; i < results.Len(); i++ {
		out = append(out, results.At(i).Type().String())
	}
	return out
}
