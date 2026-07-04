package compiler

import (
	"strings"
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

func TestGenStructEmitsTags(t *testing.T) {
	g := &generator{}
	st := &StructDecl{
		Name: "User",
		Fields: []Field{
			{Name: "id", Type: &NamedType{Name: "Int"}, Tag: "json:\"id\""},
			{Name: "name", Type: &NamedType{Name: "String"}, Tag: "json:\"name,omitempty\" yaml:\"name\""},
		},
	}

	out := g.genStruct(st)
	if len(out) != 1 {
		t.Fatalf("len(genStruct output) = %d, want %d", len(out), 1)
	}
	got := strings.TrimSpace(out[0].GoString())
	if !strings.Contains(got, "`json:\"id\"`") {
		t.Fatalf("generated struct missing id tag:\n%s", got)
	}
	if !strings.Contains(got, "`json:\"name,omitempty\" yaml:\"name\"`") {
		t.Fatalf("generated struct missing name tag:\n%s", got)
	}
}
