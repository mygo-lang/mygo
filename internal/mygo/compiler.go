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
				return nil, fmt.Errorf("%s: module %q conflicts with %q", name, file.Module, moduleName)
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
				return nil, fmt.Errorf("import alias %q conflicts between %q and %q", alias, prev, d.Path)
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
}

type exprCtx struct {
	locals          map[string]string
	bindings        map[string]string
	sourceTypes     map[string]string
	typeParams      map[string]struct{}
	constraintFuncs map[string]string
	retType         string
	currentImpl     string
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
		return "", fmt.Errorf("impl %s: missing interface declaration", d.Name)
	}
	if len(iface.TypeParams) != len(d.TypeArgs) {
		return "", fmt.Errorf("impl %s: type arity mismatch", d.Name)
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
		locals:          map[string]string{},
		bindings:        map[string]string{},
		sourceTypes:     map[string]string{},
		typeParams:      typeParamSet(d.TypeParams),
		constraintFuncs: map[string]string{},
		retType:         g.goReturnType(d.Ret, typeParamSet(d.TypeParams)),
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
	}
	for _, c := range d.Where {
		b.WriteString(", ")
		iface := g.pkg.Interfaces[c.Name]
		if iface == nil {
			return "", fmt.Errorf("function %s: missing interface %s", d.Name, c.Name)
		}
		if len(iface.TypeParams) != len(c.Args) {
			return "", fmt.Errorf("function %s: type arity mismatch for %s", d.Name, c.Name)
		}
		subst := map[string]string{}
		for i, tp := range iface.TypeParams {
			subst[tp] = g.goType(c.Args[i], typeParamSet(d.TypeParams))
		}
		for i, m := range iface.Methods {
			if i > 0 {
				b.WriteString(", ")
			}
			paramName := m.Name + "Fn"
			ctx.constraintFuncs[m.Name] = paramName
			b.WriteString(paramName)
			b.WriteString(" ")
			b.WriteString("func(")
			for i, p := range m.Params {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(typeString(p.Type, subst))
			}
			b.WriteString(") ")
			b.WriteString(typeStringReturn(m.Ret, subst))
		}
	}
	retType := g.goReturnType(d.Ret, typeParamSet(d.TypeParams))
	b.WriteString(") ")
	b.WriteString(retType)
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
		left, lt, err := g.translateExpr(n.Left, ctx, "")
		if err != nil {
			return "", "", err
		}
		right, rt, err := g.translateExpr(n.Right, ctx, "")
		if err != nil {
			return "", "", err
		}
		switch n.Op {
		case "+", "*", "==", "!=":
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
	case *FuncLitExpr:
		return g.translateFuncLit(n, ctx)
	case *SwitchExpr:
		return g.translateSwitch(n, ctx, expected)
	}
	return "", "", fmt.Errorf("unsupported expression %#v", e)
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

func (g *generator) translateSwitch(n *SwitchExpr, ctx *exprCtx, expected string) (string, string, error) {
	targetCode, targetType, err := g.translateExpr(n.Target, ctx, "")
	if err != nil {
		return "", "", err
	}
	enumName, enumArgs := splitTypeArgs(targetType)
	enumDecl := g.pkg.Enums[enumName]
	if enumDecl == nil {
		return "", "", fmt.Errorf("switch target %q is not an enum", targetType)
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
			return "", nil, fmt.Errorf("unknown variant %s of %s", pat.Name, enum.Name)
		}
		tname := variantGoTypeName(enum.Name, variant.Name)
		if len(enumArgs) > 0 {
			tname += "[" + strings.Join(enumArgs, ", ") + "]"
		}
		bindings := map[string]bindingInfo{}
		for i, arg := range pat.Args {
			if i >= len(variant.Fields) {
				return "", nil, fmt.Errorf("pattern %s arity mismatch", pat.Name)
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
		return "", nil, fmt.Errorf("unsupported pattern %#v", p)
	}
}

func (g *generator) translateCall(n *CallExpr, ctx *exprCtx, expected string) (string, string, error) {
	if id, ok := n.Callee.(*IdentExpr); ok {
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
			return fmt.Sprintf("callAny(%s%s)", id.Name, func() string {
				if len(args) == 0 {
					return ""
				}
				return ", " + strings.Join(args, ", ")
			}()), "any", nil
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
			retType := g.goReturnType(fn.Ret, ctx.typeParams)
			if len(subst) > 0 {
				retType = typeStringReturn(fn.Ret, subst)
			}
			return fmt.Sprintf("%s(%s)", callee, strings.Join(args, ", ")), retType, nil
		}
		if enumName, ok := g.variantByName[id.Name]; ok {
			return g.translateEnumConstructor(enumName, id.Name, n.Args, ctx, expected)
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
			return fmt.Sprintf("callAny(%s%s)", id.Name, func() string {
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
	return fmt.Sprintf("%s.(%s)(%s)", name, sourceType, strings.Join(argCodes, ", ")), ret, true, nil
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

func (g *generator) translateTypeclassCall(name string, args []Expr, ctx *exprCtx, expected string) (string, string, bool) {
	if ifaceName, ok := g.interfaceByMethod[name]; ok {
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
		_, firstType, err := g.translateExpr(args[0], ctx, "")
		if err != nil {
			return "", "", false
		}
		helper := helperFuncName(name, typeKeyFromType(firstType))
		if g.hasHelper(name, firstType) {
			var argCodes []string
			for _, a := range args {
				code, _, err := g.translateExpr(a, ctx, "")
				if err != nil {
					return "", "", false
				}
				argCodes = append(argCodes, code)
			}
			return fmt.Sprintf("%s(%s)", helper, strings.Join(argCodes, ", ")), methodReturnType(methodIface, name), true
		}
		_ = methodIface
	}
	return "", "", false
}

func (g *generator) hasHelper(method, typ string) bool {
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
	if _, ok := g.interfaceByMethod[name]; ok {
		if funcName, ok := ctx.constraintFuncs[name]; ok {
			return funcName, expected, true
		}
	}
	return "", "", false
}

func (g *generator) lookupFieldType(baseType, field string) string {
	base, _ := splitTypeArgs(baseType)
	st := g.pkg.Structs[base]
	if st == nil {
		return ""
	}
	for _, f := range st.Fields {
		if f.Name == field {
			return g.goType(f.Type, nil)
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
		case "String":
			return "string"
		case "Bool":
			return "bool"
		case "Unit":
			return "struct{}"
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
		locals:          map[string]string{},
		bindings:        map[string]string{},
		sourceTypes:     map[string]string{},
		typeParams:      map[string]struct{}{},
		constraintFuncs: map[string]string{},
		retType:         ctx.retType,
		currentImpl:     ctx.currentImpl,
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
	for k := range ctx.typeParams {
		dup.typeParams[k] = struct{}{}
	}
	for k, v := range ctx.constraintFuncs {
		dup.constraintFuncs[k] = v
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
		case "String":
			return "string"
		case "Bool":
			return "bool"
		case "Unit":
			return "struct{}"
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
