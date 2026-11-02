package parser2

import (
	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"strings"
	"testing"
)

func TestArraySyntaxLossless(t *testing.T) {
	source := "package arrays\ntype Values = Slice[Option[Int32][5]][3]\ntype Commented = Int[5 # length\n]\nlet value = [0] as Int[1] * 5\n"
	tree := ParseSyntaxAt("arrays.mygo", source)
	if len(tree.Diagnostics) != 0 || cstNodeText(*tree.Root) != source {
		t.Fatalf("lossless array CST: %v", tree.Diagnostics)
	}
	file, ok := resultOk[ast2.File, string](LowerSyntax(tree))
	if !ok {
		t.Fatal(LowerSyntax(tree))
	}
	decl := file.F0.Decls[len(file.F0.Decls)-1]
	if !ast2.DeclIsLet(decl) {
		t.Fatalf("last declaration is not a let: %#v", decl)
	}
	value := ast2.DeclLetBind(decl).Value
	if !ast2.ExprIsBinary(value) || ast2.ExprBinary(value).Operator != "*" {
		t.Fatalf("array repetition precedence: %#v", value)
	}
	if !ast2.ExprIsTypeAs(ast2.ExprBinary(value).Left) {
		t.Fatal("array ascription must bind before multiplication")
	}
	for _, length := range []string{"-1", "1.5", "1 + 1", "9223372036854775808", "N"} {
		result := ParseFileAt("bad.mygo", "package arrays\ntype Bad = Int["+length+"]\n")
		failure, ok := resultErr[ast2.File, string](result)
		if !ok || !strings.Contains(failure.F0, "bad.mygo:2:") || !strings.Contains(failure.F0, "array length") {
			t.Fatalf("length %s diagnostic: %v", length, result)
		}
	}
}
