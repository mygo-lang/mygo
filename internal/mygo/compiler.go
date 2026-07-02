package mygo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type Package struct {
	Name          string
	Imports       map[string]struct{}
	ImportDecls   []*ImportDecl
	ImportAliases map[string]string
	Decls         []Decl
	Enums         map[string]*EnumDecl
	Structs       map[string]*StructDecl
	Interfaces    map[string]*InterfaceDecl
	Funcs         map[string]*FuncDecl
	Impls         []*ImplDecl
}

func CompileDir(dir string) (string, error) {
	pkg, err := loadPackage(dir)
	if err != nil {
		return "", err
	}
	out := filepath.Join(dir, "zz_mygo.gen.go")
	src, err := pkg.Generate()
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(out, []byte(src), 0o644); err != nil {
		return "", err
	}
	return out, nil
}

func Sync(root string) ([]string, error) {
	var written []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, "bak") || base == ".git" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	dirs, err := mygoDirs(root)
	if err != nil {
		return nil, err
	}
	for _, dir := range dirs {
		out, err := CompileDir(dir)
		if err != nil {
			return nil, err
		}
		written = append(written, out)
	}
	sort.Strings(written)
	return written, nil
}

func mygoDirs(root string) ([]string, error) {
	seen := map[string]struct{}{}
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if path != root && (strings.HasPrefix(base, "bak") || base == ".git" || base == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".mygo") {
			dir := filepath.Dir(path)
			if _, ok := seen[dir]; !ok {
				seen[dir] = struct{}{}
				dirs = append(dirs, dir)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(dirs)
	return dirs, nil
}

func loadPackage(dir string) (*Package, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	pkg := &Package{
		Imports:       map[string]struct{}{},
		ImportAliases: map[string]string{},
		Enums:         map[string]*EnumDecl{},
		Structs:       map[string]*StructDecl{},
		Interfaces:    map[string]*InterfaceDecl{},
		Funcs:         map[string]*FuncDecl{},
	}
	moduleName := ""
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".mygo") || strings.HasSuffix(name, ".gen.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		file, err := ParseFile(string(src))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if file.Module != "" {
			if moduleName == "" {
				moduleName = file.Module
			} else if moduleName != file.Module {
				return nil, fmt.Errorf("%s: %w", name, errorAtLine(file.ModuleLine, "module %q conflicts with %q", file.Module, moduleName))
			}
		}
		pkg.Decls = append(pkg.Decls, file.Decls...)
	}
	if moduleName == "" {
		moduleName = filepath.Base(dir)
	}
	pkg.Name = toPackageName(moduleName)
	for _, decl := range pkg.Decls {
		switch d := decl.(type) {
		case *ImportDecl:
			pkg.Imports[d.Path] = struct{}{}
			pkg.ImportDecls = append(pkg.ImportDecls, d)
			alias := d.Alias
			if alias == "" {
				alias = importAliasForPath(d.Path)
			}
			if prev, ok := pkg.ImportAliases[alias]; ok && prev != d.Path {
				return nil, errorAtLine(d.Line, "import alias %q conflicts between %q and %q", alias, prev, d.Path)
			}
			pkg.ImportAliases[alias] = d.Path
		case *EnumDecl:
			pkg.Enums[d.Name] = d
		case *StructDecl:
			pkg.Structs[d.Name] = d
		case *InterfaceDecl:
			pkg.Interfaces[d.Name] = d
		case *FuncDecl:
			pkg.Funcs[d.Name] = d
		case *ImplDecl:
			pkg.Impls = append(pkg.Impls, d)
		}
	}
	return pkg, nil
}

