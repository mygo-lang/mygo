package parser2

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	ps "github.com/mygo-lang/mygo/lib/text/parsec"
	. "github.com/mygo-lang/mygo/prelude"
)

func TestParseFileAtIncludesSourceLocation(t *testing.T) {
	got := ParseFileAt("broken.mygo", "package sample\n\nfunc")
	err, ok := got.(ResultErr[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFileAt() = %T, want parse error", got)
	}
	if !strings.Contains(err.F0, "broken.mygo:3:5: parse error") {
		t.Fatalf("ParseFileAt() error = %q, want source name, line, and column", err.F0)
	}
}

func TestParseFileParsesSelf(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine parser test path")
	}
	sourcePath := filepath.Join(filepath.Dir(thisFile), "parser.mygo")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read %s: %v", sourcePath, err)
	}
	reply := ps.ParseInput(fileParser(), string(source))
	if !reply.Ok {
		t.Fatalf("ParseInput(%s) failed at %#v: %#v", sourcePath, reply.State.Position, reply.Error)
	}
	got := ParseFile(string(source))
	parsed, ok := got.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile(%s) failed: %v", sourcePath, got)
	}
	if len(parsed.F0.Decls) == 0 {
		t.Fatalf("ParseFile(%s) returned no declarations", sourcePath)
	}
}

func TestParseFileParsesPrelude(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine parser test path")
	}
	sourcePath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "prelude", "prelude.mygo")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read %s: %v", sourcePath, err)
	}
	got := ParseFileAt(sourcePath, string(source))
	parsed, ok := got.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFileAt(%s) failed: %v", sourcePath, got)
	}
	if len(parsed.F0.Decls) == 0 {
		t.Fatalf("ParseFileAt(%s) returned no declarations", sourcePath)
	}
	if parsed.F0.SourceName != sourcePath || parsed.F0.Line != 1 || parsed.F0.Column != 1 {
		t.Fatalf("file position = %q:%d:%d, want %q:1:1", parsed.F0.SourceName, parsed.F0.Line, parsed.F0.Column, sourcePath)
	}
	if len(parsed.F0.DeclPositions) != len(parsed.F0.Decls) || parsed.F0.DeclPositions[0].SourceName != sourcePath || parsed.F0.DeclPositions[0].Line != 3 {
		t.Fatalf("first declaration position = %#v, want %q:3:*", parsed.F0.DeclPositions[0], sourcePath)
	}
}

func TestParseFileParsesPreludeMapImpl(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine parser test path")
	}
	sourcePath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "prelude", "map.mygo")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read %s: %v", sourcePath, err)
	}
	parsed := ParseFileAt(sourcePath, string(source))
	if _, ok := parsed.(ResultOk[ast2.File, string]); !ok {
		t.Fatalf("ParseFileAt(%s) failed: %v", sourcePath, parsed)
	}
}

func TestParseFileParsesPreludeStringIndexImpl(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok { t.Fatal("cannot determine parser test path") }
	sourcePath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "prelude", "stringindexrune.mygo")
	source, err := os.ReadFile(sourcePath)
	if err != nil { t.Fatalf("read %s: %v", sourcePath, err) }
	parsed := ParseFileAt(sourcePath, string(source))
	if _, ok := parsed.(ResultOk[ast2.File, string]); !ok { t.Fatalf("ParseFileAt(%s) failed: %v", sourcePath, parsed) }
}

func TestParseFunctionLiteral(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func make(values: Slice[ast2.Decl])
func(_: String) -> ps.Parser[ast2.File]
    ps.PBind(kw("x"), func(_: String) -> ps.Parser[ast2.File]
      ps.PPure(ast2.File { PackageName: "sample", Decls: values })
    end)
  end
end
`)
	body := fn.F4.(ast2.ExprBlockExpr)
	got := body.F0[0].(ast2.StmtExprStmt).F0
	if _, ok := got.(ast2.ExprFuncLitExpr); !ok {
		t.Fatalf("body = %T, want ExprFuncLitExpr", got)
	}
}

func TestParseLetRecBindingGroup(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

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
`)
	body := fn.F4.(ast2.ExprBlockExpr)
	if len(body.F0) != 2 {
		t.Fatalf("body statement count = %d, want 2", len(body.F0))
	}
	rec, ok := body.F0[0].(ast2.StmtLetRecStmt)
	if !ok {
		t.Fatalf("first statement = %T, want StmtLetRecStmt", body.F0[0])
	}
	if len(rec.F0) != 2 || rec.F0[0].Name != "even" || rec.F0[1].Name != "odd" {
		t.Fatalf("letrec bindings = %#v, want even/odd", rec.F0)
	}
}

