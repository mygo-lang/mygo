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

func TestSliceDropReturnsSuffixView(t *testing.T) {
	items := []int{1, 2, 3}
	got := sliceDrop(items, 1)
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("sliceDrop(items, 1) = %v, want [2 3]", got)
	}
	got[0] = 9
	if items[1] != 9 {
		t.Fatalf("sliceDrop copied its result: items = %v", items)
	}
	if got := sliceDrop(items, -1); len(got) != len(items) {
		t.Fatalf("sliceDrop(items, -1) length = %d, want %d", len(got), len(items))
	}
	if got := sliceDrop(items, len(items)); len(got) != 0 {
		t.Fatalf("sliceDrop(items, len(items)) = %v, want empty", got)
	}
}

func TestPackageIndexCollectsLoweringMetadata(t *testing.T) {
	src := `package sample

struct User
  Name: String
end

enum Flag
  Ready
end

interface Pretty[A]
  func Show(value: A) -> String
end

impl IntPretty: Pretty[Int]
  func Show(value: Int) -> String
    "pretty"
  end
end

func render[A](value: A) -> String using Pretty[A]
  value.Show()
end
`
	parsed := parseSourceAsAst2(src)
	file, ok := parsed.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("parseSourceAsAst2 failed: %v", parsed)
	}
	index := newPackageIndex(file.F0.Decls)
	if len(index.StructFields["User"]) != 1 {
		t.Fatalf("struct fields = %#v, want User.Name", index.StructFields)
	}
	if index.EnumValueConstructors["Flag.Ready"] == "" || index.EnumVariantOwners["Ready"] != "Flag" {
		t.Fatalf("enum metadata is incomplete: constructors=%#v owners=%#v", index.EnumValueConstructors, index.EnumVariantOwners)
	}
	if len(index.InterfaceMethods["Pretty"]) != 1 || len(index.InterfaceTypeParams["Pretty"]) != 1 {
		t.Fatalf("interface metadata is incomplete: methods=%#v params=%#v", index.InterfaceMethods, index.InterfaceTypeParams)
	}
	if !index.NamedImpls["IntPretty"] {
		t.Fatalf("named impls = %#v, want IntPretty", index.NamedImpls)
	}
	if got := index.CallDictionaries["render"]; len(got) != 1 || got[0] != "Show" {
		t.Fatalf("call dictionaries = %#v, want render: Show", index.CallDictionaries)
	}
	if got := index.CallRequirements["render"]; len(got) != 1 || got[0].Interface != "Pretty" || got[0].Method != "Show" {
		t.Fatalf("call requirements = %#v, want Pretty.Show", index.CallRequirements)
	}
	if len(index.PackageCandidates["Pretty.Show"]) != 1 || len(index.TypeclassCandidates) != 1 {
		t.Fatalf("impl candidates are incomplete: %#v", index.PackageCandidates)
	}
}

func TestGenerateSourceBootstrapsPrelude(t *testing.T) {
	assertBootstrapsMyGOFile(t, filepath.Join("..", "..", "..", "prelude", "prelude.mygo"))
}