func (p *Package) Generate() (string, error) {
	g := &generator{
		pkg:               p,
		importAliases:     p.ImportAliases,
		interfaceByMethod: map[string]string{},
		variantByName:     map[string]string{},
	}
	for name, iface := range p.Interfaces {
		for _, m := range iface.Methods {
			g.interfaceByMethod[m.Name] = name
		}
	}
	for enumName, enum := range p.Enums {
		for _, variant := range enum.Variants {
			g.variantByName[variant.Name] = enumName
		}
	}
	var body strings.Builder
	for _, decl := range p.Decls {
		switch d := decl.(type) {
		case *EnumDecl:
			body.WriteString(g.genEnum(d))
		case *StructDecl:
			body.WriteString(g.genStruct(d))
		case *InterfaceDecl:
			body.WriteString(g.genInterface(d))
		}
	}
	globals, err := g.genGlobals()
	if err != nil {
		return "", err
	}
	body.WriteString(globals)
	for _, decl := range p.Decls {
		if impl, ok := decl.(*ImplDecl); ok {
			s, err := g.genImpl(impl)
			if err != nil {
				return "", err
			}
			body.WriteString(s)
		}
	}
	for _, decl := range p.Decls {
		if fn, ok := decl.(*FuncDecl); ok {
			s, err := g.genFunc(fn)
			if err != nil {
				return "", err
			}
			body.WriteString(s)
		}
	}
	if g.needsCallAny {
		body.WriteString(g.genHelpers())
	}
	imports := p.sortedImports()
	if g.needsCallAny && !hasImportPath(imports, "reflect") {
		imports = append(imports, importSpec{Path: "reflect"})
		sort.Slice(imports, func(i, j int) bool {
			if imports[i].Alias == imports[j].Alias {
				return imports[i].Path < imports[j].Path
			}
			return imports[i].Alias < imports[j].Alias
		})
	}
	if len(p.Interfaces) > 0 && !hasImportPath(imports, "reflect") {
		imports = append(imports, importSpec{Path: "reflect"})
		sort.Slice(imports, func(i, j int) bool {
			if imports[i].Alias == imports[j].Alias {
				return imports[i].Path < imports[j].Path
			}
			return imports[i].Alias < imports[j].Alias
		})
	}
	var out strings.Builder
	out.WriteString("// Code generated by mygo; DO NOT EDIT.\n")
	out.WriteString("package ")
	out.WriteString(p.Name)
	out.WriteString("\n\n")
	if len(imports) > 0 {
		out.WriteString("import (\n")
		for _, imp := range imports {
			out.WriteString("\t")
			if imp.Alias != "" && imp.Alias != importAliasForPath(imp.Path) {
				out.WriteString(imp.Alias)
				out.WriteString(" ")
			}
			out.WriteString(strconv.Quote(importPathForGo(imp.Path)))
			out.WriteString("\n")
		}
		out.WriteString(")\n\n")
	}
	out.WriteString(body.String())
	return out.String(), nil
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
			return "", errorAtLine(s.Line, "global binding %q: %v", s.Name, err)
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
	needsCallAny      bool
	localSeq          int
}

type exprCtx struct {
	locals           map[string]string
	bindings         map[string]string
	sourceTypes      map[string]string
	mutable          map[string]bool
	typeParams       map[string]struct{}
	constraintFuncs  map[string]string
	typeclassMethods map[string][]typeclassBinding
	retType          string
	currentImpl      string
}

type typeclassBinding struct {
	Interface  string
	Score      matchScore
	ParamTypes []string
	RetType    string
}

type matchScore struct {
	ConcreteTypes int
	TypeParams    int
	AnyTypes      int
}

type bindingInfo struct {
	Expr string
	Type string
}

