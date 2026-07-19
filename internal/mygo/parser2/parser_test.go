package parser2

import (
	"testing"

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
	ok, yes := got.(ResultOk[File, string])
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

	body := fn.F4.(ExprBlockExpr)
	if len(body.F0) != 1 {
		t.Fatalf("body expr count = %d, want 1", len(body.F0))
	}
	root := body.F0[0].(ExprBinaryExpr)
	if root.F0 != "-" {
		t.Fatalf("root op = %q, want -", root.F0)
	}
	left := (*root.F1).(ExprBinaryExpr)
	if left.F0 != "+" {
		t.Fatalf("left op = %q, want +", left.F0)
	}
	rightMul := (*left.F2).(ExprBinaryExpr)
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

	body := fn.F4.(ExprBlockExpr)
	root := body.F0[0].(ExprBinaryExpr)
	if root.F0 != "||" {
		t.Fatalf("root op = %q, want ||", root.F0)
	}
	left := (*root.F1).(ExprUnaryExpr)
	if left.F0 != "!" {
		t.Fatalf("left unary op = %q, want !", left.F0)
	}
	right := (*root.F2).(ExprBinaryExpr)
	if right.F0 != "&&" {
		t.Fatalf("right op = %q, want &&", right.F0)
	}
	cmp := (*right.F2).(ExprBinaryExpr)
	if cmp.F0 != ">" {
		t.Fatalf("comparison op = %q, want >", cmp.F0)
	}
	neg := (*cmp.F1).(ExprUnaryExpr)
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

	body := fn.F4.(ExprBlockExpr)
	root := body.F0[0].(ExprIfExpr)
	thenExpr := (*root.F1).(ExprNumberExpr)
	if thenExpr.F0 != "1" {
		t.Fatalf("then value = %q, want 1", thenExpr.F0)
	}
	nested := (*root.F2).(ExprIfExpr)
	nestedThen := (*nested.F1).(ExprNumberExpr)
	if nestedThen.F0 != "2" {
		t.Fatalf("elsif value = %q, want 2", nestedThen.F0)
	}
	elseExpr := (*nested.F2).(ExprNumberExpr)
	if elseExpr.F0 != "3" {
		t.Fatalf("else value = %q, want 3", elseExpr.F0)
	}
}

func parseSingleFunc(t *testing.T, src string) DeclFuncDecl {
	t.Helper()

	got := ParseFile(src)
	ok, yes := got.(ResultOk[File, string])
	if !yes {
		t.Fatalf("ParseFile failed: %v", got)
	}
	if len(ok.F0.Decls) != 1 {
		t.Fatalf("decl count = %d, want 1", len(ok.F0.Decls))
	}
	fn, yes := ok.F0.Decls[0].(DeclFuncDecl)
	if !yes {
		t.Fatalf("decl type = %T, want DeclFuncDecl", ok.F0.Decls[0])
	}
	return fn
}
