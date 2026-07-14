package codegen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strconv"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/codegen/goast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference"
)

// Generate generates all Go source for the package as a single string.
func Generate(p *Package) (string, error) {
	files, err := GenerateFiles(p)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", nil
	}
	var buf bytes.Buffer
	for name, src := range files {
		fmt.Fprintf(&buf, "// === %s ===\n\n", name)
		buf.WriteString(src)
		buf.WriteString("\n\n")
	}
	return buf.String(), nil
}

// GenerateFiles generates Go source for all .mygo files in a package.
func GenerateFiles(p *Package) (map[string]string, error) {
	// Build SourceFiles mapping for error messages.
	sourceFiles := make(map[any]string)
	for _, decl := range p.Decls {
		sourceFiles[decl] = sourceFileOf(decl)
	}

	pkgInfo := &typeinference.PkgInfo{
		Dir:           p.Dir,
		WorkspaceRoot: p.WorkspaceRoot,
		Name:          p.Name,
		Decls:         p.Decls,
		Enums:         p.Enums,
		Structs:       p.Structs,
		Interfaces:    p.Interfaces,
		Funcs:         p.Funcs,
		Impls:         p.Impls,
	}
	infState := typeinference.NewInferState()
	typedInfo, err := typeinference.InferPackage(pkgInfo, infState)
	if err != nil {
		return nil, err
	}

	g := newGen(p, typedInfo)

	files := make(map[string][]Decl)
	for _, decl := range p.Decls {
		files[sourceFileOf(decl)] = append(files[sourceFileOf(decl)], decl)
	}

	result := make(map[string]string)
	hktEmitted := false

	sortedSourceFiles := make([]string, 0, len(files))
	for name := range files {
		sortedSourceFiles = append(sortedSourceFiles, name)
	}
	sort.Strings(sortedSourceFiles)

	// Prelude declarations.
	if preludeDecls, ok := files[""]; ok {
		g.currentFile = "prelude"
		sf := goast.NewSourceFile(p.Name)
		if !hktEmitted && declsHaveInterface(preludeDecls) {
			g.genHKTDecls(sf)
			hktEmitted = true
		}
		addGoastImport(sf, p)
		for _, decl := range preludeDecls {
			g.genDecl(sf, decl)
		}
		for _, decl := range preludeDecls {
			if impl, ok := decl.(*ImplDecl); ok {
				ds := g.genImplDecls(impl)
				sf.AddDecls(ds)
			}
		}
		for _, decl := range preludeDecls {
			if fn, ok := decl.(*FuncDecl); ok {
				fd, ferr := g.genFuncDecl(fn)
				if ferr != nil {
					return nil, common.ErrorAtPos(g.currentFile, fn.Line, fn.Column, "function %s: %v", fn.Name, ferr)
				}
				sf.AddDecl(fd)
			}
		}
		if g.needsCallAny {
			sf.AddDecls(g.genHelperDecls())
		}
		out, _ := sf.Render()
		result["zz_prelude.gen.go"] = out
		delete(files, "")
	}

	// Regenerate source file list.
	sortedSourceFiles = make([]string, 0, len(files))
	for name := range files {
		sortedSourceFiles = append(sortedSourceFiles, name)
	}
	sort.Strings(sortedSourceFiles)

	if len(sortedSourceFiles) == 0 && len(p.Decls) > 0 && len(result) == 0 {
		sortedSourceFiles = []string{"__fallback__"}
		files["__fallback__"] = p.Decls
	} else {
		allEmpty := true
		for _, name := range sortedSourceFiles {
			if !skipSourceFile(name) {
				allEmpty = false
				break
			}
		}
		if allEmpty && len(p.Decls) > 0 && len(result) == 0 {
			sortedSourceFiles = []string{"__fallback__"}
			files["__fallback__"] = p.Decls
		}
	}

	for i, sourceFile := range sortedSourceFiles {
		if skipSourceFile(sourceFile) {
			continue
		}
		g.currentFile = sourceFile
		decls := files[sourceFile]
		sf := goast.NewSourceFile(p.Name)
		if !hktEmitted && declsHaveInterface(decls) && (p.NoPrelude || p.Name == "prelude") {
			g.genHKTDecls(sf)
			hktEmitted = true
		}
		addGoastImport(sf, p)
		for _, decl := range decls {
			g.genDecl(sf, decl)
		}
		for _, decl := range decls {
			if impl, ok := decl.(*ImplDecl); ok {
				ds := g.genImplDecls(impl)
				sf.AddDecls(ds)
			}
		}
		for _, decl := range decls {
			if fn, ok := decl.(*FuncDecl); ok {
				fd, ferr := g.genFuncDecl(fn)
				if ferr != nil {
					return nil, common.ErrorAtPos(g.currentFile, fn.Line, fn.Column, "function %s: %v", fn.Name, ferr)
				}
				sf.AddDecl(fd)
			}
		}
		for _, decl := range decls {
			if s, ok := decl.(*LetStmt); ok {
				ctx := &egCtx{
					locals:      map[string]string{},
					bindings:    map[string]string{},
					sourceTypes: map[string]string{},
					mutable:     map[string]bool{},
					typeParams:  map[string]struct{}{},
				}
				code, _, _ := g.translateExpr(s.Value, ctx, g.goType(s.Type, nil))
				actual := sanitizeIdent(s.Name)
				if actual == "" || actual == "_" {
					actual = "tmp"
				}
				decl := &ast.GenDecl{
					Tok: token.VAR,
					Specs: []ast.Spec{
						&ast.ValueSpec{
							Names:  []*ast.Ident{ast.NewIdent(actual)},
							Values: []ast.Expr{code},
						},
					},
				}
				sf.AddDecl(decl)
			}
		}
		if g.needsCallAny && i == len(sortedSourceFiles)-1 {
			sf.AddDecls(g.genHelperDecls())
		}
		out, err := sf.Render()
		if err != nil {
			return nil, err
		}
		result[sourceToGenName(sourceFile)] = out
	}
	return result, nil
}