type importSpec struct {
	Alias string
	Path  string
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
		return "", errorAtLine(d.Line, "impl %s: missing interface declaration", d.Name)
	}
	if len(iface.TypeParams) != len(d.TypeArgs) {
		return "", errorAtLine(d.Line, "impl %s: type arity mismatch", d.Name)
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
			return "", errorAtLine(c.Line, "function %s: missing interface %s", d.Name, c.Name)
		}
		if len(iface.TypeParams) != len(c.Args) {
			return "", errorAtLine(c.Line, "function %s: type arity mismatch for %s", d.Name, c.Name)
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

func (g *generator) translateExpr(e Expr, ctx *exprCtx, expected string) (string, string, error) {
	switch n := e.(type) {
	case *IdentExpr:
		return g.translateIdent(n.Name, ctx, expected)
	case *LiteralExpr:
		switch n.Kind {
		case "number":
			if strings.Contains(n.Value, ".") {
				return n.Value, "float64", nil
			}
			return n.Value, "int", nil
		case "string":
			return strconv.Quote(n.Value), "string", nil
		}
	case *BinaryExpr:
		if n.Op == "|>" {
			left, _, err := g.translateExpr(n.Left, ctx, "")
			if err != nil {
				return "", "", err
			}
			switch right := n.Right.(type) {
			case *CallExpr:
				callee, _, err := g.translateExpr(right.Callee, ctx, "")
				if err != nil {
					return "", "", err
				}
				args := make([]string, 0, len(right.Args)+1)
				for _, a := range right.Args {
					code, _, err := g.translateExpr(a, ctx, "")
					if err != nil {
						return "", "", err
					}
					args = append(args, code)
				}
				args = append(args, left)
				_, rt, err := g.translateExpr(right, ctx, "")
				if err != nil {
					return "", "", err
				}
				return fmt.Sprintf("%s(%s)", callee, strings.Join(args, ", ")), rt, nil
			default:
				rhs, rt, err := g.translateExpr(n.Right, ctx, "")
				if err != nil {
					return "", "", err
				}
				return fmt.Sprintf("%s(%s)", rhs, left), rt, nil
			}
		}
		if n.Op == "<|" {
			right, _, err := g.translateExpr(n.Right, ctx, "")
			if err != nil {
				return "", "", err
			}
			switch left := n.Left.(type) {
			case *CallExpr:
				callee, _, err := g.translateExpr(left.Callee, ctx, "")
				if err != nil {
					return "", "", err
				}
				args := make([]string, 0, len(left.Args)+1)
				for _, a := range left.Args {
					code, _, err := g.translateExpr(a, ctx, "")
					if err != nil {
						return "", "", err
					}
					args = append(args, code)
				}
				args = append(args, right)
				_, lt, err := g.translateExpr(left, ctx, "")
				if err != nil {
					return "", "", err
				}
				return fmt.Sprintf("%s(%s)", callee, strings.Join(args, ", ")), lt, nil
			default:
				lhs, lt, err := g.translateExpr(n.Left, ctx, "")
				if err != nil {
					return "", "", err
				}
				return fmt.Sprintf("%s(%s)", lhs, right), lt, nil
			}
		}
		left, lt, err := g.translateExpr(n.Left, ctx, "")
		if err != nil {
			return "", "", err
		}
		right, rt, err := g.translateExpr(n.Right, ctx, "")
		if err != nil {
			return "", "", err
		}
		switch n.Op {
		case "+", "*", "==", "!=", "<", ">", "<=", ">=":
			resType := "bool"
			if n.Op == "+" || n.Op == "*" {
				resType = lt
				if resType == "" || resType == "any" {
					resType = rt
				}
			}
			return fmt.Sprintf("(%s %s %s)", left, n.Op, right), resType, nil
		}
	case *PrefixExpr:
		expr, typ, err := g.translateExpr(n.Expr, ctx, "")
		if err != nil {
			return "", "", err
		}
		if n.Op == "not" {
			return fmt.Sprintf("(!%s)", expr), "bool", nil
		}
		return expr, typ, nil
	case *FieldExpr:
		if baseIdent, ok := n.Expr.(*IdentExpr); ok {
			if enumDecl := g.pkg.Enums[baseIdent.Name]; enumDecl != nil {
				if variant := g.findVariant(enumDecl, n.Field); variant != nil {
					return g.translateEnumConstructor(baseIdent.Name, n.Field, nil, ctx, expected)
				}
			}
		}
		base, baseType, err := g.translateExpr(n.Expr, ctx, "")
		if err != nil {
			return "", "", err
		}
		if id, ok := n.Expr.(*IdentExpr); ok && g.isImportAlias(id.Name) {
			return fmt.Sprintf("%s.%s", base, n.Field), "any", nil
		}
		fieldType := g.lookupFieldType(baseType, n.Field)
		return fmt.Sprintf("%s.%s", base, exportName(n.Field)), fieldType, nil
	case *CallExpr:
		return g.translateCall(n, ctx, expected)
	case *StructLitExpr:
		return g.translateStructLit(n, ctx, expected)
	case *FuncLitExpr:
		return g.translateFuncLit(n, ctx)
	case *IfExpr:
		return g.translateIf(n, ctx, expected)
	case *SwitchExpr:
		return g.translateSwitch(n, ctx, expected)
	case *BlockExpr:
		return g.translateBlock(n, ctx, expected)
	}
	return "", "", errorAtLine(nodeLine(e), "unsupported expression %#v", e)
}

func (g *generator) translateBlock(n *BlockExpr, ctx *exprCtx, expected string) (string, string, error) {
	var b strings.Builder
	b.WriteString("func()")
	if expected != "" {
		b.WriteString(" ")
		b.WriteString(expected)
	}
	b.WriteString(" {\n")
	child := ctx.child()
	var lastWasExprStmt bool
	for i, stmt := range n.Stmts {
		isLast := i == len(n.Stmts)-1
		switch s := stmt.(type) {
		case *ExprStmt:
			lastWasExprStmt = isLast
			stmtExpected := ""
			if isLast {
				stmtExpected = expected
			}
			code, typ, err := g.translateExpr(s.Expr, child, stmtExpected)
			if err != nil {
				return "", "", err
			}
			if isLast && expected != "" {
				if typ == "" {
					return "", "", errorAtLine(nodeLine(s), "block must end with an expression returning %s", expected)
				}
				b.WriteString("\treturn ")
				b.WriteString(code)
				b.WriteString("\n")
				continue
			}
			b.WriteString("\t")
			if stmtIsStatementSafe(s.Expr) {
				b.WriteString(code)
			} else {
				b.WriteString("_ = ")
				b.WriteString(code)
			}
			b.WriteString("\n")
		case *LetStmt:
			lastWasExprStmt = false
			code, typ, err := g.translateExpr(s.Value, child, g.goType(s.Type, child.typeParams))
			if err != nil {
				return "", "", err
			}
			b.WriteString("\t")
			if s.Name == "_" {
				if stmtIsStatementSafe(s.Value) {
					b.WriteString(code)
				} else {
					b.WriteString("_ = ")
					b.WriteString(code)
				}
			} else {
				actualName := g.bindLocal(child, s.Name, typ, s.Mutable)
				bindType := typ
				if s.Type != nil {
					bindType = g.goType(s.Type, child.typeParams)
					b.WriteString("var ")
					b.WriteString(actualName)
					b.WriteString(" ")
					b.WriteString(bindType)
					b.WriteString(" = ")
					b.WriteString(code)
				} else {
					b.WriteString(actualName)
					b.WriteString(" := ")
					b.WriteString(code)
				}
				child.locals[s.Name] = bindType
				child.sourceTypes[s.Name] = bindType
				child.bindings[s.Name] = actualName
			}
			b.WriteString("\n")
		case *AssignStmt:
			lastWasExprStmt = false
			actualName, ok := child.bindings[s.Name]
			if !ok {
				return "", "", errorAtLine(s.Line, "unknown binding %q", s.Name)
			}
			if !child.mutable[actualName] {
				return "", "", errorAtLine(s.Line, "cannot assign to immutable binding %q", s.Name)
			}
			targetType := child.locals[s.Name]
			code, _, err := g.translateExpr(s.Value, child, targetType)
			if err != nil {
				return "", "", err
			}
			b.WriteString("\t")
			b.WriteString(actualName)
			b.WriteString(" = ")
			b.WriteString(code)
			b.WriteString("\n")
		default:
			lastWasExprStmt = false
			return "", "", errorAtLine(nodeLine(stmt), "unsupported statement %#v", stmt)
		}
	}
	if expected != "" && !lastWasExprStmt {
		return "", "", errorAtLine(nodeLine(n), "block must end with an expression returning %s", expected)
	}
	b.WriteString("}()")
	if expected != "" {
		return b.String(), expected, nil
	}
	return b.String(), "", nil
}

func stmtIsStatementSafe(expr Expr) bool {
	switch n := expr.(type) {
	case *CallExpr, *FuncLitExpr, *IfExpr, *SwitchExpr, *BlockExpr:
		return true
	case *BinaryExpr:
		return n.Op == "|>" || n.Op == "<|"
	default:
		return false
	}
}

func (g *generator) bindLocal(ctx *exprCtx, source, typ string, mutable bool) string {
	actual := sanitizeIdent(source)
	if actual == "" || actual == "_" {
		actual = "tmp"
	}
	g.localSeq++
	actual = fmt.Sprintf("%s_%d", actual, g.localSeq)
	ctx.bindings[source] = actual
	ctx.locals[source] = typ
	ctx.sourceTypes[source] = typ
	ctx.mutable[actual] = mutable
	return actual
}

func (g *generator) translateFuncLit(n *FuncLitExpr, outer *exprCtx) (string, string, error) {
	retType := g.goReturnType(n.Ret, outer.typeParams)
	var b strings.Builder
	b.WriteString("func(")
	child := outer.child()
	child.retType = retType
	for i, p := range n.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		tp := g.goType(p.Type, outer.typeParams)
		child.locals[p.Name] = tp
		b.WriteString(p.Name)
		b.WriteString(" ")
		b.WriteString(tp)
	}
	b.WriteString(")")
	if retType != "" {
		b.WriteString(" ")
		b.WriteString(retType)
	}
	b.WriteString(" {\n")
	body, bodyType, err := g.translateExpr(n.Body, child, retType)
	if err != nil {
		return "", "", err
	}
	if retType == "" {
		g.writeUnitBody(&b, body, bodyType)
	} else {
		b.WriteString("\treturn ")
		b.WriteString(body)
		b.WriteString("\n")
	}
	b.WriteString("}")
	return b.String(), retType, nil
}