func TestGenerateSourceLowersZeroArgumentEnumVariantSelector(t *testing.T) {
	src := `package sample

enum Flag
  Ready
  Failed(String)
end

func ready() -> Flag
  Flag.Ready
end
`
	got := GenerateSource(src)
	result, ok := got.(ResultOk[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "FlagReadyCtor()") {
		t.Fatalf("generated Go does not call zero-argument enum ctor:\n%s", result.F0)
	}
}

func TestGenerateSourceLowersPackageLetAndVar(t *testing.T) {
	src := `package sample

func Read() -> Int
  fixed + changing
end

let fixed: Int = 40
var changing: Int = 2

func Increment() -> Int
  changing = changing + 1
  changing
end
`
	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	for _, want := range []string{"var fixed int = 40", "var changing int = 2", "func Read() int", "changing = changing + 1"} {
		if !strings.Contains(ok.F0, want) {
			t.Fatalf("generated source missing %q:\n%s", want, ok.F0)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "generated.go", ok.F0, parser.AllErrors); err != nil {
		t.Fatalf("generated source is invalid Go: %v\n%s", err, ok.F0)
	}
}

func TestGenerateSourcePreservesAliasAndDefinedTypeSyntax(t *testing.T) {
	src := `package sample

type UserID = Int
type AccountID Int

func use(value: UserID) -> AccountID
  go[AccountID]{code: "return AccountID(value)"}
end
`
	got := GenerateSource(src)
	result, ok := got.(ResultOk[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "type UserID = int") || !strings.Contains(result.F0, "type AccountID int") {
		t.Fatalf("generated Go lost type declaration distinction:\n%s", result.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "types.gen.go", result.F0, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestGenerateSourceExpandsGenericTypeAliasDuringInference(t *testing.T) {
	src := `package sample

type Values[A] = Slice[A]

func echo(values: Values[Int]) -> Slice[Int]
  values
end
`
	got := GenerateSource(src)
	result, ok := got.(ResultOk[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "type Values[A any] = []A") || !strings.Contains(result.F0, "func echo(values Values[int]) []int") {
		t.Fatalf("generated Go lost generic alias semantics:\n%s", result.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "generic-types.gen.go", result.F0, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestGenerateSourceReturnsFunctionLiteralThroughTypeAlias(t *testing.T) {
	src := `package sample

type Parser[A] = func(Int) -> A

func pure[A](value: A) -> Parser[A]
  func(state: Int) -> A
    value
  end
end
`
	got := GenerateSource(src)
	result, ok := got.(ResultOk[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "return func(state int) A") {
		t.Fatalf("generated Go did not return the function literal:\n%s", result.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "function-alias.gen.go", result.F0, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestGenerateSourceLowersRefBoundaryOperations(t *testing.T) {
	src := `package sample

func copy(value: Ref[Int]) -> Int
  value.value()
end

func address(value: Int) -> Ref[Int]
  Ref.new(value)
end

func number() -> Int
  1
end

func address_call() -> Ref[Int]
  Ref.new(number())
end
`
	got := GenerateSource(src)
	result, ok := got.(ResultOk[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	for _, want := range []string{"func copy(value *int) int", "return *value", "func address(value int) *int", "return &value", "return &[]int{number()}[0]"} {
		if !strings.Contains(result.F0, want) {
			t.Fatalf("generated Go missing %q:\n%s", want, result.F0)
		}
	}
}

func TestGenerateSourceAllowsExhaustiveVariantSwitchWithoutWildcard(t *testing.T) {
	src := `package sample

enum Maybe[A]
  Have(A)
  Nothing
end

func unwrap(value: Maybe[Int]) -> Int
  switch value
    case Have(item) => item
    case Nothing => 0
  end
end
`
	got := GenerateSource(src)
	result, ok := got.(ResultOk[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "exhaustive-switch.gen.go", result.F0, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
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

func TestGenerateSourceIncludesExpressionPositionInInferenceErrors(t *testing.T) {
	got := GenerateSource(`package sample

func broken() -> Int
  missing
end
`)
	err, ok := got.(ResultErr[string, string])
	if !ok {
		t.Fatalf("GenerateSource unexpectedly succeeded: %v", got)
	}
	if !strings.Contains(err.F0, "<input>:4:3: unknown identifier missing") {
		t.Fatalf("error lacks expression position: %q", err.F0)
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
	bodyExpr, yes := impl.F3[0].Body.Kind.(ast2.ExprKindBlockExpr)
	if !yes || len(bodyExpr.F0) != 1 {
		t.Fatalf("impl method body = %T, want single-item ast2.ExprBlockExpr", impl.F3[0].Body)
	}
	first, yes := bodyExpr.F0[0].(ast2.StmtExprStmt)
	if !yes {
		t.Fatalf("impl method body item = %T, want ast2.StmtExprStmt", bodyExpr.F0[0])
	}
	if _, yes := first.F0.Kind.(ast2.ExprKindStringExpr); !yes {
		t.Fatalf("impl method body expr = %T, want ast2.ExprStringExpr", first.F0)
	}
}

func TestGenerateSourceEncodesHigherKindedParameters(t *testing.T) {
	src := `package prelude

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

func TestGenerateSourceUsesPreludeHKTDeclarations(t *testing.T) {
	src := `package sample

interface Enumerable[C[A], A]
  func First(value: C[A]) -> A
end

func Default(value: Option[Int]) -> Option[Int]
  value
end
`
	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if strings.Contains(ok.F0, "type HKTType interface{}") {
		t.Fatalf("non-prelude package redeclared prelude HKT helpers:\n%s", ok.F0)
	}
	if !strings.Contains(ok.F0, `. "github.com/mygo-lang/mygo/prelude"`) {
		t.Fatalf("non-prelude package did not dot-import prelude:\n%s", ok.F0)
	}
}

func TestGenerateSourceLowersEmptySliceStructFieldWithGenericElement(t *testing.T) {
	src := `package sample

struct Reply[A]
  Value: A
end

func Empty[A]() -> Reply[Slice[A]]
  Reply[Slice[A]] { Value: [] }
end
`
	got := GenerateSource(src)
	ok, yes := got.(ResultOk[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(ok.F0, "Reply[[]A]{Value: []A{}}") {
		t.Fatalf("empty generic slice field was not lowered with its field type:\n%s", ok.F0)
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
  Have(Int)
  Nothing
end

func unwrap(value: Maybe) -> Int
  switch value
    case Have(item) => item
    case Nothing => 0
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
	if !strings.Contains(code, "value.(MaybeHave)") || !strings.Contains(code, ".F0") {
		t.Fatalf("generated variant switch is missing assertion or field binding:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceInfersResultVariantArgumentsInSwitchBranches(t *testing.T) {
	src := `package sample

enum Outcome[A, E]
  Success(A)
  Failure(E)
end

func parse(value: String) -> Outcome[Int, String]
  switch value
    case "ok" => Success(1)
    case _ => Failure("invalid")
  end
end
`

	got := GenerateSource(src)
	result, ok := got.(ResultOk[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	for _, want := range []string{"Success[int, string](1)", "Failure[int, string](\"invalid\")"} {
		if !strings.Contains(result.F0, want) {
			t.Fatalf("generated Go is missing %q:\n%s", want, result.F0)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "result-switch.gen.go", result.F0, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
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