func addGoastImport(sf *goast.SourceFile, p *Package) {
	imports := sortedImports(p)
	if p.Name != "prelude" && !p.NoPrelude {
		imports = append(imports, ImportSpec{Path: "github.com/mygo-lang/mygo/prelude", Alias: "."})
	}
	if hasImportPath(imports, "reflect") {
		imports = append(imports, ImportSpec{Path: "reflect"})
	}
	sort.Slice(imports, func(i, j int) bool {
		if imports[i].Alias == imports[j].Alias {
			return imports[i].Path < imports[j].Path
		}
		return imports[i].Alias < imports[j].Alias
	})
	for _, imp := range imports {
		path := importPathForGo(imp.Path)
		alias := imp.Alias
		if alias == importAliasForPath(path) {
			alias = ""
		}
		sf.AddImport(path, alias)
	}
}

func hasImportPath(imports []ImportSpec, path string) bool {
	for _, imp := range imports {
		if importPathForGo(imp.Path) == path {
			return true
		}
	}
	return false
}

func sortedImports(p *Package) []ImportSpec {
	imports := make([]ImportSpec, 0, len(p.ImportDecls))
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
		imports = append(imports, ImportSpec{Alias: alias, Path: imp.Path})
	}
	for path := range p.Imports {
		rawPath := importPathForGo(path)
		if _, ok := seenPaths[rawPath]; ok {
			continue
		}
		imports = append(imports, ImportSpec{Path: path})
	}
	sort.Slice(imports, func(i, j int) bool {
		if imports[i].Alias == imports[j].Alias {
			return imports[i].Path < imports[j].Path
		}
		return imports[i].Alias < imports[j].Alias
	})
	return imports
}

// genCtx is the expression/statement translation context.
type egCtx struct {
	locals      map[string]string
	bindings    map[string]string
	sourceTypes map[string]string
	mutable     map[string]bool
	typeParams  map[string]struct{}

	constraintFuncs  map[string]string
	typeclassMethods map[string][]egTcBinding
	retType          string
	retTypes         []string
	currentImpl      string
	implTypeKey      string
	implTypeParams   []string
}

type egTcBinding struct {
	Interface  string
	TargetType string
	ParamTypes []string
	RetType    string
	DictExpr   string
}

func (ctx *egCtx) constraintFuncForMethod(name string) (string, bool) {
	if ctx == nil {
		return "", false
	}
	fn, ok := ctx.constraintFuncs[name]
	return fn, ok
}

func (ctx *egCtx) child() *egCtx {
	dup := &egCtx{
		locals:           map[string]string{},
		bindings:         map[string]string{},
		sourceTypes:      map[string]string{},
		mutable:          map[string]bool{},
		typeParams:       map[string]struct{}{},
		constraintFuncs:  map[string]string{},
		typeclassMethods: map[string][]egTcBinding{},
		retType:          ctx.retType,
		retTypes:         append([]string(nil), ctx.retTypes...),
		currentImpl:      ctx.currentImpl,
		implTypeKey:      ctx.implTypeKey,
		implTypeParams:   ctx.implTypeParams,
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
		dup.typeclassMethods[k] = append([]egTcBinding(nil), v...)
	}
	return dup
}

// ============================================================
// gen (replaces Generator)
// ============================================================

type gen struct {
	pkg               *Package
	importAliases     map[string]string
	interfaceByMethod map[string]string
	inherentMethods   map[string]map[string]*struct {
		Impl        *ImplDecl
		Func        *FuncDecl
		HasReceiver bool
	}
	variantByName map[string]string
	needsCallAny  bool
	localSeq      int
	switchVarSeq  int
	typedInfo     *typeinference.TypedInfo
	currentFile   string
}