func (g *generator) translateIf(n *IfExpr, ctx *exprCtx, expected string) (string, string, error) {
	cond, _, err := g.translateExpr(n.Cond, ctx, "")
	if err != nil {
		return "", "", err
	}
	thenCtx := ctx.child()
	elseCtx := ctx.child()
	thenCode, thenType, err := g.translateExpr(n.Then, thenCtx, expected)
	if err != nil {
		return "", "", err
	}
	elseCode, elseType, err := g.translateExpr(n.Else, elseCtx, expected)
	if err != nil {
		return "", "", err
	}
	resultType := expected
	if resultType == "" {
		switch {
		case thenType != "" && thenType == elseType:
			resultType = thenType
		case thenType != "":
			resultType = thenType
		default:
			resultType = elseType
		}
	}
	var b strings.Builder
	b.WriteString("func()")
	if resultType != "" {
		b.WriteString(" ")
		b.WriteString(resultType)
	}
	b.WriteString(" {\n")
	b.WriteString("\tif ")
	b.WriteString(cond)
	b.WriteString(" {\n")
	if resultType == "" {
		g.writeUnitBody(&b, thenCode, thenType)
	} else {
		b.WriteString("\t\treturn ")
		b.WriteString(thenCode)
		b.WriteString("\n")
	}
	b.WriteString("\t} else {\n")
	if resultType == "" {
		g.writeUnitBody(&b, elseCode, elseType)
	} else {
		b.WriteString("\t\treturn ")
		b.WriteString(elseCode)
		b.WriteString("\n")
	}
	b.WriteString("\t}\n")
	b.WriteString("}()")
	return b.String(), resultType, nil
}

