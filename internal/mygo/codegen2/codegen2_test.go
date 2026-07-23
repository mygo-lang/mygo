package codegen2

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
	. "github.com/mygo-lang/mygo/prelude"
)

func TestGenerateSourceBootstrapsAst2(t *testing.T) {
	assertBootstrapsMyGOFile(t, filepath.Join("..", "ast2", "ast2.mygo"))
}

func TestGenerateSourceAtIncludesSourceLocation(t *testing.T) {
	got := GenerateSourceAt("broken.mygo", "package sample\n\nfunc")
	err, ok := got.(ResultErr[string, string])
	if !ok {
		t.Fatalf("GenerateSourceAt() = %T, want parse error", got)
	}
	if !strings.Contains(err.F0, "broken.mygo:3:5: parse error") {
		t.Fatalf("GenerateSourceAt() error = %q, want source name, line, and column", err.F0)
	}
}

func assertBootstrapsMyGOFile(t *testing.T, relativePath string) {
	t.Helper()
	_, thisFile, _, found := runtime.Caller(0)
	if !found {
		t.Fatal("cannot determine codegen2 test path")
	}
	sourcePath := filepath.Join(filepath.Dir(thisFile), relativePath)
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read %s: %v", sourcePath, err)
	}
	got := GenerateSourceAt(sourcePath, string(source))
	result, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource(%s) failed: %v", sourcePath, got)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "ast2.bootstrap.go", result.F0, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

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
	bodyExpr, yes := impl.F3[0].Body.(ast2.ExprBlockExpr)
	if !yes || len(bodyExpr.F0) != 1 {
		t.Fatalf("impl method body = %T, want single-item ast2.ExprBlockExpr", impl.F3[0].Body)
	}
	first, yes := bodyExpr.F0[0].(ast2.StmtExprStmt)
	if !yes {
		t.Fatalf("impl method body item = %T, want ast2.StmtExprStmt", bodyExpr.F0[0])
	}
	if _, yes := first.F0.(ast2.ExprStringExpr); !yes {
		t.Fatalf("impl method body expr = %T, want ast2.ExprStringExpr", first.F0)
	}
}

func TestGenerateSourceEncodesHigherKindedParameters(t *testing.T) {
	src := `package sample

interface Enumerable[C[A], A]
  func First(value: C[A]) -> A
end
`
	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(ok.F0, "type HKT[F any, A any] interface{}") {
		t.Fatalf("HKT application was not preserved:\n%s", ok.F0)
	}
	if strings.Contains(ok.F0, "type Enumerable") {
		t.Fatalf("typeclass interface must not be emitted as a Go interface:\n%s", ok.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "hkt.gen.go", ok.F0, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, ok.F0)
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

func TestGenerateSourceVarDeclaration(t *testing.T) {
	src := `package sample

func foo() -> Int
  var x: Int = 42
  x
end
`

	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "var x int = 42") {
		t.Fatalf("generated var declaration is missing:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceLetRecBindingGroup(t *testing.T) {
	src := `package sample

func parity(n: Int) -> Bool
  letrec
    even: func(Int) -> Bool = func(value: Int) -> Bool
      if value == 0 => true else odd(value - 1)
    end
    odd: func(Int) -> Bool = func(value: Int) -> Bool
      if value == 0 => false else even(value - 1)
    end
  end
  even(n)
end
`

	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	for _, want := range []string{
		"var even func(int) bool",
		"var odd func(int) bool",
		"even = func(value int) bool",
		"odd = func(",
		"return odd(value - 1)",
		"return even(value_1 - 1)",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("generated letrec is missing %q:\n%s", want, code)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceWhileLoop(t *testing.T) {
	src := `package sample

func count(n: Int) -> Int
  var i: Int = 0
  while i > 10
    i = i + 1
  end
  i
end
`

	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "for") {
		t.Fatalf("generated while loop is missing 'for':\n%s", code)
	}
	if !strings.Contains(code, ">") {
		t.Fatalf("generated while loop is missing condition operator '>':\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceInlineGoExpression(t *testing.T) {
	src := `package sample

import strconv "go:strconv"

func toString(n: Int) -> String
  go[String]{code: "return strconv.Itoa(n)"}
end
`

	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "return strconv.Itoa(n)") {
		t.Fatalf("generated inline go is missing body:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceInlineGoOperands(t *testing.T) {
	src := `package sample

func toString(n: Int) -> String
  go[String]{code: "{T}({v})" in v = n type T = String}
end
`

	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(ok.F0, "return string(n)") {
		t.Fatalf("inline operands were not substituted in generated AST:\n%s", ok.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", ok.F0, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, ok.F0)
	}
}

func TestGenerateSourceReturnStatement(t *testing.T) {
	src := `package sample

func earlyReturn(flag: Bool) -> Int
  if flag then
    return 1
  else
    return 2
  end
end
`

	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "return 1") {
		t.Fatalf("generated return statement is missing 'return 1':\n%s", code)
	}
	if !strings.Contains(code, "return 2") {
		t.Fatalf("generated return statement is missing 'return 2':\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceBareReturn(t *testing.T) {
	src := `package sample

func bareReturn(flag: Bool) -> ()
  if flag then
    return
  end
  ()
end
`

	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "return") {
		t.Fatalf("generated bare return is missing 'return':\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceLowersLiteralSwitch(t *testing.T) {
	src := `package sample

func classify(value: Int) -> String
  switch value
    case 0 => "zero"
    case _ => "other"
  end
end
`

	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "if value == 0") {
		t.Fatalf("generated switch is missing literal guard:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceLowersVariantSwitchWithBinding(t *testing.T) {
	src := `package sample

enum Maybe
  Some(Int)
  None
end

func unwrap(value: Maybe) -> Int
  switch value
    case Some(item) => item
    case None => 0
    case _ => 0
  end
end
`

	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "value.(MaybeSome)") || !strings.Contains(code, ".F0") {
		t.Fatalf("generated variant switch is missing assertion or field binding:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceSupportsForwardFunctionReferences(t *testing.T) {
	src := `package sample

func first() -> Int
  second()
end

func second() -> Int
  42
end
`
	got := GenerateSource(src)
	result, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", result.F0, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestGenerateSourceLowersEscapedRune(t *testing.T) {
	src := `package sample

func newline() -> Rune
  '\n'
end
`
	got := GenerateSource(src)
	result, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "return '\\n'") {
		t.Fatalf("generated rune literal is missing:\n%s", result.F0)
	}
}

func TestGenerateSourceLowersTypedSliceLiteral(t *testing.T) {
	src := `package sample

func values() -> Slice[Int]
  [1, 2,] as Slice[Int]
end
`
	got := GenerateSource(src)
	result, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "return []int{1, 2}") {
		t.Fatalf("generated typed slice literal is missing:\n%s", result.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", result.F0, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestGenerateSourcePreservesGenericStructLiteralArguments(t *testing.T) {
	src := `package sample

struct Box[A]
  value: A
end

func makeBox() -> Box[Int]
  Box[Int] { value: 42 }
end
`
	got := GenerateSource(src)
	result, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "return Box[int]{Value: 42}") {
		t.Fatalf("generated generic struct literal lost type arguments:\n%s", result.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", result.F0, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}