func newGen(p *Package, typedInfo *typeinference.TypedInfo) *gen {
	g := &gen{
		pkg:               p,
		importAliases:     p.ImportAliases,
		interfaceByMethod: map[string]string{},
		inherentMethods: map[string]map[string]*struct {
			Impl        *ImplDecl
			Func        *FuncDecl
			HasReceiver bool
		}{},
		variantByName: map[string]string{},
		typedInfo:     typedInfo,
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
	for _, impl := range p.Impls {
		if impl.InterfaceName != "" || impl.Name != "" {
			continue
		}
		recvName := inherentReceiverName(impl.Type)
		if recvName == "" {
			continue
		}
		if g.inherentMethods[recvName] == nil {
			g.inherentMethods[recvName] = map[string]*struct {
				Impl        *ImplDecl
				Func        *FuncDecl
				HasReceiver bool
			}{}
		}
		for _, m := range impl.Methods {
			hasRecv := len(m.Params) > 0 && isInherentReceiverParam(m.Params[0].Type, impl.Type, recvName)
			g.inherentMethods[recvName][m.Name] = &struct {
				Impl        *ImplDecl
				Func        *FuncDecl
				HasReceiver bool
			}{Impl: impl, Func: m, HasReceiver: hasRecv}
		}
	}
	return g
}

func (g *gen) inferredType(e Expr) string {
	if g == nil || g.typedInfo == nil || e == nil {
		return ""
	}
	if mt, ok := g.typedInfo.ExprTypes[e]; ok && mt != nil {
		return mygoSigTypeToGo(mt.String())
	}
	return ""
}

// genDecl adds declarations for enum/struct/interface to the source file.
func (g *gen) genDecl(sf *goast.SourceFile, decl Decl) {
	switch d := decl.(type) {
	case *EnumDecl:
		g.genEnumDecl(sf, d)
	case *StructDecl:
		g.genStructDecl(sf, d)
	case *InterfaceDecl:
		g.genInterfaceDecl(sf, d)
	}
}

func (g *gen) genHKTDecls(sf *goast.SourceFile) {
	needsHKT := false
	for _, iface := range g.pkg.Interfaces {
		if len(g.hktParams(iface)) > 0 {
			needsHKT = true
			break
		}
	}
	if !needsHKT {
		return
	}
	// Use ast.NewIdent("interface{}") to get compact rendering without spaces
	emptyIface := ast.NewIdent("interface{}")
	sf.AddDecl(astTypeDecl("HKTType", nil, emptyIface))
	sf.AddDecl(astTypeDecl("HKT1", typeParamFields([]string{"F"}), emptyIface))
	sf.AddDecl(astTypeDecl("HKT2", typeParamFields([]string{"A"}), emptyIface))
	sf.AddDecl(astTypeDecl("HKT", typeParamFields([]string{"F", "A"}), emptyIface))
}

func (g *gen) hktParams(iface *InterfaceDecl) map[string]struct{} {
	set := make(map[string]struct{})
	validParams := typeParamSet(iface.TypeParams)
	for _, m := range iface.Methods {
		for _, p := range m.Params {
			collectHKTNames(p.Type, set, validParams, nil)
		}
		collectHKTNames(m.Ret, set, validParams, nil)
	}
	return set
}

func collectHKTNames(t TypeExpr, set map[string]struct{}, valid map[string]struct{}, _ interface{}) {
	switch tt := t.(type) {
	case *NamedType:
		if valid != nil && len(tt.Args) > 0 {
			// Check if tt.Name is an HKT type parameter constructor name.
			// valid may contain plain names (old format) or HKT names like "C[A]" (new format).
			isHKT := false
			if _, ok := valid[tt.Name]; ok {
				isHKT = true
			} else {
				// Check if any valid param starts with "tt.Name["
				for vp := range valid {
					if strings.HasPrefix(vp, tt.Name+"[") {
						isHKT = true
						break
					}
				}
			}
			if isHKT {
				set[tt.Name] = struct{}{}
			}
		}
		for _, a := range tt.Args {
			collectHKTNames(a, set, valid, nil)
		}
	case *FuncType:
		for _, p := range tt.Params {
			collectHKTNames(p, set, valid, nil)
		}
		collectHKTNames(tt.Ret, set, valid, nil)
	}
}

func (g *gen) genEnumDecl(sf *goast.SourceFile, d *EnumDecl) {
	// type Name interface { isName() }
	methods := []*ast.Field{{
		Names: []*ast.Ident{ast.NewIdent("is" + d.Name)},
		Type:  &ast.FuncType{Params: &ast.FieldList{}},
	}}
	ifaceType := &ast.InterfaceType{Methods: &ast.FieldList{List: methods}}
	sf.AddDecl(astTypeDecl(d.Name, typeParamFields(d.TypeParams), ifaceType))

	for _, v := range d.Variants {
		tname := variantGoTypeName(d.Name, v.Name)
		fields := make([]*ast.Field, len(v.Fields))
		for i, f := range v.Fields {
			fields[i] = &ast.Field{
				Names: []*ast.Ident{ast.NewIdent("F" + strconv.Itoa(i))},
				Type:  goastTypeExpr(f.Type),
			}
		}
		// Variant struct
		sf.AddDecl(astTypeDecl(tname, typeParamFields(d.TypeParams),
			&ast.StructType{Fields: &ast.FieldList{List: fields}}))

		// receiver type
		var recvType ast.Expr = ast.NewIdent(tname)
		if len(d.TypeParams) > 0 {
			recvType = genericIdent(tname, typeArgIdents(d.TypeParams)...)
		}
		recv := &ast.FieldList{
			List: []*ast.Field{{Names: []*ast.Ident{ast.NewIdent("_")}, Type: recvType}},
		}
		// isName() method
		sf.AddDecl(astFuncDecl("is"+d.Name, recv, nil, nil, nil, &ast.BlockStmt{}))

		// Constructor
		ctorParams := make([]*ast.Field, len(v.Fields))
		for i, f := range v.Fields {
			ctorParams[i] = &ast.Field{
				Names: []*ast.Ident{ast.NewIdent("a" + strconv.Itoa(i))},
				Type:  goastTypeExpr(f.Type),
			}
		}
		var ctorRet ast.Expr = ast.NewIdent(d.Name)
		if len(d.TypeParams) > 0 {
			ctorRet = genericIdent(d.Name, typeArgIdents(d.TypeParams)...)
		}
		// Build return: VariantName{F0: a0, ...}
		elts := make([]ast.Expr, len(v.Fields))
		for i := range v.Fields {
			elts[i] = &ast.KeyValueExpr{
				Key:   ast.NewIdent("F" + strconv.Itoa(i)),
				Value: ast.NewIdent("a" + strconv.Itoa(i)),
			}
		}
		var structLit ast.Expr = ast.NewIdent(tname)
		if len(d.TypeParams) > 0 {
			structLit = genericIdent(tname, typeArgIdents(d.TypeParams)...)
		}
		body := &ast.BlockStmt{
			List: []ast.Stmt{&ast.ReturnStmt{
				Results: []ast.Expr{&ast.CompositeLit{Type: structLit, Elts: elts}},
			}},
		}
		sf.AddDecl(astFuncDecl(v.Name, nil, typeParamFields(d.TypeParams),
			ctorParams, []*ast.Field{{Type: ctorRet}}, body))
	}
}

func (g *gen) genStructDecl(sf *goast.SourceFile, d *StructDecl) {
	fields := make([]*ast.Field, len(d.Fields))
	for i, f := range d.Fields {
		if f.Name == "embed" {
			fields[i] = &ast.Field{Type: goastTypeExpr(f.Type)}
		} else {
			fields[i] = &ast.Field{Names: []*ast.Ident{ast.NewIdent(f.Name)}, Type: goastTypeExpr(f.Type)}
		}
		if f.Tag != "" {
			fields[i].Tag = &ast.BasicLit{Kind: token.STRING, Value: "`" + f.Tag + "`"}
		}
	}
	sf.AddDecl(astTypeDecl(d.Name, typeParamFields(d.TypeParams),
		&ast.StructType{Fields: &ast.FieldList{List: fields}}))
}

func (g *gen) genInterfaceDecl(sf *goast.SourceFile, d *InterfaceDecl) {
	// No-op: do not generate Go interface types.
	// Interfaces are only used for type inference and method dispatch at compile time.
}

// genImplDecls generates impl helper functions.
func (g *gen) genImplDecls(d *ImplDecl) []ast.Decl {
	if d.InterfaceName == "" && d.Name == "" {
		return g.genInherentDecls(d)
	}
	ifaceName := d.InterfaceName
	if ifaceName == "" {
		ifaceName = d.Name
	}
	return g.genTypedImpl(d, ifaceName)
}

func (g *gen) genTypedImpl(d *ImplDecl, ifaceName string) []ast.Decl {
	iface := g.pkg.Interfaces[ifaceName]
	if iface == nil {
		return nil
	}
	typeArgs := d.InterfaceArgs
	if len(typeArgs) == 0 {
		typeArgs = d.TypeArgs
	}
	if len(iface.TypeParams) != len(typeArgs) {
		return nil
	}
	subst := map[string]string{}
	for i, tp := range iface.TypeParams {
		subst[tp] = g.goType(typeArgs[i], nil)
	}
	typeKey := g.implHelperKey(d, typeArgs)
	methodBodies := map[string]*FuncDecl{}
	for _, m := range d.Methods {
		methodBodies[m.Name] = m
	}
	var decls []ast.Decl
	for _, sig := range iface.Methods {
		m := methodBodies[sig.Name]
		if m == nil {
			continue
		}
		combinedTP := typeParamSet(d.TypeParams)
		for tp := range typeParamSet(sig.TypeParams) {
			combinedTP[tp] = struct{}{}
		}
		retType := g.goReturnType(m.Ret, combinedTP)
		fnName := helperFuncName(sig.Name, typeKey)

		// Params
		// Build the parameter list with capacity only; using constraints may be skipped
		// when they cannot be resolved, and a pre-sized slice would leave nil fields
		// behind for go/printer to trip over.
		params := make([]*ast.Field, 0, len(m.Params)+len(m.Using))
		for _, p := range m.Params {
			params = append(params, &ast.Field{
				Names: []*ast.Ident{ast.NewIdent(sanitizeIdent(p.Name))},
				Type:  goastTypeExpr(p.Type),
			})
		}
		usingConstraintFuncs := map[string]string{}
		usingTypeclassMethods := map[string][]egTcBinding{}
		for _, cu := range m.Using {
			namedImpl, ifc, ok := resolveConstraint(cu, g.pkg)
			if !ok {
				continue
			}
			if namedImpl != nil && cu.BindName == "" && cu.Name == ifc.Name {
				namedImpl = nil
			}
			typeArgs := append([]TypeExpr(nil), cu.Args...)
			if namedImpl != nil {
				typeArgs = append([]TypeExpr(nil), namedImpl.InterfaceArgs...)
				if len(typeArgs) == 0 {
					typeArgs = append([]TypeExpr(nil), namedImpl.TypeArgs...)
				}
			}
			localSubst := map[string]string{}
			for i, tp := range ifc.TypeParams {
				if i < len(typeArgs) {
					localSubst[tp] = g.goType(typeArgs[i], combinedTP)
				}
			}
			concreteArgs := make(map[string]TypeExpr, len(ifc.TypeParams))
			for i, tp := range ifc.TypeParams {
				if i < len(typeArgs) {
					concreteArgs[tp] = typeArgs[i]
				}
			}
			namedImplTypeKey := ""
			if namedImpl != nil {
				namedImplTypeKey = g.implHelperKey(namedImpl, typeArgs)
			}
			for _, mm := range ifc.Methods {
				paramTypes := make([]string, 0, len(mm.Params))
				for _, p := range mm.Params {
					paramTypes = append(paramTypes, g.goTypeStringForTypeclass(p.Type, concreteArgs, localSubst))
				}
				retTypeStr := g.goTypeStringForTypeclass(mm.Ret, concreteArgs, localSubst)
				if namedImplTypeKey != "" {
					fnName := helperFuncName(mm.Name, namedImplTypeKey)
					usingConstraintFuncs[mm.Name] = fnName
					usingTypeclassMethods[mm.Name] = append(usingTypeclassMethods[mm.Name], egTcBinding{
						Interface:  cu.Name,
						TargetType: firstTypeArgString(typeArgs, localSubst),
						ParamTypes: paramTypes,
						RetType:    retTypeStr,
						DictExpr:   fnName,
					})
					continue
				}
				pt := typeclassFuncType(paramTypes, retTypeStr)
				params = append(params, &ast.Field{
					Names: []*ast.Ident{ast.NewIdent(mm.Name + "Fn")},
					Type:  ast.NewIdent(pt),
				})
				if _, ok := usingConstraintFuncs[mm.Name]; !ok {
					usingConstraintFuncs[mm.Name] = mm.Name + "Fn"
				}
				usingTypeclassMethods[mm.Name] = append(usingTypeclassMethods[mm.Name], egTcBinding{
					Interface:  cu.Name,
					TargetType: firstTypeArgString(typeArgs, localSubst),
					ParamTypes: paramTypes,
					RetType:    retTypeStr,
					DictExpr:   mm.Name + "Fn",
				})
			}
		}

		// Results
		var results []*ast.Field
		if retType != "" {
			results = []*ast.Field{{Type: ast.NewIdent(retType)}}
		}

		// Body
		var bodyStmts []ast.Stmt
		if m.Body == nil {
			if retType == "" {
				bodyStmts = append(bodyStmts, &ast.ReturnStmt{})
			} else {
				bodyStmts = append(bodyStmts, &ast.ExprStmt{X: &ast.CallExpr{Fun: ast.NewIdent("panic"), Args: []ast.Expr{astStringLit("unimplemented")}}})
			}
		} else {
			ctx := &egCtx{
				locals:           map[string]string{},
				bindings:         map[string]string{},
				sourceTypes:      map[string]string{},
				mutable:          map[string]bool{},
				typeParams:       combinedTP,
				constraintFuncs:  usingConstraintFuncs,
				typeclassMethods: usingTypeclassMethods,
				retType:          retType,
				currentImpl:      ifaceName,
				implTypeKey:      typeKey,
			}
			for _, p := range m.Params {
				ctx.locals[p.Name] = g.goType(p.Type, combinedTP)
				ctx.bindings[p.Name] = p.Name
			}
			if retType == "" {
				if goExpr, ok := m.Body.(*GoExpr); ok && g.isUnitGoExpr(goExpr, ctx) {
					if goStmts, err := g.translateGoUnitStmts(goExpr, ctx); err == nil {
						bodyStmts = append(bodyStmts, goStmts...)
						bodyStmts = append(bodyStmts, &ast.ReturnStmt{})
						constraints := mapKeyTypeParamConstraintsForImplMethod(d, m)
						decl := astFuncDecl(fnName, nil, typeParamFieldsWithConstraints(mergedTypeParams(d.TypeParams, sig.TypeParams), constraints),
							params, results, &ast.BlockStmt{List: bodyStmts})
						decls = append(decls, decl)
						continue
					}
				}
				code, _, _ := g.translateExpr(m.Body, ctx, retType)
				bodyStmts = append(bodyStmts, &ast.ExprStmt{X: code})
				bodyStmts = append(bodyStmts, &ast.ReturnStmt{})
			} else {
				code, _, _ := g.translateExpr(m.Body, ctx, retType)
				bodyStmts = append(bodyStmts, &ast.ReturnStmt{Results: []ast.Expr{code}})
			}
		}
		constraints := mapKeyTypeParamConstraintsForImplMethod(d, m)
		decl := astFuncDecl(fnName, nil, typeParamFieldsWithConstraints(mergedTypeParams(d.TypeParams, sig.TypeParams), constraints),
			params, results, &ast.BlockStmt{List: bodyStmts})
		decls = append(decls, decl)
	}
	return decls
}

func mergedTypeParams(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, tp := range a {
		if !seen[tp] {
			out = append(out, tp)
			seen[tp] = true
		}
	}
	for _, tp := range b {
		if !seen[tp] {
			out = append(out, tp)
			seen[tp] = true
		}
	}
	return out
}

func mapKeyTypeParamConstraintsForImplMethod(d *ImplDecl, m *FuncDecl) map[string]string {
	constraints := mergeMapKeyTypeParamConstraints(nil, d.Type)
	for _, arg := range d.TypeArgs {
		constraints = mergeMapKeyTypeParamConstraints(constraints, arg)
	}
	for _, arg := range d.InterfaceArgs {
		constraints = mergeMapKeyTypeParamConstraints(constraints, arg)
	}
	for _, p := range m.Params {
		constraints = mergeMapKeyTypeParamConstraints(constraints, p.Type)
	}
	constraints = mergeMapKeyTypeParamConstraints(constraints, m.Ret)
	return constraints
}

func (g *gen) genInherentDecls(d *ImplDecl) []ast.Decl {
	recvName := inherentReceiverName(d.Type)
	if recvName == "" {
		return nil
	}
	var decls []ast.Decl
	for _, m := range d.Methods {
		hasRecv := len(m.Params) > 0 && isInherentReceiverParam(m.Params[0].Type, d.Type, recvName)
		mtp := mergedTypeParams(d.TypeParams, m.TypeParams)
		tpSet := typeParamSet(mtp)
		retTypes := g.goReturnTypes(m.Ret, tpSet)
		retType := ""
		if len(retTypes) == 1 {
			retType = retTypes[0]
		}
		ctx := &egCtx{
			locals:      map[string]string{},
			bindings:    map[string]string{},
			sourceTypes: map[string]string{},
			mutable:     map[string]bool{},
			typeParams:  tpSet,
			retType:     retType,
			retTypes:    retTypes,
		}
		for i, p := range m.Params {
			if i == 0 && hasRecv {
				ctx.locals[p.Name] = g.goType(p.Type, tpSet)
				ctx.bindings[p.Name] = p.Name
				continue
			}
			gt := g.goType(p.Type, tpSet)
			ctx.locals[p.Name] = gt
			ctx.bindings[p.Name] = p.Name
		}

		fnName := inherentMethodName(recvName, m.Name)

		// Params
		var params []*ast.Field
		for i, p := range m.Params {
			if i == 0 && hasRecv {
				params = append(params, &ast.Field{
					Names: []*ast.Ident{ast.NewIdent(sanitizeIdent(p.Name))},
					Type:  goastTypeExpr(p.Type),
				})
				continue
			}
			params = append(params, &ast.Field{
				Names: []*ast.Ident{ast.NewIdent(sanitizeIdent(p.Name))},
				Type:  goastTypeExpr(p.Type),
			})
		}

		// Results
		var results []*ast.Field
		if len(retTypes) == 1 {
			results = []*ast.Field{{Type: ast.NewIdent(retTypes[0])}}
		} else if len(retTypes) > 1 {
			for _, rt := range retTypes {
				results = append(results, &ast.Field{Type: ast.NewIdent(rt)})
			}
		}

		// Body
		var bodyStmts []ast.Stmt
		if block, ok := m.Body.(*BlockExpr); ok {
			blockStmts, _ := g.translateBlockStmts(block, ctx, retType, retTypes)
			bodyStmts = append(bodyStmts, blockStmts...)
		} else if len(retTypes) == 0 {
			if goExpr, ok := m.Body.(*GoExpr); ok && g.isUnitGoExpr(goExpr, ctx) {
				if goStmts, err := g.translateGoUnitStmts(goExpr, ctx); err == nil {
					bodyStmts = append(bodyStmts, goStmts...)
					bodyStmts = append(bodyStmts, &ast.ReturnStmt{})
					decls = append(decls, astFuncDecl(fnName, nil, typeParamFields(mtp),
						params, results, &ast.BlockStmt{List: bodyStmts}))
					continue
				}
			}
			code, _, _ := g.translateExpr(m.Body, ctx, retType)
			bodyStmts = append(bodyStmts, &ast.ExprStmt{X: code})
			bodyStmts = append(bodyStmts, &ast.ReturnStmt{})
		} else {
			code, _, _ := g.translateExpr(m.Body, ctx, retType)
			bodyStmts = append(bodyStmts, &ast.ReturnStmt{Results: []ast.Expr{code}})
		}

		decls = append(decls, astFuncDecl(fnName, nil, typeParamFields(mtp),
			params, results, &ast.BlockStmt{List: bodyStmts}))
	}
	return decls
}

func resolveConstraint(c Constraint, p *Package) (*ImplDecl, *InterfaceDecl, bool) {
	// First try exact interface name match.
	if iface := p.Interfaces[c.Name]; iface != nil {
		// Find the first matching impl.
		for _, impl := range p.Impls {
			iname := impl.InterfaceName
			if iname == "" {
				iname = impl.Name
			}
			if iname == c.Name && len(impl.InterfaceArgs) == len(c.Args) {
				return impl, iface, true
			}
		}
		return nil, iface, true
	}
	// Try matching by impl type name (e.g., "using SliceIEnumerable[Int]").
	for _, impl := range p.Impls {
		implName := ""
		if impl.Type != nil {
			if nt, ok := impl.Type.(*NamedType); ok {
				implName = nt.Name
			}
		}
		if implName == "" {
			implName = impl.Name
		}
		if implName == "" {
			implName = impl.InterfaceName
		}
		if implName != c.Name {
			continue
		}
		ifaceName := impl.InterfaceName
		if ifaceName == "" {
			ifaceName = impl.Name
		}
		iface := p.Interfaces[ifaceName]
		if iface != nil {
			return impl, iface, true
		}
	}
	return nil, nil, false
}

func (g *gen) goType(t TypeExpr, tp map[string]struct{}) string {
	return goTypeInner(t, tp, g.pkg)
}

func (g *gen) goReturnType(t TypeExpr, tp map[string]struct{}) string {
	if isUnitType(t) {
		return ""
	}
	return goTypeInner(t, tp, g.pkg)
}

func (g *gen) goReturnTypes(t TypeExpr, tp map[string]struct{}) []string {
	if tt, ok := t.(*TupleType); ok {
		out := make([]string, len(tt.Elems))
		for i, e := range tt.Elems {
			out[i] = goTypeInner(e, tp, g.pkg)
		}
		return out
	}
	if rt := g.goReturnType(t, tp); rt != "" {
		return []string{rt}
	}
	return nil
}

func goTypeInner(t TypeExpr, tp map[string]struct{}, p *Package) string {
	switch tt := t.(type) {
	case *NamedType:
		if tp != nil {
			if _, ok := tp[tt.Name]; ok && len(tt.Args) == 0 {
				return tt.Name
			}
		}
		switch tt.Name {
		case "Int":
			return "int"
		case "Int8":
			return "int8"
		case "Int16":
			return "int16"
		case "Int32":
			return "int32"
		case "Int64":
			return "int64"
		case "UInt":
			return "uint"
		case "UInt8":
			return "uint8"
		case "UInt16":
			return "uint16"
		case "UInt32":
			return "uint32"
		case "UInt64":
			return "uint64"
		case "Byte":
			return "byte"
		case "Rune":
			return "rune"
		case "Float32":
			return "float32"
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
				return "*" + goTypeInner(tt.Args[0], tp, p)
			}
		case "Slice":
			if len(tt.Args) == 1 {
				return "[]" + goTypeInner(tt.Args[0], tp, p)
			}
		case "Map":
			if len(tt.Args) == 2 {
				return "map[" + goTypeInner(tt.Args[0], tp, p) + "]" + goTypeInner(tt.Args[1], tp, p)
			}
		case "Set":
			if len(tt.Args) == 1 {
				return "map[" + goTypeInner(tt.Args[0], tp, p) + "]struct{}"
			}
		case "Chan":
			if len(tt.Args) == 1 {
				return "chan " + goTypeInner(tt.Args[0], tp, p)
			}
		case "SendChan":
			if len(tt.Args) == 1 {
				return "chan<- " + goTypeInner(tt.Args[0], tp, p)
			}
		case "RecvChan":
			if len(tt.Args) == 1 {
				return "<-chan " + goTypeInner(tt.Args[0], tp, p)
			}
		}
		if len(tt.Args) == 0 {
			return tt.Name
		}
		args := make([]string, len(tt.Args))
		for i, a := range tt.Args {
			args[i] = goTypeInner(a, tp, p)
		}
		return tt.Name + "[" + strings.Join(args, ", ") + "]"
	case *FuncType:
		params := make([]string, len(tt.Params))
		for i, p := range tt.Params {
			params[i] = goTypeInner(p, tp, nil)
		}
		ret := goTypeInner(tt.Ret, tp, nil)
		if ret == "" || ret == "struct{}" {
			return "func(" + strings.Join(params, ", ") + ")"
		}
		return "func(" + strings.Join(params, ", ") + ") " + ret
	case *TupleType:
		if len(tt.Elems) == 0 {
			return "struct{}"
		}
		parts := make([]string, 0, len(tt.Elems))
		for i, e := range tt.Elems {
			parts = append(parts, "F"+strconv.Itoa(i)+" "+goTypeInner(e, tp, p))
		}
		return "struct { " + strings.Join(parts, "; ") + " }"
	default:
		return "any"
	}
}

