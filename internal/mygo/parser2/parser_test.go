package parser2

import (
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	. "github.com/mygo-lang/mygo/prelude"
)

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
