package codegen

import (
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	myparser "github.com/mygo-lang/mygo/internal/mygo/parser"
	"github.com/mygo-lang/mygo/internal/mygo/pkg"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference"
)

// Type aliases for shared types from the pkg package.
// This keeps existing code in the codegen package working without mass renames.
type Package = pkg.Package
type ImportSpec = pkg.ImportSpec

type GoPackageSigs struct {
	funcs   map[string]*GoFuncSig
	methods map[string]map[string]*GoFuncSig
	pkg     *types.Package
}

type GoFuncSig struct {
	params []string
	ret    []string
}

type exprCtx struct {
	locals           map[string]string
	bindings         map[string]string
	sourceTypes      map[string]string
	mutable          map[string]bool
	typeParams       map[string]struct{}
	constraintFuncs  map[string]string
	typeclassMethods map[string][]typeclassBinding
	dictBindings     []dictBinding
	retType          string
	retTypes         []string
	currentImpl      string
	implSymbol       string
	implTypeKey      string
	implTypeParams   []string
}

type typeclassBinding struct {
	Interface  string
	Score      matchScore
	TargetType string
	ParamTypes []string
	RetType    string
	DictExpr   string
	DictType   string
}

type dictBinding struct {
	Interface string
	Args      []string
	Expr      string
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

type Generator struct {
	inner             *gen
	pkg               *pkg.Package
	importAliases     map[string]string
	interfaceByMethod map[string]string
	inherentMethods   map[string]map[string]*InherentMethod
	variantByName     map[string]string
	goSigCache        map[string]*GoPackageSigs
	needsCallAny      bool
	localSeq          int
	switchVarSeq      int
	typedInfo         *typeinference.TypedInfo
}

type InherentMethod struct {
	Impl        *ImplDecl
	Func        *FuncDecl
	HasReceiver bool
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
		dictBindings:     nil,
		retType:          ctx.retType,
		retTypes:         append([]string(nil), ctx.retTypes...),
		currentImpl:      ctx.currentImpl,
		implSymbol:       ctx.implSymbol,
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
		dup.typeclassMethods[k] = append([]typeclassBinding(nil), v...)
	}
	dup.dictBindings = append([]dictBinding(nil), ctx.dictBindings...)
	return dup
}

// NewGenerator creates a new Generator with all internal maps initialized.
func NewGenerator(p *Package, typedInfo *typeinference.TypedInfo) *Generator {
	ng := newGen(p, typedInfo)
	g := &Generator{
		inner:             ng,
		pkg:               p,
		importAliases:     ng.importAliases,
		interfaceByMethod: ng.interfaceByMethod,
		inherentMethods:   map[string]map[string]*InherentMethod{},
		variantByName:     ng.variantByName,
		goSigCache:        map[string]*GoPackageSigs{},
		typedInfo:         typedInfo,
	}
	for _, impl := range p.Impls {
		if impl.InterfaceName != "" {
			continue
		}
		receiverName := inherentReceiverName(impl.Type)
		if receiverName == "" {
			continue
		}
		if g.inherentMethods[receiverName] == nil {
			g.inherentMethods[receiverName] = map[string]*InherentMethod{}
		}
		for _, m := range impl.Methods {
			hasReceiver := len(m.Params) > 0 && isInherentReceiverParam(m.Params[0].Type, impl.Type, receiverName)
			g.inherentMethods[receiverName][m.Name] = &InherentMethod{Impl: impl, Func: m, HasReceiver: hasReceiver}
		}
	}
	return g
}