func (g *gen) genHelperDecls() []ast.Decl {
	// Generate callAny helper
	callAnyBody := &ast.BlockStmt{List: []ast.Stmt{
		&ast.AssignStmt{
			Lhs: []ast.Expr{ast.NewIdent("values")},
			Rhs: []ast.Expr{astCall(ast.NewIdent("make"), ast.NewIdent("[]reflect.Value"), astCall(ast.NewIdent("len"), ast.NewIdent("args")))},
			Tok: token.DEFINE,
		},
		&ast.ForStmt{
			Body: &ast.BlockStmt{List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent("values"), ast.NewIdent("_")},
					Rhs: []ast.Expr{ast.NewIdent("i"), ast.NewIdent("arg")},
					Tok: token.DEFINE,
				},
			}},
		},
	}}
	fn := astFuncDecl("callAny", nil, nil,
		[]*ast.Field{
			{Names: []*ast.Ident{ast.NewIdent("fn")}, Type: ast.NewIdent("any")},
			{Names: []*ast.Ident{ast.NewIdent("args")}, Type: &ast.ArrayType{Elt: ast.NewIdent("any")}},
		},
		[]*ast.Field{{Type: ast.NewIdent("any")}},
		callAnyBody,
	)
	return []ast.Decl{fn}
}

