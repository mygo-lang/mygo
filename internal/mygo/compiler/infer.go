package compiler

import (
	"github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference"
)

// InferTyped runs type inference on a package and returns the TypedInfo.
func InferTyped(p *Package) (*typeinference.TypedInfo, error) {
	pkgInfo := &typeinference.PkgInfo{
		Dir:            p.Dir,
		WorkspaceRoot:  p.WorkspaceRoot,
		Name:           p.Name,
		Decls:          p.Decls,
		Enums:          p.Enums,
		Structs:        p.Structs,
		Interfaces:     p.Interfaces,
		Funcs:          p.Funcs,
		Impls:          p.Impls,
		DotImportEnums: map[string]*ast.EnumDecl{},
	}
	state := typeinference.NewInferState()
	return typeinference.InferPackage(pkgInfo, state)
}