func (g *generator) translateSwitch(n *SwitchExpr, ctx *exprCtx, expected string) (string, string, error) {
	targetCode, targetType, err := g.translateExpr(n.Target, ctx, "")
	if err != nil {
		return "", "", err
	}
	enumName, enumArgs := splitTypeArgs(targetType)
	enumDecl := g.pkg.Enums[enumName]
	if enumDecl == nil {
		return "", "", errorAtLine(n.Line, "switch target %q is not an enum", targetType)
	}
	needsSwitchVar := false
	for _, c := range n.Cases {
		if pat, ok := c.Pattern.(*VariantPattern); ok {
			for _, arg := range pat.Args {
				if exprUsesIdent(c.Body, arg) {
					needsSwitchVar = true
					break
				}
			}
			if needsSwitchVar {
				break
			}
		}
	}
	var b strings.Builder
	b.WriteString("func()")
	if expected != "" {
		b.WriteString(" ")
		b.WriteString(expected)
	}
	b.WriteString(" {\n")
	if needsSwitchVar {
		b.WriteString("\tswitch v := ")
		b.WriteString(targetCode)
		b.WriteString(".(type) {\n")
	} else {
		b.WriteString("\tswitch ")
		b.WriteString(targetCode)
		b.WriteString(".(type) {\n")
	}
	for _, c := range n.Cases {
		pat, bindings, err := g.translatePattern(c.Pattern, enumDecl, enumArgs, "v", c.Body)
		if err != nil {
			return "", "", err
		}
		b.WriteString("\tcase ")
		b.WriteString(pat)
		b.WriteString(":\n")
		child := ctx.child()
		for name, info := range bindings {
			child.locals[name] = info.Type
			child.bindings[name] = info.Expr
		}
		body, bodyType, err := g.translateExpr(c.Body, child, expected)
		if err != nil {
			return "", "", err
		}
		if expected == "" {
			b.WriteString("\t\t")
			if bodyType == "" {
				b.WriteString(body)
			} else {
				b.WriteString("_ = ")
				b.WriteString(body)
			}
			b.WriteString("\n")
		} else {
			b.WriteString("\t\treturn ")
			b.WriteString(body)
			b.WriteString("\n")
		}
	}
	if expected == "" {
		b.WriteString("\t}\n}()")
	} else {
		b.WriteString("\t}\n\tpanic(\"unreachable\")\n}()")
	}
	return b.String(), expected, nil
}