// variantNameForEnum constructs the variant type name for an enum.
func variantNameForEnum(enumName, variantName string) string {
	if enumName == "" {
		return variantName
	}
	return enumName + variantName
}

// ============================================================
// Utility functions needed by the codegen
// ============================================================

func inherentReceiverName(t TypeExpr) string {
	if nt, ok := t.(*NamedType); ok {
		return nt.Name
	}
	return ""
}

// splitTypeArgs splits a type string into base name and type arguments.
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

func isInherentReceiverParam(paramType TypeExpr, implType TypeExpr, receiverName string) bool {
	if paramType == nil || implType == nil {
		return false
	}
	return typeString(paramType, nil) == typeString(implType, nil)
}

func inherentMethodName(receiverName, methodName string) string {
	return sanitizeIdent(receiverName) + "_" + sanitizeIdent(methodName)
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

func typeString(t TypeExpr, subst map[string]string) string {
	return goTypeString(t, subst)
}

func typeStringReturn(t TypeExpr, subst map[string]string) string {
	if isUnitType(t) {
		return ""
	}
	return goTypeString(t, subst)
}

func goTypeString(t TypeExpr, subst map[string]string) string {
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
		case "Int8":
			return "int8"
		case "Int16":
			return "int16"
		case "Int32":
			return "int32"
		case "Int64":
			return "int64"
		case "UInt":
			return "uint"
		case "UInt8":
			return "uint8"
		case "UInt16":
			return "uint16"
		case "UInt32":
			return "uint32"
		case "UInt64":
			return "uint64"
		case "Byte":
			return "byte"
		case "Rune":
			return "rune"
		case "Float32":
			return "float32"
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
				return "*" + goTypeString(tt.Args[0], subst)
			}
		case "Slice":
			if len(tt.Args) == 1 {
				return "[]" + goTypeString(tt.Args[0], subst)
			}
		case "Map":
			if len(tt.Args) == 2 {
				return "map[" + goTypeString(tt.Args[0], subst) + "]" + goTypeString(tt.Args[1], subst)
			}
		case "Set":
			if len(tt.Args) == 1 {
				return "map[" + goTypeString(tt.Args[0], subst) + "]struct{}"
			}
		}
		if len(tt.Args) == 0 {
			return tt.Name
		}
		args := make([]string, len(tt.Args))
		for i, a := range tt.Args {
			args[i] = goTypeString(a, subst)
		}
		return tt.Name + "[" + strings.Join(args, ", ") + "]"
	case *FuncType:
		params := make([]string, len(tt.Params))
		for i, p := range tt.Params {
			params[i] = goTypeString(p, subst)
		}
		ret := goTypeString(tt.Ret, subst)
		if ret == "" {
			return "func(" + strings.Join(params, ", ") + ")"
		}
		return "func(" + strings.Join(params, ", ") + ") " + ret
	default:
		return "any"
	}
}

