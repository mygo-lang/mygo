package compiler

import (
	"go/types"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
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