func (g *generator) translatePattern(p Pattern, enum *EnumDecl, enumArgs []string, switchVar string, body Expr) (string, map[string]bindingInfo, error) {
	switch pat := p.(type) {
	case *WildcardPattern:
		return "interface{}", nil, nil
	case *VariantPattern:
		variant := g.findVariant(enum, pat.Name)
		if variant == nil {
			return "", nil, errorAtLine(pat.Line, "unknown variant %s of %s", pat.Name, enum.Name)
		}
		tname := variantGoTypeName(enum.Name, variant.Name)
		if len(enumArgs) > 0 {
			tname += "[" + strings.Join(enumArgs, ", ") + "]"
		}
		bindings := map[string]bindingInfo{}
		for i, arg := range pat.Args {
			if i >= len(variant.Fields) {
				return "", nil, errorAtLine(pat.Line, "pattern %s arity mismatch", pat.Name)
			}
			if !exprUsesIdent(body, arg) {
				continue
			}
			bindings[arg] = bindingInfo{
				Expr: fmt.Sprintf("%s.F%d", switchVar, i),
				Type: g.goType(variant.Fields[i].Type, nil),
			}
		}
		return tname, bindings, nil
	default:
		return "", nil, errorAtLine(nodeLine(p), "unsupported pattern %#v", p)
	}
}

