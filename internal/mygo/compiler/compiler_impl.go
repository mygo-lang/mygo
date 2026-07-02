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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
	parserpkg "github.com/mygo-lang/mygo/internal/mygo/parser"
)

func loadPreludeDecls() ([]Decl, error) {
	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("prelude.mysrc: unable to locate compiler source")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(filePath), "..", "prelude.mysrc"))
	if err != nil {
		return nil, err
	}
	file, err := parserpkg.ParseFile(string(src))
	if err != nil {
		return nil, fmt.Errorf("prelude.mysrc: %w", err)
	}
	return file.Decls, nil
}

func (g *generator) genGlobals() (string, error) {
	var b strings.Builder
	ctx := &exprCtx{
		locals:          map[string]string{},
		bindings:        map[string]string{},
		sourceTypes:     map[string]string{},
		mutable:         map[string]bool{},
		typeParams:      map[string]struct{}{},
		constraintFuncs: map[string]string{},
	}
	for _, decl := range g.pkg.Decls {
		s, ok := decl.(*LetStmt)
		if !ok {
			continue
		}
		code, typ, err := g.translateExpr(s.Value, ctx, g.goType(s.Type, nil))
		if err != nil {
			return "", common.ErrorAtPos(s.Line, s.Column, "global binding %q: %v", s.Name, err)
		}
		actual := sanitizeIdent(s.Name)
		if actual == "" || actual == "_" {
			actual = "tmp"
		}
		if s.Name == "_" {
			b.WriteString("var _ = ")
			b.WriteString(code)
			b.WriteString("\n")
			continue
		}
		b.WriteString("var ")
		b.WriteString(actual)
		if s.Type != nil {
			b.WriteString(" ")
			b.WriteString(g.goType(s.Type, nil))
		}
		b.WriteString(" = ")
		b.WriteString(code)
		b.WriteString("\n")
		ctx.bindings[s.Name] = actual
		if typ == "" && s.Type != nil {
			typ = g.goType(s.Type, nil)
		}
		ctx.locals[s.Name] = typ
		ctx.sourceTypes[s.Name] = typ
		ctx.mutable[actual] = s.Mutable
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	b.WriteString(g.genTypeclassDispatchers())
	return b.String(), nil
}

func (g *generator) genTypeclassDispatchers() string {
	var b strings.Builder
	for _, ifaceName := range g.sortedTypeclassNames() {
		iface := g.pkg.Interfaces[ifaceName]
		for _, m := range iface.Methods {
			retType := typeStringReturn(m.Ret, nil)
			b.WriteString("var ")
			b.WriteString(dispatchRegistryName(ifaceName, m.Name))
			b.WriteString(" = map[string]func(")
			for i := range m.Params {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString("any")
			}
			b.WriteString(")")
			if retType != "" {
				b.WriteString(" ")
				b.WriteString(retType)
			}
			b.WriteString("{}\n")
			b.WriteString("func ")
			b.WriteString(dispatchFuncName(ifaceName, m.Name))
			b.WriteString("(")
			for i, p := range m.Params {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(p.Name)
				b.WriteString(" any")
			}
			b.WriteString(")")
			if retType != "" {
				b.WriteString(" ")
				b.WriteString(retType)
			}
			b.WriteString(" {\n")
			b.WriteString("\tkey := ")
			b.WriteString(dispatchKeyExpr(m.Params, nil))
			b.WriteString("\n")
			b.WriteString("\tif fn, ok := ")
			b.WriteString(dispatchRegistryName(ifaceName, m.Name))
			b.WriteString("[key]; ok {\n")
			if retType != "" {
				b.WriteString("\t\treturn fn(")
				for i, p := range m.Params {
					if i > 0 {
						b.WriteString(", ")
					}
					b.WriteString(p.Name)
				}
				b.WriteString(")\n\t}\n")
			} else {
				b.WriteString("\t\tfn(")
				for i, p := range m.Params {
					if i > 0 {
						b.WriteString(", ")
					}
					b.WriteString(p.Name)
				}
				b.WriteString(")\n\t\treturn\n\t}\n")
			}
			b.WriteString("\tpanic(\"missing typeclass implementation\")\n")
			if retType == "" {
				b.WriteString("\treturn\n")
			}
			b.WriteString("}\n\n")
		}
	}
	return b.String()
}

func (p *Package) sortedImports() []importSpec {
	imports := make([]importSpec, 0, len(p.ImportDecls))
	seen := map[string]struct{}{}
	seenPaths := map[string]struct{}{}
	for _, imp := range p.ImportDecls {
		alias := imp.Alias
		if alias == "" {
			alias = importAliasForPath(imp.Path)
		}
		path := importPathForGo(imp.Path)
		key := alias + "\x00" + path
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		seenPaths[path] = struct{}{}
		imports = append(imports, importSpec{Alias: alias, Path: imp.Path})
	}
	for path := range p.Imports {
		rawPath := importPathForGo(path)
		if _, ok := seenPaths[rawPath]; ok {
			continue
		}
		imports = append(imports, importSpec{Path: path})
	}
	sort.Slice(imports, func(i, j int) bool {
		if imports[i].Alias == imports[j].Alias {
			return imports[i].Path < imports[j].Path
		}
		return imports[i].Alias < imports[j].Alias
	})
	return imports
}

type generator struct {
	pkg               *Package
	importAliases     map[string]string
	interfaceByMethod map[string]string
	variantByName     map[string]string
	goSigCache        map[string]*goPackageSigs
	needsCallAny      bool
	localSeq          int
}

func hasImportPath(imports []importSpec, path string) bool {
	for _, imp := range imports {
		if importPathForGo(imp.Path) == path {
			return true
		}
	}
	return false
}

func (g *generator) genEnum(d *EnumDecl) string {
	var b strings.Builder
	typeParams := genTypeParams(d.TypeParams)
	typeArgs := typeArgList(d.TypeParams)
	b.WriteString("type ")
	b.WriteString(d.Name)
	b.WriteString(typeParams)
	b.WriteString(" interface{ is")
	b.WriteString(d.Name)
	b.WriteString("() }\n")
	for _, v := range d.Variants {
		tname := variantGoTypeName(d.Name, v.Name)
		b.WriteString("type ")
		b.WriteString(tname)
		b.WriteString(typeParams)
		b.WriteString(" struct {\n")
		for i := range v.Fields {
			b.WriteString("\tF")
			b.WriteString(strconv.Itoa(i))
			b.WriteString(" ")
			b.WriteString(g.goType(v.Fields[i].Type, typeParamSet(d.TypeParams)))
			b.WriteString("\n")
		}
		b.WriteString("}\n")
		b.WriteString("func (")
		b.WriteString(tname)
		b.WriteString(typeArgList(d.TypeParams))
		b.WriteString(") is")
		b.WriteString(d.Name)
		b.WriteString("() {}\n")
	}
	for _, v := range d.Variants {
		tname := variantGoTypeName(d.Name, v.Name)
		b.WriteString("func ")
		b.WriteString(v.Name)
		b.WriteString(typeParams)
		b.WriteString("(")
		if len(v.Fields) > 0 {
			args := make([]string, 0, len(v.Fields))
			for i, f := range v.Fields {
				args = append(args, fmt.Sprintf("a%d %s", i, g.goType(f.Type, typeParamSet(d.TypeParams))))
			}
			b.WriteString(strings.Join(args, ", "))
		}
		b.WriteString(") ")
		b.WriteString(d.Name)
		b.WriteString(typeArgs)
		b.WriteString(" {\n")
		b.WriteString("\treturn ")
		b.WriteString(tname)
		b.WriteString(typeArgs)
		b.WriteString("{")
		for i := range v.Fields {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("F%d: a%d", i, i))
		}
		b.WriteString("}\n}\n")
	}
	b.WriteString("\n")
	return b.String()
}