func TestParseSliceLiteralWithTrailingCommaAndTypeAs(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func values()
  [1, 2,] as Slice[Int]
end
`)
	body := fn.F4.(ast2.ExprBlockExpr)
	cast, ok := body.F0[0].(ast2.StmtExprStmt).F0.(ast2.ExprTypeAsExpr)
	if !ok {
		t.Fatalf("body = %T, want ExprTypeAsExpr", body.F0[0].(ast2.StmtExprStmt).F0)
	}
	slice, ok := (*cast.F0).(ast2.ExprSliceLitExpr)
	if !ok || len(slice.F0) != 2 {
		t.Fatalf("cast value = %T, want two-item ExprSliceLitExpr", *cast.F0)
	}
}

func TestParseEscapedRuneLiterals(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func newline() -> Rune
  '\n'
end
`)
	body := fn.F4.(ast2.ExprBlockExpr)
	runeExpr, ok := body.F0[0].(ast2.StmtExprStmt).F0.(ast2.ExprRuneExpr)
	if !ok {
		t.Fatalf("body = %T, want ExprRuneExpr", body.F0[0].(ast2.StmtExprStmt).F0)
	}
	if runeExpr.F0 != "\n" {
		t.Fatalf("rune value = %q, want newline", runeExpr.F0)
	}
}

func TestParseSwitchVariantAndWildcardPatterns(t *testing.T) {
	got := ParseFile(`package sample

enum Maybe[A]
  Some(A)
  None
end

func unwrap(value: Maybe[Int]) -> Int
  switch value
    case Some(item) => item
    case None => 0
    case _ => -1
  end
end
`)
	parsed, ok := got.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile failed: %v", got)
	}
	fn, ok := parsed.F0.Decls[1].(ast2.DeclFuncDecl)
	if !ok {
		t.Fatalf("decl[1] = %T, want DeclFuncDecl", parsed.F0.Decls[1])
	}
	body := fn.F4.(ast2.ExprBlockExpr)
	sw, ok := body.F0[0].(ast2.StmtExprStmt).F0.(ast2.ExprSwitchExpr)
	if !ok {
		t.Fatalf("body = %T, want ExprSwitchExpr", body.F0[0].(ast2.StmtExprStmt).F0)
	}
	if len(sw.F1) != 3 {
		t.Fatalf("case count = %d, want 3", len(sw.F1))
	}
	variant, ok := sw.F1[0].Pattern.(ast2.PatternVariantPattern)
	item, itemOK := variant.F1[0].(ast2.PatternBindPattern)
	if !ok || variant.F0 != "Some" || len(variant.F1) != 1 || !itemOK || item.F0 != "item" {
		t.Fatalf("first pattern = %#v, want Some(item)", sw.F1[0].Pattern)
	}
	if _, ok := sw.F1[2].Pattern.(ast2.PatternWildcardPattern); !ok {
		t.Fatalf("third pattern = %T, want PatternWildcardPattern", sw.F1[2].Pattern)
	}
}

func TestParseSwitchCaseBlock(t *testing.T) {
	got := ParseFile(`package sample

enum Maybe[A]
  Some(A)
  None
end

func unwrap(value: Maybe[Int]) -> Int
  switch value
    case Some(item) then
      item
    end
    case None => 0
  end
end
`)
	parsed, ok := got.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile failed: %v", got)
	}
	fn := parsed.F0.Decls[1].(ast2.DeclFuncDecl)
	body := fn.F4.(ast2.ExprBlockExpr)
	sw := body.F0[0].(ast2.StmtExprStmt).F0.(ast2.ExprSwitchExpr)
	if len(sw.F1) != 2 {
		t.Fatalf("case count = %d, want 2", len(sw.F1))
	}
	if _, ok := sw.F1[0].Body.(ast2.ExprIdentExpr); !ok {
		t.Fatalf("block case body = %T, want ast2.ExprIdentExpr", sw.F1[0].Body)
	}
}

