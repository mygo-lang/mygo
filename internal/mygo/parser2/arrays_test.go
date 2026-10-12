package parser2

import (
	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	. "github.com/mygo-lang/mygo/prelude"
	"strings"
	"testing"
)

func TestArraySyntaxLossless(t *testing.T) {
	source := "package arrays\ntype Values = Slice[Option[Int32][5]][3]\ntype Commented = Int[5 # length\n]\nlet value = [0] as Int[1] * 5\n"
	tree := ParseSyntaxAt("arrays.mygo", source)
	if len(tree.Diagnostics) != 0 || cstNodeText(*tree.Root) != source {
		t.Fatalf("lossless array CST: %v", tree.Diagnostics)
	}
	file, ok := LowerSyntax(tree).(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatal(LowerSyntax(tree))
	}
	decl := file.F0.Decls[len(file.F0.Decls)-1].(ast2.Decl__LetDecl)
	binary, ok := decl.F0.Value.Kind.(ast2.ExprKind__BinaryExpr)
	if !ok || binary.F0 != "*" {
		t.Fatalf("array repetition precedence: %v", decl.F0.Value.Kind)
	}
	if _, ok := binary.F1.Kind.(ast2.ExprKind__TypeAsExpr); !ok {
		t.Fatal("array ascription must bind before multiplication")
	}
	for _, length := range []string{"-1", "1.5", "1 + 1", "9223372036854775808", "N"} {
		result := ParseFileAt("bad.mygo", "package arrays\ntype Bad = Int["+length+"]\n")
		failure, ok := result.(Result__Err[ast2.File, string])
		if !ok || !strings.Contains(failure.F0, "bad.mygo:2:") || !strings.Contains(failure.F0, "array length") {
			t.Fatalf("length %s diagnostic: %v", length, result)
		}
	}
}