func (g *generator) genStruct(d *StructDecl) string {
	var b strings.Builder
	b.WriteString("type ")
	b.WriteString(d.Name)
	b.WriteString(genTypeParams(d.TypeParams))
	b.WriteString(" struct {\n")
	for _, f := range d.Fields {
		b.WriteString("\t")
		if f.Name == "embed" {
			b.WriteString(g.goType(f.Type, typeParamSet(d.TypeParams)))
			b.WriteString("\n")
			continue
		}
		b.WriteString(exportName(f.Name))
		b.WriteString(" ")
		b.WriteString(g.goType(f.Type, typeParamSet(d.TypeParams)))
		b.WriteString("\n")
	}
	b.WriteString("}\n\n")
	return b.String()
}

func (g *generator) genInterface(d *InterfaceDecl) string {
	var b strings.Builder
	b.WriteString("type ")
	b.WriteString(d.Name)
	b.WriteString(genTypeParams(d.TypeParams))
	b.WriteString(" interface {\n")
	for _, m := range d.Methods {
		b.WriteString("\t")
		b.WriteString(m.Name)
		b.WriteString("(")
		for i, p := range m.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(p.Name)
			b.WriteString(" ")
			b.WriteString(g.goType(p.Type, typeParamSet(d.TypeParams)))
		}
		b.WriteString(") ")
		b.WriteString(g.goReturnType(m.Ret, typeParamSet(d.TypeParams)))
		b.WriteString("\n")
	}
	b.WriteString("}\n\n")
	return b.String()
}

func (g *generator) genImpl(d *ImplDecl) (string, error) {
	iface := g.pkg.Interfaces[d.Name]
	if iface == nil {
		return "", common.ErrorAtPos(d.Line, d.Column, "impl %s: missing interface declaration", d.Name)
	}
	if len(iface.TypeParams) != len(d.TypeArgs) {
		return "", common.ErrorAtPos(d.Line, d.Column, "impl %s: type arity mismatch", d.Name)
	}
	subst := map[string]string{}
	for i, tp := range iface.TypeParams {
		subst[tp] = g.goType(d.TypeArgs[i], nil)
	}
	typeKey := g.implTypeKey(d.TypeArgs)
	var b strings.Builder
	methodBodies := map[string]*FuncDecl{}
	for _, m := range d.Methods {
		methodBodies[m.Name] = m
	}
	for _, sig := range iface.Methods {
		method := methodBodies[sig.Name]
		bodyExpr := sig.Body
		params := sig.Params
		ret := sig.Ret
		if method != nil {
			bodyExpr = method.Body
			params = method.Params
			ret = method.Ret
		}
		ctx := &exprCtx{
			locals:          map[string]string{},
			bindings:        map[string]string{},
			sourceTypes:     map[string]string{},
			mutable:         map[string]bool{},
			typeParams:      map[string]struct{}{},
			constraintFuncs: map[string]string{},
			retType:         typeStringReturn(ret, subst),
			currentImpl:     d.Name,
		}
		b.WriteString("func ")
		b.WriteString(helperFuncName(sig.Name, typeKey))
		b.WriteString("(")
		for i, p := range params {
			if i > 0 {
				b.WriteString(", ")
			}
			goType := typeString(p.Type, subst)
			b.WriteString(p.Name)
			b.WriteString(" ")
			b.WriteString(goType)
			ctx.locals[p.Name] = goType
			ctx.sourceTypes[p.Name] = typeString(p.Type, subst)
			ctx.bindings[p.Name] = p.Name
			ctx.mutable[p.Name] = false
		}
		retType := typeStringReturn(ret, subst)
		b.WriteString(") ")
		b.WriteString(retType)
		b.WriteString(" {\n")
		if bodyExpr == nil {
			if retType == "" {
				b.WriteString("\treturn\n")
			} else {
				b.WriteString("\tpanic(\"unimplemented\")\n")
			}
		} else {
			expr, exprType, err := g.translateExpr(bodyExpr, ctx, retType)
			if err != nil {
				return "", err
			}
			if retType == "" {
				g.writeUnitBody(&b, expr, exprType)
			} else {
				b.WriteString("\treturn ")
				b.WriteString(expr)
				b.WriteString("\n")
			}
		}
		b.WriteString("}\n")
		b.WriteString("func init() {\n")
		b.WriteString("\t")
		b.WriteString(dispatchRegistryName(d.Name, sig.Name))
		b.WriteString("[")
		b.WriteString(strconv.Quote(g.implDispatchKey(sig.Params, subst)))
		b.WriteString("] = func(")
		for i, p := range sig.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(p.Name)
			b.WriteString(" any")
		}
		b.WriteString(")")
		if retType != "" {
			b.WriteString(" ")
			b.WriteString(retType)
		}
		b.WriteString(" {\n")
		for i, p := range sig.Params {
			b.WriteString("\t\t")
			b.WriteString(p.Name)
			b.WriteString("Typed := ")
			b.WriteString(p.Name)
			b.WriteString(".(")
			b.WriteString(typeString(p.Type, subst))
			b.WriteString(")\n")
			_ = i
		}
		if retType != "" {
			b.WriteString("\t\treturn ")
		} else {
			b.WriteString("\t\t")
		}
		b.WriteString(helperFuncName(sig.Name, typeKey))
		b.WriteString("(")
		for i, p := range sig.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(p.Name)
			b.WriteString("Typed")
		}
		b.WriteString(")\n")
		if retType == "" {
			b.WriteString("\t\treturn\n")
		}
		b.WriteString("\t}\n")
		b.WriteString("}\n")
	}
	b.WriteString("\n")
	return b.String(), nil
}

