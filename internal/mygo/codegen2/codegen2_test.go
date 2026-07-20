package codegen2

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
	. "github.com/mygo-lang/mygo/prelude"
)

func TestParseSourceAsAst2KeepsInterfaceAndImplDecls(t *testing.T) {
	src := `package sample

interface Show[A]
  func Show(value: A) -> String
end

impl IntShow: Show[Int]
  func Show(value: Int) -> String
    "show"
  end
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
	body, yes := impl.F3[0].Body.(ast2.ExprBlockExpr)
	if !yes || len(body.F0) != 1 {
		t.Fatalf("impl method body = %T, want single-item ast2.ExprBlockExpr", impl.F3[0].Body)
	}
	if _, yes := body.F0[0].(ast2.ExprStringExpr); !yes {
		t.Fatalf("impl method body item = %T, want ast2.ExprStringExpr", body.F0[0])
	}
}

func TestGenerateSourceUsesCurrentImplMangling(t *testing.T) {
	src := `package sample

interface Pretty[A]
  func Show(value: A) -> String
end

impl IntPretty: Pretty[Int]
  func Show(value: Int) -> String
    "pretty"
  end
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

func TestGenerateFilesUsesGoTestOutputName(t *testing.T) {
	file := ast2.File{PackageName: "sample", Decls: []ast2.Decl{}}
	got := GenerateFiles([]SourceFileInput{{Path: "math_test.mygo", File: file}}, typeinference2.PackageInfo{})
	ok, yes := got.(ResultOk[map[string]string, string])
	if !yes {
		t.Fatalf("GenerateFiles failed: %v", got)
	}
	if _, exists := ok.F0["zz_math.gen_test.go"]; !exists {
		t.Fatalf("generated files = %#v, want zz_math.gen_test.go", ok.F0)
	}
	if _, exists := ok.F0["zz_math_test.gen.go"]; exists {
		t.Fatalf("generated legacy test filename: %#v", ok.F0)
	}
}

func TestGenerateSourceLowersExpressionIfToTemp(t *testing.T) {
	src := `package sample

func pick(flag: Bool) -> Int
  let value: Int = if flag then
    1
  else
    2
  end
  value
end
`

	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if strings.Contains(code, "func() any") {
		t.Fatalf("generated expression if still uses IIFE:\n%s", code)
	}
	if !strings.Contains(code, "var __mygo_expr_0 int") {
		t.Fatalf("generated expression if does not allocate typed temp:\n%s", code)
	}
	if !strings.Contains(code, "var value int = __mygo_expr_0") {
		t.Fatalf("typed let was not emitted as a Go var declaration:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceInfersStructFieldAccess(t *testing.T) {
	src := `package sample

struct Point
  x: Int
  name: String
end

func getX(p: Point) -> Int
  p.x
end
`

	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "type Point struct") || !strings.Contains(code, "X    int") || !strings.Contains(code, "Name string") {
		t.Fatalf("struct fields were not generated as expected:\n%s", code)
	}
	if !strings.Contains(code, "return p.X") {
		t.Fatalf("field access was not lowered as expected:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}
