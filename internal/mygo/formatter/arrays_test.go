package formatter

import (
	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/parser2"
	. "github.com/mygo-lang/mygo/prelude"
	"testing"
)

func TestArrayFormatting(t *testing.T) {
	source := "package arrays\ntype Rows = Slice[Option[Int32][5]][3]\nlet values=[0] as Int[1]*5\nlet number=2*3 as Int\n"
	one, ok := FormatSource("arrays.mygo", source).(Result__Ok[string, string])
	if !ok {
		t.Fatal(FormatSource("arrays.mygo", source))
	}
	two, ok := FormatSource("arrays.mygo", one.F0).(Result__Ok[string, string])
	if !ok || one.F0 != two.F0 {
		t.Fatalf("non-idempotent formatting: %v", two)
	}
	if _, ok := parser2.ParseFile(one.F0).(Result__Ok[ast2.File, string]); !ok {
		t.Fatal(parser2.ParseFile(one.F0))
	}
}