func (g *generator) genFunc(d *FuncDecl) (string, error) {
	var b strings.Builder
	b.WriteString("func ")
	b.WriteString(d.Name)
	b.WriteString(genTypeParams(d.TypeParams))
	b.WriteString("(")
	ctx := &exprCtx{
		locals:           map[string]string{},
		bindings:         map[string]string{},
		sourceTypes:      map[string]string{},
		mutable:          map[string]bool{},
		typeParams:       typeParamSet(d.TypeParams),
		constraintFuncs:  map[string]string{},
		typeclassMethods: map[string][]typeclassBinding{},
		retType:          g.goReturnType(d.Ret, typeParamSet(d.TypeParams)),
	}
	for i, p := range d.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		goType := g.goType(p.Type, typeParamSet(d.TypeParams))
		b.WriteString(p.Name)
		b.WriteString(" ")
		b.WriteString(goType)
		ctx.locals[p.Name] = goType
		ctx.sourceTypes[p.Name] = goType
		ctx.bindings[p.Name] = p.Name
		ctx.mutable[p.Name] = false
	}
	type constraintParam struct {
		name string
		typ  string
	}
	constraintParams := map[string]constraintParam{}
	var constraintOrder []string
	for _, c := range d.Where {
		iface := g.pkg.Interfaces[c.Name]
		if iface == nil {
			return "", common.ErrorAtPos(c.Line, c.Column, "function %s: missing interface %s", d.Name, c.Name)
		}
		if len(iface.TypeParams) != len(c.Args) {
			return "", common.ErrorAtPos(c.Line, c.Column, "function %s: type arity mismatch for %s", d.Name, c.Name)
		}
		subst := map[string]string{}
		for i, tp := range iface.TypeParams {
			subst[tp] = g.goType(c.Args[i], typeParamSet(d.TypeParams))
		}
		for _, m := range iface.Methods {
			paramName := m.Name + "Fn"
			binding := typeclassBinding{
				Interface: c.Name,
				Score:     typeclassMatchScore(c.Args, typeParamSet(d.TypeParams)),
				ParamTypes: func() []string {
					out := make([]string, 0, len(m.Params))
					for _, p := range m.Params {
						out = append(out, typeString(p.Type, subst))
					}
					return out
				}(),
				RetType: typeStringReturn(m.Ret, subst),
			}
			ctx.typeclassMethods[m.Name] = append(ctx.typeclassMethods[m.Name], binding)
			if _, ok := constraintParams[m.Name]; !ok {
				ctx.constraintFuncs[m.Name] = paramName
				constraintParams[m.Name] = constraintParam{
					name: paramName,
					typ:  typeclassFuncType(binding.ParamTypes, binding.RetType),
				}
				constraintOrder = append(constraintOrder, m.Name)
			} else {
				if betterMatch(binding.Score, typeclassBindingBest(ctx.typeclassMethods[m.Name]).Score) {
					ctx.constraintFuncs[m.Name] = paramName
					constraintParams[m.Name] = constraintParam{name: paramName, typ: typeclassFuncType(binding.ParamTypes, binding.RetType)}
				}
			}
		}
	}
	for _, methodName := range constraintOrder {
		cp := constraintParams[methodName]
		b.WriteString(", ")
		b.WriteString(cp.name)
		b.WriteString(" ")
		b.WriteString(cp.typ)
	}
	retType := g.goReturnType(d.Ret, typeParamSet(d.TypeParams))
	b.WriteString(")")
	if retType != "" {
		b.WriteString(" ")
		b.WriteString(retType)
	}
	b.WriteString(" {\n")
	expr, exprType, err := g.translateExpr(d.Body, ctx, retType)
	if err != nil {
		return "", err
	}
	if retType == "" {
		g.writeUnitBody(&b, expr, exprType)
	} else {
		b.WriteString("\treturn ")
		b.WriteString(expr)
		b.WriteString("\n")
	}
	b.WriteString("}\n\n")
	return b.String(), nil
}

func (g *generator) genHelpers() string {
	return `
func callAny(fn any, args ...any) any {
	values := make([]reflect.Value, len(args))
	for i, arg := range args {
		values[i] = reflect.ValueOf(arg)
	}
	out := reflect.ValueOf(fn).Call(values)
	if len(out) == 0 {
		return nil
	}
	return out[0].Interface()
}

`
}

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

func goMethodReturnType(ret []string) string {
	if len(ret) == 0 {
		return ""
	}
	if len(ret) == 1 {
		return ret[0]
	}
	return "(" + strings.Join(ret, ", ") + ")"
}

func goMyGoTypeString(t types.Type) string {
	if t == nil {
		return "any"
	}
	switch tt := t.(type) {
	case *types.Pointer:
		return "Ref[" + goMyGoTypeString(tt.Elem()) + "]"
	case *types.Basic:
		switch tt.Kind() {
		case types.Int:
			return "Int"
		case types.Int64:
			return "Int64"
		case types.Float64:
			return "Float64"
		case types.String:
			return "String"
		case types.Bool:
			return "Bool"
		}
	case *types.Named:
		if obj := tt.Obj(); obj != nil && obj.Pkg() != nil {
			return obj.Pkg().Name() + "." + obj.Name()
		}
		return tt.Obj().Name()
	}
	return t.String()
}

func (g *generator) findGoMethodSig(baseType, name string) (*goFuncSig, bool) {
	baseType = strings.TrimSpace(baseType)
	if strings.HasPrefix(baseType, "Ref[") && strings.HasSuffix(baseType, "]") {
		baseType = "*" + strings.TrimSuffix(strings.TrimPrefix(baseType, "Ref["), "]")
	}
	for _, imp := range g.pkg.ImportDecls {
		if !strings.HasPrefix(imp.Path, "go:") {
			continue
		}
		sigs, err := g.goPackageSigsFor(importPathForGo(imp.Path))
		if err != nil || sigs == nil {
			continue
		}
		if sig, ok := sigs.methods[baseType][name]; ok {
			return sig, true
		}
		if strings.HasPrefix(baseType, "*") {
			if sig, ok := sigs.methods[strings.TrimPrefix(baseType, "*")][name]; ok {
				return sig, true
			}
		} else {
			if sig, ok := sigs.methods["*"+baseType][name]; ok {
				return sig, true
			}
		}
	}
	return nil, false
}

func nodeLineFromExprSlice(exprs []Expr) int {
	for _, e := range exprs {
		if l, _ := common.NodePos(e); l != 0 {
			return l
		}
	}
	return 0
}

func nodeColFromExprSlice(exprs []Expr) int {
	for _, e := range exprs {
		_, c := common.NodePos(e)
		if c != 0 {
			return c
		}
	}
	return 0
}