func TestParseFileBasicDeclarations(t *testing.T) {
	src := `package sample

import fmt "go:fmt"

struct Point
  x: Int
  y: Int
end

enum Maybe[A]
  Some(A)
  None
end

func add(a: Int, b: Int) -> Int
  a + b
end
`

	got := ParseFile(src)
	ok, yes := got.(ResultOk[ast2.File, string])
	if !yes {
		t.Fatalf("ParseFile failed: %v", got)
	}
	file := ok.F0
	if file.PackageName != "sample" {
		t.Fatalf("package = %q, want sample", file.PackageName)
	}
	if len(file.Decls) != 4 {
		t.Fatalf("decl count = %d, want 4", len(file.Decls))
	}
}

func TestParseExpressionPrecedence(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func calc(a: Int, b: Int, c: Int, d: Int) -> Int
  a + b * c - d
end
`)

	body := fn.F4.(ast2.ExprBlockExpr)
	if len(body.F0) != 1 {
		t.Fatalf("body expr count = %d, want 1", len(body.F0))
	}
	first := body.F0[0].(ast2.StmtExprStmt)
	root := first.F0.(ast2.ExprBinaryExpr)
	if root.F0 != "-" {
		t.Fatalf("root op = %q, want -", root.F0)
	}
	left := (*root.F1).(ast2.ExprBinaryExpr)
	if left.F0 != "+" {
		t.Fatalf("left op = %q, want +", left.F0)
	}
	rightMul := (*left.F2).(ast2.ExprBinaryExpr)
	if rightMul.F0 != "*" {
		t.Fatalf("nested op = %q, want *", rightMul.F0)
	}
}

func TestParseLogicalAndUnaryExpressions(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func ok(a: Bool, b: Bool, c: Bool, n: Int) -> Bool
  !a || b && -n > 0
end
`)

	body := fn.F4.(ast2.ExprBlockExpr)
	first := body.F0[0].(ast2.StmtExprStmt)
	root := first.F0.(ast2.ExprBinaryExpr)
	if root.F0 != "||" {
		t.Fatalf("root op = %q, want ||", root.F0)
	}
	left := (*root.F1).(ast2.ExprUnaryExpr)
	if left.F0 != "!" {
		t.Fatalf("left unary op = %q, want !", left.F0)
	}
	right := (*root.F2).(ast2.ExprBinaryExpr)
	if right.F0 != "&&" {
		t.Fatalf("right op = %q, want &&", right.F0)
	}
	cmp := (*right.F2).(ast2.ExprBinaryExpr)
	if cmp.F0 != ">" {
		t.Fatalf("comparison op = %q, want >", cmp.F0)
	}
	neg := (*cmp.F1).(ast2.ExprUnaryExpr)
	if neg.F0 != "-" {
		t.Fatalf("comparison left unary op = %q, want -", neg.F0)
	}
}

func TestParseBlockIfWithElsif(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func choose(a: Int) -> Int
  if a > 10 then
    1
  elsif a > 5 then
    2
  else
    3
  end
end
`)

	body := fn.F4.(ast2.ExprBlockExpr)
	first := body.F0[0].(ast2.StmtExprStmt)
	root := first.F0.(ast2.ExprIfExpr)
	thenExpr := (*root.F1).(ast2.ExprNumberExpr)
	if thenExpr.F0 != "1" {
		t.Fatalf("then value = %q, want 1", thenExpr.F0)
	}
	nested := (*root.F2).(ast2.ExprIfExpr)
	nestedThen := (*nested.F1).(ast2.ExprNumberExpr)
	if nestedThen.F0 != "2" {
		t.Fatalf("elsif value = %q, want 2", nestedThen.F0)
	}
	elseExpr := (*nested.F2).(ast2.ExprNumberExpr)
	if elseExpr.F0 != "3" {
		t.Fatalf("else value = %q, want 3", elseExpr.F0)
	}
}

func TestParseVarDeclaration(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo() -> Int
  var x: Int = 42
  x
end
`)

	body := fn.F4.(ast2.ExprBlockExpr)
	if len(body.F0) != 2 {
		t.Fatalf("body expr count = %d, want 2", len(body.F0))
	}
	varStmt, ok := body.F0[0].(ast2.StmtVarStmt)
	if !ok {
		t.Fatalf("first stmt = %T, want StmtVarStmt", body.F0[0])
	}
	if varStmt.F0.Name != "x" {
		t.Fatalf("var name = %q, want x", varStmt.F0.Name)
	}
}