// simpleLoadPackage loads a .mygo package from a directory.
// Used by tests. This is a lightweight implementation.
func simpleLoadPackage(dir string, noPrelude bool) *Package {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	p := &Package{
		Dir:           dir,
		NoPrelude:     noPrelude,
		Imports:       map[string]struct{}{},
		ImportAliases: map[string]string{},
		Enums:         map[string]*EnumDecl{},
		Structs:       map[string]*StructDecl{},
		Interfaces:    map[string]*InterfaceDecl{},
		Funcs:         map[string]*FuncDecl{},
	}
	pkgName := ""
	var fileDecls []Decl
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".mygo") || strings.HasSuffix(name, ".gen.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		parsed, err := myparser.ParseFile(filepath.Join(dir, name), string(src))
		if err != nil {
			continue
		}
		if parsed.PackageName != "" {
			if pkgName == "" {
				pkgName = parsed.PackageName
			} else if pkgName != parsed.PackageName {
				continue
			}
		}
		fileDecls = append(fileDecls, parsed.Decls...)
	}
	if pkgName == "" {
		pkgName = filepath.Base(dir)
	}
	p.Name = toPackageName(pkgName)
	p.Decls = fileDecls
	for _, decl := range p.Decls {
		switch d := decl.(type) {
		case *ImportDecl:
			p.Imports[d.Path] = struct{}{}
			p.ImportDecls = append(p.ImportDecls, d)
			alias := d.Alias
			if alias == "" {
				alias = importAliasForPath(d.Path)
			}
			p.ImportAliases[alias] = d.Path
		case *EnumDecl:
			p.Enums[d.Name] = d
		case *StructDecl:
			p.Structs[d.Name] = d
		case *InterfaceDecl:
			p.Interfaces[d.Name] = d
		case *FuncDecl:
			p.Funcs[d.Name] = d
		case *ImplDecl:
			p.Impls = append(p.Impls, d)
		}
	}
	return p
}

// toPackageName sanitizes a string to be a valid Go package name.
// (Duplicate of the one in compiler/utils.go for independence.)
func toPackageName(name string) string {
	if name == "" {
		return "main"
	}
	return strings.ToLower(sanitizeIdent(name))
}

// toGen creates a *gen from this Generator for method delegation.
func (g *Generator) toGen() *gen {
	if g.inner == nil {
		g.inner = newGen(g.pkg, g.typedInfo)
	}
	ng := g.inner
	// Keep compatibility with tests that still set legacy fields directly.
	if len(g.interfaceByMethod) > 0 {
		ng.interfaceByMethod = g.interfaceByMethod
	}
	if len(g.variantByName) > 0 {
		ng.variantByName = g.variantByName
	}
	ng.needsCallAny = g.needsCallAny
	ng.localSeq = g.localSeq
	ng.switchVarSeq = g.switchVarSeq
	return ng
}

// translateSwitch delegates to gen.translateSwitch for test compatibility.
func (g *Generator) translateSwitch(n *SwitchExpr, ctx *exprCtx, expected string) (ast.Expr, string, error) {
	ng := g.toGen()
	// Convert exprCtx to egCtx
	eg := &egCtx{
		locals:      ctx.locals,
		bindings:    ctx.bindings,
		sourceTypes: ctx.sourceTypes,
		mutable:     ctx.mutable,
		typeParams:  ctx.typeParams,
		retType:     ctx.retType,
		retTypes:    ctx.retTypes,
	}
	result, err := ng.translateSwitch(n, eg, expected)
	if result.Expr == nil && err == nil {
		return ast.NewIdent("_"), result.Type, nil
	}
	return result.Expr, result.Type, err
}

// genImpl delegates to gen.genImplDecls for test compatibility.
func (g *Generator) genImpl(d *ImplDecl) ([]ast.Decl, error) {
	ng := g.toGen()
	return ng.genImplDecls(d), nil
}

// translateExpr delegates to gen.translateExpr for test compatibility.
func (g *Generator) translateExpr(e Expr, ctx *exprCtx, expected string) (ast.Expr, string, error) {
	ng := g.toGen()
	eg := &egCtx{
		locals:      ctx.locals,
		bindings:    ctx.bindings,
		sourceTypes: ctx.sourceTypes,
		mutable:     ctx.mutable,
		typeParams:  ctx.typeParams,
		retType:     ctx.retType,
		retTypes:    ctx.retTypes,
	}
	return ng.translateExpr(e, eg, expected)
}