func (g *generator) goTypeCompatible(expected, actual string) bool {
	if strings.TrimSpace(expected) == "any" {
		return true
	}
	expectedType, ok := goTypeFromString(expected)
	if !ok {
		if strings.HasPrefix(strings.TrimSpace(actual), "Ref[") && strings.HasSuffix(strings.TrimSpace(actual), "]") {
			inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(actual), "Ref["), "]")
			expectedNorm := normalizeGoTypeName(expected)
			if expectedNorm == inner {
				return true
			}
			if strings.HasPrefix(strings.TrimSpace(expected), "*") {
				if normalizeGoTypeName(expected[1:]) == inner {
					return true
				}
				if resolved, ok := g.resolveGoNamedType(strings.TrimSpace(expected[1:])); ok {
					if namedResolved, ok := g.resolveGoNamedType(inner); ok {
						ptrResolved := types.NewPointer(resolved)
						return types.Identical(namedResolved, resolved) || types.Identical(namedResolved.Underlying(), resolved.Underlying()) || types.AssignableTo(namedResolved, ptrResolved)
					}
				}
			}
			if resolved, ok := g.resolveGoNamedType(expected); ok {
				if namedResolved, ok := g.resolveGoNamedType(inner); ok {
					return types.AssignableTo(namedResolved, resolved) || types.Identical(namedResolved, resolved) || types.Identical(namedResolved.Underlying(), resolved.Underlying())
				}
			}
		}
		if strings.HasPrefix(strings.TrimSpace(expected), "*") {
			if resolved, ok := g.resolveGoNamedType(strings.TrimSpace(expected[1:])); ok {
				if actualType, ok := goTypeFromString(actual); ok {
					ptrResolved := types.NewPointer(resolved)
					return types.AssignableTo(actualType, ptrResolved) || types.Identical(actualType, ptrResolved) || types.Identical(actualType.Underlying(), ptrResolved.Underlying())
				}
			}
		}
		if resolved, ok := g.resolveGoNamedType(expected); ok {
			if actualType, ok := goTypeFromString(actual); ok {
				return types.AssignableTo(actualType, resolved) || types.Identical(actualType, resolved) || types.Identical(actualType.Underlying(), resolved.Underlying())
			}
		}
		if actualType, ok := goTypeFromString(actual); ok {
			if basicExpected, ok := goNamedUnderlyingBasic(expected); ok {
				return types.Identical(actualType.Underlying(), basicExpected)
			}
		}
		return strings.TrimSpace(expected) == strings.TrimSpace(actual)
	}
	actualType, ok := goTypeFromString(actual)
	if !ok {
		return false
	}
	if types.Identical(expectedType, actualType) || types.AssignableTo(actualType, expectedType) || types.Identical(actualType.Underlying(), expectedType.Underlying()) {
		return true
	}
	if isAnyType(expectedType) {
		return true
	}
	return false
}

func (g *generator) resolveGoNamedType(name string) (types.Type, bool) {
	name = normalizeGoTypeName(name)
	if name == "" || strings.Contains(name, "[") || strings.Contains(name, "(") {
		return nil, false
	}
	parts := strings.Split(name, ".")
	if len(parts) != 2 {
		return nil, false
	}
	pkgName, typeName := parts[0], parts[1]
	for _, imp := range g.pkg.ImportDecls {
		if !strings.HasPrefix(imp.Path, "go:") {
			continue
		}
		sigs, err := g.goPackageSigsFor(importPathForGo(imp.Path))
		if err != nil || sigs == nil || sigs.pkg == nil || sigs.pkg.Name() != pkgName {
			continue
		}
		if obj := sigs.pkg.Scope().Lookup(typeName); obj != nil {
			return obj.Type(), true
		}
	}
	return nil, false
}

func goTypeFromString(s string) (types.Type, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	switch s {
	case "any":
		return types.NewInterfaceType(nil, nil), true
	case "int":
		return types.Typ[types.Int], true
	case "int64":
		return types.Typ[types.Int64], true
	case "float64":
		return types.Typ[types.Float64], true
	case "string":
		return types.Typ[types.String], true
	case "bool":
		return types.Typ[types.Bool], true
	case "error":
		return types.Universe.Lookup("error").Type(), true
	}
	if strings.HasPrefix(s, "*") {
		if elem, ok := goTypeFromString(s[1:]); ok {
			return types.NewPointer(elem), true
		}
		return nil, false
	}
	if strings.HasPrefix(s, "[]") {
		if elem, ok := goTypeFromString(s[2:]); ok {
			return types.NewSlice(elem), true
		}
		return nil, false
	}
	return nil, false
}

func isAnyType(t types.Type) bool {
	if t == nil {
		return false
	}
	if iface, ok := t.Underlying().(*types.Interface); ok {
		return iface.NumMethods() == 0 && iface.NumEmbeddeds() == 0
	}
	return false
}

func goNamedUnderlyingBasic(name string) (types.Type, bool) {
	parts := strings.Split(strings.TrimSpace(name), ".")
	if len(parts) != 2 {
		return nil, false
	}
	switch parts[1] {
	case "Month", "Weekday", "Mode", "Flag", "Status", "Side":
		return types.Typ[types.Int], true
	case "Duration":
		return types.Typ[types.Int64], true
	case "Byte":
		return types.Typ[types.Uint8], true
	case "Rune":
		return types.Typ[types.Int32], true
	}
	return nil, false
}

func normalizeGoTypeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}
	if strings.HasPrefix(name, "*") {
		return "*" + normalizeGoTypeName(name[1:])
	}
	if strings.HasPrefix(name, "Ref[") && strings.HasSuffix(name, "]") {
		return "Ref[" + normalizeGoTypeName(strings.TrimSuffix(strings.TrimPrefix(name, "Ref["), "]")) + "]"
	}
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}

func isGoErrorType(t string) bool {
	t = strings.TrimSpace(t)
	return t == "error" || strings.HasSuffix(t, ".Error")
}

func (g *generator) translateAnyFuncCall(name string, args []Expr, ctx *exprCtx) (string, string, bool, error) {
	sourceType, ok := ctx.sourceTypes[name]
	if !ok || !strings.HasPrefix(strings.TrimSpace(sourceType), "func(") {
		return "", "", false, nil
	}
	_, ret := splitFuncType(sourceType)
	var argCodes []string
	for _, a := range args {
		code, _, err := g.translateExpr(a, ctx, "")
		if err != nil {
			return "", "", false, err
		}
		argCodes = append(argCodes, code)
	}
	actualName := name
	if bound, ok := ctx.bindings[name]; ok {
		actualName = bound
	}
	return fmt.Sprintf("%s.(%s)(%s)", actualName, sourceType, strings.Join(argCodes, ", ")), ret, true, nil
}