func TestParseWhileLoop(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo(n: Int) -> Int
  while n > 0
    n
  end
  n
end
`)

	body := fn.F4.(ast2.ExprBlockExpr)
	if len(body.F0) != 2 {
		t.Fatalf("body expr count = %d, want 2", len(body.F0))
	}
	whileStmt, ok := body.F0[0].(ast2.StmtWhileStmt)
	if !ok {
		t.Fatalf("first stmt = %T, want StmtWhileStmt", body.F0[0])
	}
	cond := whileStmt.F0.(ast2.ExprBinaryExpr)
	if cond.F0 != ">" {
		t.Fatalf("while cond op = %q, want >", cond.F0)
	}
}

func TestParseReturnStatement(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo(n: Int) -> Int
  return n
end
`)

	body := fn.F4.(ast2.ExprBlockExpr)
	retStmt, ok := body.F0[0].(ast2.StmtReturnWithStmt)
	if !ok {
		t.Fatalf("stmt = %T, want StmtReturnWithStmt", body.F0[0])
	}
	identExpr := retStmt.F0.(ast2.ExprIdentExpr)
	if identExpr.F0 != "n" {
		t.Fatalf("return ident = %q, want n", identExpr.F0)
	}
}

func TestParseBareReturnStatement(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo(n: Int)
  return
end
`)

	body := fn.F4.(ast2.ExprBlockExpr)
	_, ok := body.F0[0].(ast2.StmtReturnStmt)
	if !ok {
		t.Fatalf("stmt = %T, want StmtReturnStmt", body.F0[0])
	}
}

func TestParseInlineGoExpression(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo(n: Int) -> Int
  go[String]{code: "return strconv.Itoa(n)"}
end
`)

	body := fn.F4.(ast2.ExprBlockExpr)
	first := body.F0[0].(ast2.StmtExprStmt)
	_, ok := first.F0.(ast2.ExprInlineGoExpr)
	if !ok {
		t.Fatalf("expr = %T, want ExprInlineGoExpr", first.F0)
	}
}

func TestParseInlineGoOperands(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo(n: Int) -> String
  go[String]{code: "{T}({v})" in v = n type T = String}
end
`)
	body := fn.F4.(ast2.ExprBlockExpr)
	expr := body.F0[0].(ast2.StmtExprStmt).F0.(ast2.ExprInlineGoExpr)
	if len(expr.F2) != 1 || expr.F2[0].Name != "v" {
		t.Fatalf("value operands = %#v, want one v operand", expr.F2)
	}
	if len(expr.F3) != 1 || expr.F3[0].Name != "T" {
		t.Fatalf("type operands = %#v, want one T operand", expr.F3)
	}
}

func parseSingleFunc(t *testing.T, src string) ast2.DeclFuncDecl {
	t.Helper()

	got := ParseFile(src)
	ok, yes := got.(ResultOk[ast2.File, string])
	if !yes {
		t.Fatalf("ParseFile failed: %v", got)
	}
	if len(ok.F0.Decls) != 1 {
		t.Fatalf("decl count = %d, want 1", len(ok.F0.Decls))
	}
	fn, yes := ok.F0.Decls[0].(ast2.DeclFuncDecl)
	if !yes {
		t.Fatalf("decl type = %T, want DeclFuncDecl", ok.F0.Decls[0])
	}
	return fn
}

func TestParseAssignSimpleVar(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo() -> Int
  var x: Int = 42
  x = 1
  x
end
`)

	body := fn.F4.(ast2.ExprBlockExpr)
	if len(body.F0) != 3 {
		t.Fatalf("body expr count = %d, want 3", len(body.F0))
	}
	assign, ok := body.F0[1].(ast2.StmtAssignStmt)
	if !ok {
		t.Fatalf("second stmt = %T, want StmtAssignStmt", body.F0[1])
	}
	lhs := assign.F0
	rhs := assign.F1
	if lhs.(ast2.ExprIdentExpr).F0 != "x" {
		t.Fatalf("assign lhs = %q, want x", lhs.(ast2.ExprIdentExpr).F0)
	}
	if rhs.(ast2.ExprNumberExpr).F0 != "1" {
		t.Fatalf("assign rhs = %q, want 1", rhs.(ast2.ExprNumberExpr).F0)
	}
}

