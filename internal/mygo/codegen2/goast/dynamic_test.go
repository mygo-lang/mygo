package goast

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestDynamicValueDeclsEmitOrdinaryWrapperAPI(t *testing.T) {
	fset := token.NewFileSet()
	file := &ast.File{Name: ast.NewIdent("generated"), Decls: DynamicValueDecls("MygoDynamicShow")}
	var source strings.Builder
	if err := format.Node(&source, fset, file); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"type MygoDynamicShow struct",
		"func NewMygoDynamicShow(payload any, implementation string) MygoDynamicShow",
		"func MygoDynamicShowPayload(value MygoDynamicShow) any",
		"func MygoDynamicShowImplementation(value MygoDynamicShow) string",
	} {
		if !strings.Contains(source.String(), want) {
			t.Errorf("generated wrapper API is missing %q:\n%s", want, source.String())
		}
	}
	usage, err := parser.ParseFile(fset, "usage.go", `package generated
var _ = []MygoDynamicShow{
	NewMygoDynamicShow(42, "int-show"),
	NewMygoDynamicShow(struct{ Name string }{"widget"}, "widget-show"),
}
`, parser.AllErrors)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&types.Config{}).Check("generated", fset, []*ast.File{file, usage}, nil); err != nil {
		t.Fatalf("different concrete payloads do not share one wrapper type: %v", err)
	}
}

func TestDynamicAdapterDeclCallsPackageLevelHelper(t *testing.T) {
	adapter, err := DynamicAdapterDecl(
		"MygoDynamicAdapterShowWidget",
		[]string{"B"}, []string{"value", "format"}, []string{"Widget", "B"},
		0, "Widget", "mygoShowWidget", []string{"B"}, []string{"string"},
	)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file := &ast.File{Name: ast.NewIdent("generated"), Decls: []ast.Decl{adapter}}
	var source strings.Builder
	if err := format.Node(&source, fset, file); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(source.String(), "func (Widget)") {
		t.Fatalf("dynamic adapter unexpectedly became a Go receiver method:\n%s", source.String())
	}
	helper, err := parser.ParseFile(fset, "helper.go", `package generated
type Widget struct{}
func mygoShowWidget[B any](value Widget, format B) string { return "shown" }
`, parser.AllErrors)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&types.Config{}).Check("generated", fset, []*ast.File{file, helper}, nil); err != nil {
		t.Fatalf("generated adapter does not call its package-level helper: %v\n%s", err, source.String())
	}
}