func (g *generator) translateEnumConstructor(enumName, name string, args []Expr, ctx *exprCtx, expected string) (string, string, error) {
	expectedEnum, enumArgs := splitTypeArgs(expected)
	if enumName == "" {
		enumName = expectedEnum
	}
	if enumName == "" {
		enumName = name
	}
	var typeArgs []string
	if len(enumArgs) > 0 {
		typeArgs = enumArgs
	}
	if len(typeArgs) == 0 {
		if _, ok := g.pkg.Enums[enumName]; ok && expected != "" {
			_, typeArgs = splitTypeArgs(expected)
		}
	}
	var variant *EnumVariant
	if enumDecl := g.pkg.Enums[enumName]; enumDecl != nil {
		variant = g.findVariant(enumDecl, name)
	}
	subst := map[string]string{}
	if enumDecl := g.pkg.Enums[enumName]; enumDecl != nil {
		for i, tp := range enumDecl.TypeParams {
			if i < len(typeArgs) {
				subst[tp] = typeArgs[i]
			}
		}
	}
	var argCodes []string
	for i, a := range args {
		argExpected := ""
		if variant != nil && i < len(variant.Fields) {
			argExpected = typeString(variant.Fields[i].Type, subst)
		}
		code, _, err := g.translateExpr(a, ctx, argExpected)
		if err != nil {
			return "", "", err
		}
		argCodes = append(argCodes, code)
	}
	typeArgStr := ""
	if len(typeArgs) > 0 {
		typeArgStr = "[" + strings.Join(typeArgs, ", ") + "]"
	}
	switch name {
	case "Some", "None":
		return fmt.Sprintf("%s%s(%s)", name, typeArgStr, strings.Join(argCodes, ", ")), expected, nil
	case "Ok", "Err":
		return fmt.Sprintf("%s%s(%s)", name, typeArgStr, strings.Join(argCodes, ", ")), expected, nil
	case "Nil", "Cons":
		return fmt.Sprintf("%s%s(%s)", name, typeArgStr, strings.Join(argCodes, ", ")), expected, nil
	default:
		return fmt.Sprintf("%s%s(%s)", name, typeArgStr, strings.Join(argCodes, ", ")), expected, nil
	}
}

func (g *generator) translateStructLit(n *StructLitExpr, ctx *exprCtx, expected string) (string, string, error) {
	st := g.pkg.Structs[n.TypeName]
	if st == nil {
		return "", "", common.ErrorAtPos(n.Line, n.Column, "unknown struct type %s", n.TypeName)
	}
	subst := map[string]string{}
	if len(n.TypeArgs) > 0 {
		if len(st.TypeParams) != len(n.TypeArgs) {
			return "", "", common.ErrorAtPos(n.Line, n.Column, "struct %s: type arity mismatch", n.TypeName)
		}
		for i, tp := range st.TypeParams {
			subst[tp] = g.goType(n.TypeArgs[i], ctx.typeParams)
		}
	} else if len(st.TypeParams) > 0 {
		if base, args := splitTypeArgs(expected); base == n.TypeName && len(args) == len(st.TypeParams) {
			for i, tp := range st.TypeParams {
				subst[tp] = args[i]
			}
		}
	}
	for _, f := range n.Fields {
		var fieldDecl *Field
		for i := range st.Fields {
			if st.Fields[i].Name == f.Name {
				fieldDecl = &st.Fields[i]
				break
			}
		}
		if fieldDecl == nil && f.Name == "embed" {
			for i := range st.Fields {
				if st.Fields[i].Name == "embed" {
					fieldDecl = &st.Fields[i]
					break
				}
			}
		}
		if fieldDecl == nil {
			return "", "", common.ErrorAtPos(f.Line, f.Column, "unknown field %s on struct %s", f.Name, n.TypeName)
		}
		fieldExpected := typeString(fieldDecl.Type, subst)
		code, typ, err := g.translateExpr(f.Value, ctx, fieldExpected)
		if err != nil {
			return "", "", err
		}
		_ = code
		unifyType(fieldDecl.Type, typ, typeParamSet(st.TypeParams), subst)
	}
	if len(st.TypeParams) > 0 {
		for _, tp := range st.TypeParams {
			if subst[tp] == "" {
				return "", "", common.ErrorAtPos(n.Line, n.Column, "struct %s: could not infer type parameters", n.TypeName)
			}
		}
	}
	fieldTypes := map[string]string{}
	for _, f := range st.Fields {
		fieldTypes[f.Name] = typeString(f.Type, subst)
	}
	parts := make([]string, 0, len(n.Fields))
	for _, f := range n.Fields {
		fieldType := fieldTypes[f.Name]
		if fieldType == "" && f.Name == "embed" {
			for _, stField := range st.Fields {
				if stField.Name == "embed" {
					fieldType = typeString(stField.Type, subst)
					break
				}
			}
		}
		if fieldType == "" {
			return "", "", common.ErrorAtPos(f.Line, f.Column, "unknown field %s on struct %s", f.Name, n.TypeName)
		}
		code, _, err := g.translateExpr(f.Value, ctx, fieldType)
		if err != nil {
			return "", "", err
		}
		key := exportName(f.Name)
		if f.Name == "embed" {
			key = fieldType
		}
		parts = append(parts, fmt.Sprintf("%s: %s", key, code))
	}
	typeArgStr := ""
	if len(n.TypeArgs) > 0 {
		var args []string
		for _, arg := range n.TypeArgs {
			args = append(args, g.goType(arg, ctx.typeParams))
		}
		typeArgStr = "[" + strings.Join(args, ", ") + "]"
	} else if len(st.TypeParams) > 0 {
		var args []string
		for _, tp := range st.TypeParams {
			args = append(args, subst[tp])
		}
		typeArgStr = "[" + strings.Join(args, ", ") + "]"
	}
	typeArgs := n.TypeArgs
	if len(typeArgs) == 0 && len(st.TypeParams) > 0 {
		typeArgs = make([]TypeExpr, 0, len(st.TypeParams))
		for _, tp := range st.TypeParams {
			typeArgs = append(typeArgs, &NamedType{Name: subst[tp]})
		}
	}
	return fmt.Sprintf("%s%s{%s}", n.TypeName, typeArgStr, strings.Join(parts, ", ")), typeString(&NamedType{Name: n.TypeName, Args: typeArgs}, nil), nil
}

func (g *generator) translateTypeclassCall(name string, args []Expr, ctx *exprCtx, expected string) (string, string, bool) {
	if ifaceName, ok := g.resolveTypeclassInterface(name, ctx); ok {
		methodIface := g.pkg.Interfaces[ifaceName]
		if methodIface == nil {
			return "", "", false
		}
		if funcName, ok := ctx.constraintFuncs[name]; ok {
			var argCodes []string
			for _, a := range args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return "", "", false
				}
				argCodes = append(argCodes, code)
			}
			return fmt.Sprintf("%s(%s)", funcName, strings.Join(argCodes, ", ")), methodReturnType(methodIface, name), true
		}
		if len(args) == 0 {
			return "", "", false
		}
		var argCodes []string
		for _, a := range args {
			code, _, err := g.translateExpr(a, ctx, "")
			if err != nil {
				return "", "", false
			}
			argCodes = append(argCodes, code)
		}
		return fmt.Sprintf("%s(%s)", dispatchFuncName(ifaceName, name), strings.Join(argCodes, ", ")), methodReturnType(methodIface, name), true
	}
	return "", "", false
}

func (g *generator) hasHelper(method, typ string) bool {
	_ = typ
	return true
}