func (g *generator) translateCall(n *CallExpr, ctx *exprCtx, expected string) (string, string, error) {
	if field, ok := n.Callee.(*FieldExpr); ok {
		if baseIdent, ok := field.Expr.(*IdentExpr); ok {
			if enumDecl := g.pkg.Enums[baseIdent.Name]; enumDecl != nil {
				if variant := g.findVariant(enumDecl, field.Field); variant != nil {
					return g.translateEnumConstructor(baseIdent.Name, field.Field, n.Args, ctx, expected)
				}
			}
		}
	}
	if id, ok := n.Callee.(*IdentExpr); ok {
		if st := g.pkg.Structs[id.Name]; st != nil && len(n.Args) == len(st.Fields) && len(st.Fields) > 0 && strings.HasPrefix(st.Fields[0].Name, "F") {
			var args []string
			for i, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, g.goType(st.Fields[i].Type, nil))
				if err != nil {
					return "", "", err
				}
				args = append(args, code)
			}
			parts := make([]string, 0, len(args))
			for i, arg := range args {
				parts = append(parts, fmt.Sprintf("F%d: %s", i, arg))
			}
			return fmt.Sprintf("%s{%s}", id.Name, strings.Join(parts, ", ")), id.Name, nil
		}
		if typ, ok := ctx.locals[id.Name]; ok && typ == "any" {
			if code, ret, ok, err := g.translateAnyFuncCall(id.Name, n.Args, ctx); err != nil {
				return "", "", err
			} else if ok {
				return code, ret, nil
			}
			g.needsCallAny = true
			var args []string
			for _, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return "", "", err
				}
				args = append(args, code)
			}
			actualName := id.Name
			if bound, ok := ctx.bindings[id.Name]; ok {
				actualName = bound
			}
			return fmt.Sprintf("callAny(%s%s)", actualName, func() string {
				if len(args) == 0 {
					return ""
				}
				return ", " + strings.Join(args, ", ")
			}()), "any", nil
		}
		if typ, ok := ctx.locals[id.Name]; ok && strings.HasPrefix(strings.TrimSpace(typ), "func(") {
			var args []string
			for _, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return "", "", err
				}
				args = append(args, code)
			}
			return fmt.Sprintf("%s(%s)", id.Name, strings.Join(args, ", ")), funcReturnType(typ), nil
		}
		if g.pkg.Funcs[id.Name] != nil {
			fn := g.pkg.Funcs[id.Name]
			var args []string
			var argTypes []string
			for _, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return "", "", err
				}
				args = append(args, code)
				_, typ, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return "", "", err
				}
				argTypes = append(argTypes, typ)
			}
			subst := inferFuncTypeArgs(fn, argTypes, expected, ctx.typeParams)
			callee := id.Name
			if len(fn.TypeParams) > 0 && len(subst) == len(fn.TypeParams) {
				var typeArgs []string
				for _, tp := range fn.TypeParams {
					typeArgs = append(typeArgs, subst[tp])
				}
				callee += "[" + strings.Join(typeArgs, ", ") + "]"
			}
			for _, c := range fn.Where {
				iface := g.pkg.Interfaces[c.Name]
				if iface == nil {
					return "", "", errorAtLine(c.Line, "call %s: missing interface %s", fn.Name, c.Name)
				}
				if len(iface.TypeParams) != len(c.Args) {
					return "", "", errorAtLine(c.Line, "call %s: type arity mismatch for %s", fn.Name, c.Name)
				}
				cTypeArgs := make([]string, 0, len(c.Args))
				for _, arg := range c.Args {
					cTypeArgs = append(cTypeArgs, typeString(arg, subst))
				}
				for _, m := range iface.Methods {
					resolvedType := ""
					if len(cTypeArgs) > 0 {
						resolvedType = cTypeArgs[0]
					}
					if _, ok := ctx.typeParams[resolvedType]; ok {
						if helper, ok := ctx.constraintFuncs[m.Name]; ok {
							args = append(args, helper)
							continue
						}
					}
					args = append(args, helperFuncName(m.Name, typeKeyFromType(resolvedType)))
				}
			}
			retType := g.goReturnType(fn.Ret, ctx.typeParams)
			if len(subst) > 0 {
				retType = typeStringReturn(fn.Ret, subst)
			}
			return fmt.Sprintf("%s(%s)", callee, strings.Join(args, ", ")), retType, nil
		}
		if enumName, ok := g.variantByName[id.Name]; ok {
			return g.translateEnumConstructor(enumName, id.Name, n.Args, ctx, expected)
		}
		if helper, typ, ok := g.translateTypeclassCall(id.Name, n.Args, ctx, expected); ok {
			return helper, typ, nil
		}
		if typ, ok := ctx.locals[id.Name]; ok && typ == "any" {
			if code, ret, ok, err := g.translateAnyFuncCall(id.Name, n.Args, ctx); err != nil {
				return "", "", err
			} else if ok {
				return code, ret, nil
			}
			g.needsCallAny = true
			var args []string
			for _, a := range n.Args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return "", "", err
				}
				args = append(args, code)
			}
			actualName := id.Name
			if bound, ok := ctx.bindings[id.Name]; ok {
				actualName = bound
			}
			return fmt.Sprintf("callAny(%s%s)", actualName, func() string {
				if len(args) == 0 {
					return ""
				}
				return ", " + strings.Join(args, ", ")
			}()), "any", nil
		}
	}
	callee, calleeType, err := g.translateExpr(n.Callee, ctx, "")
	if err != nil {
		return "", "", err
	}
	var args []string
	for _, a := range n.Args {
		code, _, err := g.translateExpr(a, ctx, "")
		if err != nil {
			return "", "", err
		}
		args = append(args, code)
	}
	retType := expected
	if parsedRet := funcReturnType(calleeType); parsedRet != "" {
		retType = parsedRet
	}
	return fmt.Sprintf("%s(%s)", callee, strings.Join(args, ", ")), retType, nil
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
		return "", "", errorAtLine(n.Line, "unknown struct type %s", n.TypeName)
	}
	subst := map[string]string{}
	if len(n.TypeArgs) > 0 {
		if len(st.TypeParams) != len(n.TypeArgs) {
			return "", "", errorAtLine(n.Line, "struct %s: type arity mismatch", n.TypeName)
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
			return "", "", errorAtLine(f.Line, "unknown field %s on struct %s", f.Name, n.TypeName)
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
				return "", "", errorAtLine(n.Line, "struct %s: could not infer type parameters", n.TypeName)
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
			return "", "", errorAtLine(f.Line, "unknown field %s on struct %s", f.Name, n.TypeName)
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
		case "String":
			return "string"
		case "Bool":
			return "bool"
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
