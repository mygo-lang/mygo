package compiler

import (
	"path/filepath"
	"strings"
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference"
)

func TestPreludeConversionReportsBadReturnTypes(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "prelude")
	p, _, err := loadPackage(dir, true)
	if err != nil {
		t.Fatalf("loadPackage() error = %v", err)
	}
	p.WorkspaceRoot = filepath.Dir(dir)

	info, err := typeinference.InferPackage(&typeinference.PkgInfo{
		Dir:            p.Dir,
		WorkspaceRoot:  p.WorkspaceRoot,
		Name:           p.Name,
		Decls:          p.Decls,
		Enums:          p.Enums,
		Structs:        p.Structs,
		Interfaces:     p.Interfaces,
		Funcs:          p.Funcs,
		Impls:          p.Impls,
		DotImportEnums: map[string]*EnumDecl{},
	}, typeinference.NewInferState())
	if err != nil {
		if !strings.Contains(err.Error(), "return type mismatch") {
			t.Fatalf("InferPackage() error = %v, want return type mismatch", err)
		}
		return
	}

	err = Validate(p, info)
	if err == nil {
		t.Fatal("Validate() succeeded, want bad conversion return type error")
	}
	if !strings.Contains(err.Error(), "return type mismatch") {
		t.Fatalf("Validate() error = %v, want return type mismatch", err)
	}
}