func (g *generator) findVariant(enum *EnumDecl, name string) *EnumVariant {
	for i := range enum.Variants {
		if enum.Variants[i].Name == name {
			return &enum.Variants[i]
		}
	}
	return nil
}

func (g *generator) translateIdent(name string, ctx *exprCtx, expected string) (string, string, error) {
	if expr, ok := ctx.bindings[name]; ok {
		return expr, ctx.locals[name], nil
	}
	if typ, ok := ctx.locals[name]; ok {
		if typ == "any" {
			if sourceType, ok := ctx.sourceTypes[name]; ok && sourceType != "" && sourceType != "any" {
				return fmt.Sprintf("%s.(%s)", name, sourceType), sourceType, nil
			}
		}
		return name, typ, nil
	}
	switch name {
	case "true", "false":
		return name, "bool", nil
	case "None":
		base, args := splitTypeArgs(expected)
		if base != "" {
			if len(args) > 0 {
				return fmt.Sprintf("None[%s]()", strings.Join(args, ", ")), expected, nil
			}
			return "None[any]()", expected, nil
		}
		return "None[any]()", expected, nil
	case "Nil":
		base, args := splitTypeArgs(expected)
		if base != "" {
			if len(args) > 0 {
				return fmt.Sprintf("Nil[%s]()", strings.Join(args, ", ")), expected, nil
			}
			return "Nil[any]()", expected, nil
		}
		return "Nil[any]()", expected, nil
	}
	if enumName, ok := g.variantByName[name]; ok {
		return g.translateEnumConstructor(enumName, name, nil, ctx, expected)
	}
	if typeclassHelper, typ, ok := g.translateTypeclassIdent(name, ctx, expected); ok {
		return typeclassHelper, typ, nil
	}
	return name, ctx.locals[name], nil
}

func (g *generator) translateTypeclassIdent(name string, ctx *exprCtx, expected string) (string, string, bool) {
	if ifaceName, ok := g.resolveTypeclassInterface(name, ctx); ok {
		if funcName, ok := ctx.constraintFuncs[name]; ok {
			return funcName, expected, true
		}
		return dispatchFuncName(ifaceName, name), expected, true
	}
	return "", "", false
}

func (g *generator) resolveTypeclassInterface(name string, ctx *exprCtx) (string, bool) {
	if bindings, ok := ctx.typeclassMethods[name]; ok && len(bindings) > 0 {
		return typeclassBindingBest(bindings).Interface, true
	}
	if ifaceName, ok := g.interfaceByMethod[name]; ok {
		return ifaceName, true
	}
	return "", false
}

func typeclassBindingBest(bindings []typeclassBinding) typeclassBinding {
	best := bindings[0]
	for _, b := range bindings[1:] {
		if betterMatch(b.Score, best.Score) {
			best = b
		}
	}
	return best
}

func typeclassFuncType(paramTypes []string, retType string) string {
	if len(paramTypes) == 0 {
		if retType == "" {
			return "func()"
		}
		return "func() " + retType
	}
	fn := "func(" + strings.Join(paramTypes, ", ") + ")"
	if retType != "" {
		fn += " " + retType
	}
	return fn
}

func typeclassMatchScore(args []TypeExpr, scopeTypes map[string]struct{}) matchScore {
	var score matchScore
	for _, arg := range args {
		score = score.add(typeMatchScore(arg, scopeTypes))
	}
	return score
}

func typeMatchScore(t TypeExpr, scopeTypes map[string]struct{}) matchScore {
	switch tt := t.(type) {
	case *NamedType:
		if scopeTypes != nil {
			if _, ok := scopeTypes[tt.Name]; ok && len(tt.Args) == 0 {
				return matchScore{TypeParams: 1}
			}
		}
		score := matchScore{ConcreteTypes: 1}
		for _, a := range tt.Args {
			score = score.add(typeMatchScore(a, scopeTypes))
		}
		return score
	case *FuncType:
		score := matchScore{ConcreteTypes: 1}
		for _, p := range tt.Params {
			score = score.add(typeMatchScore(p, scopeTypes))
		}
		score = score.add(typeMatchScore(tt.Ret, scopeTypes))
		return score
	default:
		return matchScore{AnyTypes: 1}
	}
}

func (m matchScore) add(other matchScore) matchScore {
	m.ConcreteTypes += other.ConcreteTypes
	m.TypeParams += other.TypeParams
	m.AnyTypes += other.AnyTypes
	return m
}

func betterMatch(a, b matchScore) bool {
	if a.ConcreteTypes != b.ConcreteTypes {
		return a.ConcreteTypes > b.ConcreteTypes
	}
	if a.TypeParams != b.TypeParams {
		return a.TypeParams < b.TypeParams
	}
	return a.AnyTypes < b.AnyTypes
}

func sameMatch(a, b matchScore) bool {
	return a.ConcreteTypes == b.ConcreteTypes && a.TypeParams == b.TypeParams && a.AnyTypes == b.AnyTypes
}

func (g *generator) lookupFieldType(baseType, field string) string {
	base, args := splitTypeArgs(baseType)
	st := g.pkg.Structs[base]
	if st == nil {
		return ""
	}
	subst := map[string]string{}
	for i, tp := range st.TypeParams {
		if i < len(args) {
			subst[tp] = args[i]
		}
	}
	for _, f := range st.Fields {
		if f.Name == field {
			return typeString(f.Type, subst)
		}
	}
	for _, f := range st.Fields {
		if f.Name != "embed" {
			continue
		}
		embeddedType := typeString(f.Type, subst)
		if t := g.lookupFieldType(embeddedType, field); t != "" {
			return t
		}
	}
	return ""
}

func (g *generator) goType(t TypeExpr, typeParams map[string]struct{}) string {
	switch tt := t.(type) {
	case *NamedType:
		if typeParams != nil {
			if _, ok := typeParams[tt.Name]; ok && len(tt.Args) == 0 {
				return tt.Name
			}
		}
		switch tt.Name {
		case "Int":
			return "int"
		case "Int64":
			return "int64"
		case "Float64":
			return "float64"
		case "String":
			return "string"
		case "Bool":
			return "bool"
		case "Any":
			return "any"
		case "Unit":
			return "struct{}"
		case "Ref":
			if len(tt.Args) == 1 {
				return "*" + g.goType(tt.Args[0], typeParams)
			}
		}
		if len(tt.Args) == 0 {
			return tt.Name
		}
		args := make([]string, 0, len(tt.Args))
		for _, a := range tt.Args {
			args = append(args, g.goType(a, typeParams))
		}
		return tt.Name + "[" + strings.Join(args, ", ") + "]"
	case *FuncType:
		params := make([]string, 0, len(tt.Params))
		for _, p := range tt.Params {
			params = append(params, g.goType(p, typeParams))
		}
		ret := g.goReturnType(tt.Ret, typeParams)
		if ret == "" {
			return "func(" + strings.Join(params, ", ") + ")"
		}
		return "func(" + strings.Join(params, ", ") + ") " + ret
	default:
		return "any"
	}
}

