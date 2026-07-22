package goast

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestRenderFileBuildsImportsAndDecls(t *testing.T) {
	decl := Func("answer", nil, []*ast.Field{Field(nil, Ident("int"), "")}, []Stmt{
		Return([]Expr{Number("42")}),
	})

	source, err := RenderFile("sample", []Import{{Path: "fmt"}}, []Decl{decl})
	if err != nil {
		t.Fatalf("RenderFile() error: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.go", source, 0); err != nil {
		t.Fatalf("RenderFile() produced invalid Go: %v\n%s", err, source)
	}
	if !strings.Contains(source, "func answer() int") || !strings.Contains(source, "return 42") {
		t.Fatalf("RenderFile() =\n%s", source)
	}
}

func TestRenderSourcesRejectsMultipleDeclarations(t *testing.T) {
	_, err := RenderSources("sample", nil, []string{"type A struct{}\ntype B struct{}"})
	if err == nil || !strings.Contains(err.Error(), "expected one declaration") {
		t.Fatalf("RenderSources() error = %v, want declaration-count error", err)
	}
}
