package codegen2

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	. "github.com/mygo-lang/mygo/prelude"
)

func TestParseSourceAsAst2KeepsInterfaceAndImplDecls(t *testing.T) {
	src := `package sample

interface Show[A]
  func Show(value: A) -> String
end

impl IntShow: Show[Int]
  func Show(value: Int) -> String
end
`

	got := parseSourceAsAst2(src)
	ok, yes := got.(ResultOk[ast2.File, string])
	if !yes {
		t.Fatalf("parseSourceAsAst2 failed: %v", got)
	}
	if len(ok.F0.Decls) != 2 {
		t.Fatalf("decl count = %d, want 2", len(ok.F0.Decls))
	}
	if _, yes := ok.F0.Decls[0].(ast2.DeclInterfaceDecl); !yes {
		t.Fatalf("decl[0] = %T, want ast2.DeclInterfaceDecl", ok.F0.Decls[0])
	}
	impl, yes := ok.F0.Decls[1].(ast2.DeclImplDecl)
	if !yes {
		t.Fatalf("decl[1] = %T, want ast2.DeclImplDecl", ok.F0.Decls[1])
	}
	if len(impl.F3) != 1 {
		t.Fatalf("impl method count = %d, want 1", len(impl.F3))
	}
}

func TestGenerateSourceUsesCurrentImplMangling(t *testing.T) {
	src := `package sample

interface Pretty[A]
  func Show(value: A) -> String
end

impl IntPretty: Pretty[Int]
  func Show(value: Int) -> String
end
`

	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "func MygoIT6PrettyFN9IntPrettyGN3IntEM4Show(value int) string") {
		t.Fatalf("generated impl helper does not use current mangling:\n%s", code)
	}
	if strings.Contains(code, "impl_pretty") || strings.Contains(code, "Show_impl") {
		t.Fatalf("generated impl helper still uses legacy temporary naming:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}