func (g *generator) goReturnType(t TypeExpr, typeParams map[string]struct{}) string {
	if isUnitType(t) {
		return ""
	}
	return g.goType(t, typeParams)
}

func (g *generator) constraintTypeArgs(args []TypeExpr, typeParams map[string]struct{}) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, g.goType(a, typeParams))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func containsTypeParam(t TypeExpr, typeParams map[string]struct{}) bool {
	switch tt := t.(type) {
	case *NamedType:
		if typeParams != nil {
			if _, ok := typeParams[tt.Name]; ok && len(tt.Args) == 0 {
				return true
			}
		}
		for _, a := range tt.Args {
			if containsTypeParam(a, typeParams) {
				return true
			}
		}
	case *FuncType:
		for _, p := range tt.Params {
			if containsTypeParam(p, typeParams) {
				return true
			}
		}
		return containsTypeParam(tt.Ret, typeParams)
	}
	return false
}

func exprUsesIdent(e Expr, name string) bool {
	switch n := e.(type) {
	case *IdentExpr:
		return n.Name == name
	case *CallExpr:
		if exprUsesIdent(n.Callee, name) {
			return true
		}
		for _, arg := range n.Args {
			if exprUsesIdent(arg, name) {
				return true
			}
		}
	case *StructLitExpr:
		for _, field := range n.Fields {
			if exprUsesIdent(field.Value, name) {
				return true
			}
		}
	case *BinaryExpr:
		return exprUsesIdent(n.Left, name) || exprUsesIdent(n.Right, name)
	case *PrefixExpr:
		return exprUsesIdent(n.Expr, name)
	case *FieldExpr:
		return exprUsesIdent(n.Expr, name)
	case *FuncLitExpr:
		for _, p := range n.Params {
			if p.Name == name {
				return false
			}
		}
		return exprUsesIdent(n.Body, name)
	case *SwitchExpr:
		if exprUsesIdent(n.Target, name) {
			return true
		}
		for _, c := range n.Cases {
			if exprUsesIdent(c.Body, name) {
				return true
			}
		}
	case *IfExpr:
		return exprUsesIdent(n.Cond, name) || exprUsesIdent(n.Then, name) || exprUsesIdent(n.Else, name)
	case *BlockExpr:
		for _, stmt := range n.Stmts {
			switch s := stmt.(type) {
			case *ExprStmt:
				if exprUsesIdent(s.Expr, name) {
					return true
				}
			case *LetStmt:
				if exprUsesIdent(s.Value, name) {
					return true
				}
			case *AssignStmt:
				if exprUsesIdent(s.Value, name) {
					return true
				}
			}
		}
	}
	return false
}

func (g *generator) implTypeKey(args []TypeExpr) string {
	if len(args) == 0 {
		return ""
	}
	var out []string
	for _, a := range args {
		out = append(out, typeKeyFromType(g.goType(a, nil)))
	}
	return "_" + strings.Join(out, "_")
}

func (ctx *exprCtx) child() *exprCtx {
	dup := &exprCtx{
		locals:           map[string]string{},
		bindings:         map[string]string{},
		sourceTypes:      map[string]string{},
		mutable:          map[string]bool{},
		typeParams:       map[string]struct{}{},
		constraintFuncs:  map[string]string{},
		typeclassMethods: map[string][]typeclassBinding{},
		retType:          ctx.retType,
		currentImpl:      ctx.currentImpl,
	}
	for k, v := range ctx.locals {
		dup.locals[k] = v
	}
	for k, v := range ctx.bindings {
		dup.bindings[k] = v
	}
	for k, v := range ctx.sourceTypes {
		dup.sourceTypes[k] = v
	}
	for k, v := range ctx.mutable {
		dup.mutable[k] = v
	}
	for k := range ctx.typeParams {
		dup.typeParams[k] = struct{}{}
	}
	for k, v := range ctx.constraintFuncs {
		dup.constraintFuncs[k] = v
	}
	for k, v := range ctx.typeclassMethods {
		dup.typeclassMethods[k] = append([]typeclassBinding(nil), v...)
	}
	return dup
}

