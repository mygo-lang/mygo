package compiler

import (
	"go/types"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference"
)

type Package struct {
	Name          string
	NoPrelude     bool // if true, skip auto-importing prelude declarations
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

type goPackageSigs struct {
	funcs   map[string]*goFuncSig
	methods map[string]map[string]*goFuncSig
	pkg     *types.Package
}

type goFuncSig struct {
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
	currentImpl      string
	implTypeKey      string
	implTypeParams   []string
}

type typeclassBinding struct {
	Interface  string
	Score      matchScore
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

type importSpec struct {
	Alias string
	Path  string
}

type generator struct {
	pkg               *Package
	importAliases     map[string]string
	interfaceByMethod map[string]string
	variantByName     map[string]string
	goSigCache        map[string]*goPackageSigs
	needsCallAny      bool
	localSeq          int
	typedInfo         *typeinference.TypedInfo
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
		dup.typeclassMethods[k] = append([]typeclassBinding(nil), v...)
	}
	dup.dictBindings = append([]dictBinding(nil), ctx.dictBindings...)
	return dup
}
