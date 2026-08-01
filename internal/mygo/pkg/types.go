package pkg

import (
	"github.com/mygo-lang/mygo/internal/mygo/ast"
)

// Package holds all parsed declarations and metadata for a single .mygo package.
type Package struct {
	Name           string
	Dir            string
	WorkspaceRoot  string
	NoPrelude      bool // if true, skip auto-importing prelude declarations
	Imports        map[string]struct{}
	ImportDecls    []*ast.ImportDecl
	ImportAliases  map[string]string
	Decls          []ast.Decl
	TypeAliases    map[string]*ast.TypeAliasDecl
	Types          map[string]*ast.TypeDecl
	Enums          map[string]*ast.EnumDecl
	Structs        map[string]*ast.StructDecl
	Interfaces     map[string]*ast.InterfaceDecl
	Funcs          map[string]*ast.FuncDecl
	Impls          []*ast.ImplDecl
	DotImportTypes map[string]struct{}
	// DotImportFuncs are callable bindings supplied by the automatically
	// dot-imported prelude. They participate in type inference and validation,
	// but are not local declarations and therefore must not be emitted again.
	DotImportFuncs map[string]*ast.FuncDecl
	Files          map[string][]ast.Decl // source file name -> declarations
}

// ImportSpec represents a single import with alias and path.
type ImportSpec struct {
	Alias string
	Path  string
}