func genTypeParams(params []string) string {
	if len(params) == 0 {
		return ""
	}
	parts := make([]string, 0, len(params))
	for _, p := range params {
		parts = append(parts, p+" any")
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func typeArgList(params []string) string {
	if len(params) == 0 {
		return ""
	}
	return "[" + strings.Join(params, ", ") + "]"
}

func typeParamSet(params []string) map[string]struct{} {
	m := make(map[string]struct{}, len(params))
	for _, p := range params {
		m[p] = struct{}{}
	}
	return m
}

func exportName(name string) string {
	if name == "" {
		return name
	}
	r := []rune(name)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func toPackageName(name string) string {
	if name == "" {
		return "main"
	}
	return strings.ToLower(sanitizeIdent(name))
}

func sanitizeIdent(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			if i == 0 && unicode.IsDigit(r) {
				b.WriteRune('_')
			}
			b.WriteRune(r)
			continue
		}
		b.WriteRune('_')
	}
	if b.Len() == 0 {
		return "_"
	}
	return b.String()
}

func variantGoTypeName(enumName, variant string) string {
	return enumName + variant
}

func dictVarName(iface string) string {
	return strings.ToLower(iface[:1]) + iface[1:] + "Dict"
}

func dispatchRegistryName(iface, method string) string {
	return iface + "_" + method + "DispatchRegistry"
}

func dispatchFuncName(iface, method string) string {
	return iface + "_" + method
}

func (g *generator) sortedTypeclassNames() []string {
	names := make([]string, 0, len(g.pkg.Interfaces))
	for name := range g.pkg.Interfaces {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (g *generator) implDispatchKey(params []Param, subst map[string]string) string {
	return dispatchKeyForTypes(g.paramTypes(params, subst))
}

func (g *generator) paramTypes(params []Param, subst map[string]string) []string {
	out := make([]string, 0, len(params))
	for _, p := range params {
		out = append(out, typeString(p.Type, subst))
	}
	return out
}

func dispatchKeyForTypes(types []string) string {
	if len(types) == 0 {
		return "unit"
	}
	parts := make([]string, 0, len(types))
	for _, typ := range types {
		parts = append(parts, typeKeyFromType(typ))
	}
	return strings.Join(parts, "|")
}

func dispatchKeyExpr(params []Param, subst map[string]string) string {
	if len(params) == 0 {
		return "\"unit\""
	}
	var parts []string
	for _, p := range params {
		typ := "reflect.TypeOf(" + p.Name + ").String()"
		if subst != nil {
			_ = subst
		}
		parts = append(parts, "typeKeyFromType("+typ+")")
	}
	return strings.Join(parts, ` + "|" + `)
}

func helperFuncName(method, typeKey string) string {
	typeKey = strings.TrimPrefix(typeKey, "_")
	return sanitizeIdent(method + "_" + typeKey)
}

func typeKeyFromType(typ string) string {
	typ = strings.ReplaceAll(typ, "[", "_")
	typ = strings.ReplaceAll(typ, "]", "")
	typ = strings.ReplaceAll(typ, ", ", "_")
	typ = strings.ReplaceAll(typ, ",", "_")
	typ = strings.ReplaceAll(typ, " ", "")
	typ = strings.ReplaceAll(typ, "*", "Ptr")
	typ = strings.ReplaceAll(typ, ".", "_")
	typ = strings.ReplaceAll(typ, "func(", "Func_")
	typ = strings.ReplaceAll(typ, ")", "")
	return sanitizeIdent(strings.ToLower(typ))
}

func splitTypeArgs(typ string) (string, []string) {
	typ = strings.TrimSpace(typ)
	if typ == "" {
		return "", nil
	}
	idx := strings.Index(typ, "[")
	if idx < 0 {
		return typ, nil
	}
	end := strings.LastIndex(typ, "]")
	if end < 0 || end < idx {
		return typ, nil
	}
	name := typ[:idx]
	inner := typ[idx+1 : end]
	if inner == "" {
		return name, nil
	}
	return name, splitTopLevel(inner, ',')
}

func splitFuncType(typ string) ([]string, string) {
	typ = strings.TrimSpace(typ)
	if !strings.HasPrefix(typ, "func(") {
		return nil, ""
	}
	start := strings.Index(typ, "(")
	depth := 0
	for i := start; i < len(typ); i++ {
		switch typ[i] {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
			if depth == 0 && typ[i] == ')' {
				params := strings.TrimSpace(typ[start+1 : i])
				ret := strings.TrimSpace(typ[i+1:])
				if params == "" {
					return nil, ret
				}
				return splitTopLevel(params, ','), ret
			}
		}
	}
	return nil, ""
}

func funcReturnType(typ string) string {
	_, ret := splitFuncType(typ)
	return ret
}

func splitTopLevel(s string, sep rune) []string {
	var parts []string
	depth := 0
	start := 0
	for i, r := range s {
		switch r {
		case '[', '(':
			depth++
		case ']', ')':
			if depth > 0 {
				depth--
			}
		default:
			if r == sep && depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + len(string(r))
			}
		}
	}
	parts = append(parts, strings.TrimSpace(s[start:]))
	return parts
}

func methodReturnType(iface *InterfaceDecl, method string) string {
	for _, m := range iface.Methods {
		if m.Name == method {
			return typeStringReturn(m.Ret, nil)
		}
	}
	return "any"
}

func inferFuncTypeArgs(fn *FuncDecl, argTypes []string, expectedRet string, inScope map[string]struct{}) map[string]string {
	subst := map[string]string{}
	params := map[string]struct{}{}
	for _, tp := range fn.TypeParams {
		params[tp] = struct{}{}
	}
	for i, p := range fn.Params {
		if i >= len(argTypes) {
			break
		}
		unifyType(p.Type, argTypes[i], params, subst)
	}
	if expectedRet != "" {
		unifyType(fn.Ret, expectedRet, params, subst)
	}
	return subst
}

func unifyType(pattern TypeExpr, actual string, params map[string]struct{}, subst map[string]string) {
	if actual == "" || actual == "any" {
		return
	}
	switch p := pattern.(type) {
	case *NamedType:
		if _, ok := params[p.Name]; ok && len(p.Args) == 0 {
			subst[p.Name] = actual
			return
		}
		patternName := primitiveGoName(p.Name)
		if patternName == "" {
			patternName = p.Name
		}
		actualName, actualArgs := splitTypeArgs(actual)
		if patternName != actualName || len(p.Args) != len(actualArgs) {
			return
		}
		for i, arg := range p.Args {
			unifyType(arg, actualArgs[i], params, subst)
		}
	case *FuncType:
		actualParams, actualRet := splitFuncType(actual)
		if len(actualParams) != len(p.Params) {
			return
		}
		for i, param := range p.Params {
			unifyType(param, actualParams[i], params, subst)
		}
		unifyType(p.Ret, actualRet, params, subst)
	}
}

func primitiveGoName(name string) string {
	switch name {
	case "Int":
		return "int"
	case "Int64":
		return "int64"
	case "Float64":
		return "float64"
	case "String":
		return "string"
	case "Bool":
		return "bool"
	case "Unit":
		return "struct{}"
	default:
		return ""
	}
}

func typeString(t TypeExpr, subst map[string]string) string {
	switch tt := t.(type) {
	case *NamedType:
		if subst != nil {
			if repl, ok := subst[tt.Name]; ok && len(tt.Args) == 0 {
				return repl
			}
		}
		switch tt.Name {
		case "Int":
			return "int"
		case "Int64":
			return "int64"
		case "Float64":
			return "float64"
		case "String":
			return "string"
		case "Bool":
			return "bool"
		case "Any":
			return "any"
		case "Unit":
			return "struct{}"
		case "Ref":
			if len(tt.Args) == 1 {
				return "*" + typeString(tt.Args[0], subst)
			}
		}
		if len(tt.Args) == 0 {
			return tt.Name
		}
		args := make([]string, 0, len(tt.Args))
		for _, a := range tt.Args {
			args = append(args, typeString(a, subst))
		}
		return tt.Name + "[" + strings.Join(args, ", ") + "]"
	case *FuncType:
		params := make([]string, 0, len(tt.Params))
		for _, p := range tt.Params {
			params = append(params, typeString(p, subst))
		}
		ret := typeStringReturn(tt.Ret, subst)
		if ret == "" {
			return "func(" + strings.Join(params, ", ") + ")"
		}
		return "func(" + strings.Join(params, ", ") + ") " + ret
	default:
		return "any"
	}
}

func typeStringReturn(t TypeExpr, subst map[string]string) string {
	if isUnitType(t) {
		return ""
	}
	return typeString(t, subst)
}

func isUnitType(t TypeExpr) bool {
	tt, ok := t.(*NamedType)
	return ok && tt.Name == "Unit" && len(tt.Args) == 0
}

func (g *generator) writeUnitBody(b *strings.Builder, expr, exprType string) {
	b.WriteString("\t")
	if exprType == "" {
		b.WriteString(expr)
		b.WriteString("\n")
		return
	}
	b.WriteString("_ = ")
	b.WriteString(expr)
	b.WriteString("\n")
}

func importAliasForPath(path string) string {
	path = importPathForGo(path)
	if path == "" {
		return ""
	}
	path = strings.TrimSuffix(path, "/")
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return sanitizeIdent(path[idx+1:])
	}
	return sanitizeIdent(path)
}

func importPathForGo(path string) string {
	return strings.TrimPrefix(path, "go:")
}

func (g *generator) isImportAlias(name string) bool {
	if g == nil {
		return false
	}
	_, ok := g.importAliases[name]
	return ok
}