func variantGoTypeName(enumName, variant string) string {
	return enumName + variant
}

func typeParamSet(params []string) map[string]struct{} {
	m := make(map[string]struct{}, len(params))
	for _, p := range params {
		m[p] = struct{}{}
	}
	return m
}

func importAliasForPath(path string) string {
	path = importPathForGo(path)
	if path == "" {
		return ""
	}
	path = strings.TrimSuffix(path, "/")
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return toPackageName(path[idx+1:])
	}
	return toPackageName(path)
}

func importPathForGo(path string) string {
	return strings.TrimPrefix(path, "go:")
}

func gopathForGo(path string) string {
	return importPathForGo(path)
}

func sanitizeIdent(s string) string {
	var b strings.Builder
	for i, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			if i == 0 && r >= '0' && r <= '9' {
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
	result := b.String()
	if isGoKeyword(result) {
		result += "_"
	}
	return result
}

func isGoKeyword(s string) bool {
	switch s {
	case "break", "case", "chan", "const", "continue", "default", "defer",
		"else", "fallthrough", "for", "func", "go", "goto", "if", "import",
		"interface", "map", "package", "range", "return", "select", "struct",
		"switch", "type", "var":
		return true
	}
	return false
}

func exportName(name string) string {
	if name == "" {
		return name
	}
	r := []rune(name)
	r[0] = toUpper(r[0])
	return string(r)
}

func toUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}