func TestParseAssignFieldSimple(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo()
  var p: Point = Point { x: 1, y: 2 }
  p.x = 99
end
`)

	body := fn.F4.(ast2.ExprBlockExpr)
	assign, ok := body.F0[1].(ast2.StmtAssignStmt)
	if !ok {
		t.Fatalf("second stmt = %T, want StmtAssignStmt", body.F0[1])
	}
	lhs := assign.F0
	field, ok := lhs.(ast2.ExprFieldExpr)
	if !ok {
		t.Fatalf("assign lhs = %T, want ExprFieldExpr", lhs)
	}
	if field.F1 != "x" {
		t.Fatalf("assign field name = %q, want x", field.F1)
	}
	obj := *field.F0
	if obj.(ast2.ExprIdentExpr).F0 != "p" {
		t.Fatalf("assign field obj = %q, want p", obj.(ast2.ExprIdentExpr).F0)
	}
	rhs := assign.F1
	if rhs.(ast2.ExprNumberExpr).F0 != "99" {
		t.Fatalf("assign rhs = %q, want 99", rhs.(ast2.ExprNumberExpr).F0)
	}
}

func TestParseAssignFieldChain(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo()
  cfg.settings.theme = "dark"
end
`)

	body := fn.F4.(ast2.ExprBlockExpr)
	assign, ok := body.F0[0].(ast2.StmtAssignStmt)
	if !ok {
		t.Fatalf("first stmt = %T, want StmtAssignStmt", body.F0[0])
	}
	// lhs = cfg.settings.theme
	lhs := assign.F0
	themeField, ok := lhs.(ast2.ExprFieldExpr)
	if !ok {
		t.Fatalf("assign lhs = %T, want ExprFieldExpr", lhs)
	}
	if themeField.F1 != "theme" {
		t.Fatalf("outer field = %q, want theme", themeField.F1)
	}
	// cfg.settings
	inner := *themeField.F0
	settingsField, ok := inner.(ast2.ExprFieldExpr)
	if !ok {
		t.Fatalf("inner = %T, want ExprFieldExpr", inner)
	}
	if settingsField.F1 != "settings" {
		t.Fatalf("inner field = %q, want settings", settingsField.F1)
	}
	cfg := *settingsField.F0
	if cfg.(ast2.ExprIdentExpr).F0 != "cfg" {
		t.Fatalf("base ident = %q, want cfg", cfg.(ast2.ExprIdentExpr).F0)
	}
	rhs := assign.F1
	if rhs.(ast2.ExprStringExpr).F0 != "dark" {
		t.Fatalf("assign rhs = %q, want dark", rhs.(ast2.ExprStringExpr).F0)
	}
}

func TestParseAssignInBlock(t *testing.T) {
	fn := parseSingleFunc(t, `package sample

func foo() -> Int
  var x: Int = 1
  var y: Int = 2
  x = y
  x + y
end
`)

	body := fn.F4.(ast2.ExprBlockExpr)
	if len(body.F0) != 4 {
		t.Fatalf("body expr count = %d, want 4", len(body.F0))
	}
	_, ok := body.F0[2].(ast2.StmtAssignStmt)
	if !ok {
		t.Fatalf("third stmt = %T, want StmtAssignStmt", body.F0[2])
	}
}
