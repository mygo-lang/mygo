package formatter

import (
	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/parser2"
	. "github.com/mygo-lang/mygo/prelude"
	"testing"
)

func TestArrayFormatting(t *testing.T) {
	source := "package arrays\ntype Rows = Slice[Option[Int32][5]][3]\nlet values=[0] as Int[1]*5\nlet number=2*3 as Int\n"
	one, ok := formattedText(FormatSource("arrays.mygo", source))
	if !ok {
		t.Fatal(FormatSource("arrays.mygo", source))
	}
	two, ok := formattedText(FormatSource("arrays.mygo", one))
	if !ok || one != two {
		t.Fatalf("non-idempotent formatting: %v", two)
	}
	if parsed := parser2.ParseFile(one); !MygoIN6ResultM4IsOk[ast2.File, string](parsed) {
		t.Fatal(parsed)
	}
}
