package parser2

import (
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	. "github.com/mygo-lang/mygo/prelude"
)

// structLiteralPostfixFixture exercises a struct literal used directly as the
// head of a postfix chain.  A struct literal freezes into its own node, so the
// parser must keep reading the chain instead of ending the expression at the
// literal.
const structLiteralPostfixFixture = `package sample

struct Point
  x: Int
  y: Int
end

struct Box[A]
  value: A
end

impl Point
  func Sum(self: Point) -> Int
    self.x + self.y
  end
end

impl[A] Box[A]
  func Value(self: Box[A]) -> A
    self.value
  end
end

func sink(v: Int) -> Int
  v
end

func useReturn() -> Int
  Point { x: 1, y: 2 }.Sum()
end

func useArgument() -> Int
  sink(Point { x: 3, y: 4 }.Sum())
end

func useGeneric() -> Int
  Box[Int] { value: 42 }.Value()
end

func useField() -> Int
  Point { x: 5, y: 6 }.x
end
`

func parseStructLiteralPostfixFixture(t *testing.T) ast2.File {
	t.Helper()
	tree := ParseSyntaxAt("structlit.mygo", structLiteralPostfixFixture)
	if len(tree.Diagnostics) != 0 {
		t.Fatalf("fixture has syntax diagnostics: %+v", tree.Diagnostics)
	}
	if got := cstNodeText(*tree.Root); got != structLiteralPostfixFixture {
		t.Fatalf("CST is not lossless: %q", got)
	}
	file, ok := LowerSyntax(tree).(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("lowering failed: %v", LowerSyntax(tree))
	}
	return file.F0
}

func functionBodyExpr(t *testing.T, file ast2.File, name string) ast2.Expr {
	t.Helper()
	for _, decl := range file.Decls {
		fn, ok := decl.(ast2.Decl__FuncDecl)
		if !ok || fn.F0 != name {
			continue
		}
		block, ok := fn.F4.Kind.(ast2.ExprKind__BlockExpr)
		if !ok || len(block.F0) != 1 {
			t.Fatalf("%s body is not a single-statement block: %v", name, fn.F4.Kind)
		}
		stmt, ok := block.F0[0].(ast2.Stmt__ExprStmt)
		if !ok {
			t.Fatalf("%s statement is not an expression: %v", name, block.F0[0])
		}
		return stmt.F0
	}
	t.Fatalf("function %s not found", name)
	return ast2.Expr{}
}

func TestStructLiteralPostfixChainParsesAsMethodCall(t *testing.T) {
	file := parseStructLiteralPostfixFixture(t)

	// Return position: the call node must span the literal and its suffix.
	expr := functionBodyExpr(t, file, "useReturn")
	call, ok := expr.Kind.(ast2.ExprKind__CallExpr)
	if !ok {
		t.Fatalf("useReturn body is %T, want CallExpr", expr.Kind)
	}
	field, ok := call.F0.Kind.(ast2.ExprKind__FieldExpr)
	if !ok || field.F1 != "Sum" {
		t.Fatalf("callee is %#v, want field Sum", call.F0.Kind)
	}
	if _, ok := field.F0.Kind.(ast2.ExprKind__StructLitExpr); !ok {
		t.Fatalf("receiver is %T, want StructLitExpr", field.F0.Kind)
	}
	if expr.Span.End.Column <= field.F0.Span.End.Column {
		t.Fatalf("call span %v does not extend past the struct literal %v", expr.Span.End, field.F0.Span.End)
	}

	// Argument position: the call on the literal is the inner call.
	argExpr := functionBodyExpr(t, file, "useArgument")
	argCall, ok := argExpr.Kind.(ast2.ExprKind__CallExpr)
	if !ok || len(argCall.F2) != 1 {
		t.Fatalf("useArgument body is %#v, want a one-argument call", argExpr.Kind)
	}
	inner, ok := argCall.F2[0].Kind.(ast2.ExprKind__CallExpr)
	if !ok {
		t.Fatalf("argument is %T, want CallExpr", argCall.F2[0].Kind)
	}
	innerField, ok := inner.F0.Kind.(ast2.ExprKind__FieldExpr)
	if !ok || innerField.F1 != "Sum" {
		t.Fatalf("argument callee is %#v, want field Sum", inner.F0.Kind)
	}
	if _, ok := innerField.F0.Kind.(ast2.ExprKind__StructLitExpr); !ok {
		t.Fatalf("argument receiver is %T, want StructLitExpr", innerField.F0.Kind)
	}
}

func TestStructLiteralPostfixChainGenericAndField(t *testing.T) {
	file := parseStructLiteralPostfixFixture(t)

	genericExpr := functionBodyExpr(t, file, "useGeneric")
	genericCall, ok := genericExpr.Kind.(ast2.ExprKind__CallExpr)
	if !ok {
		t.Fatalf("useGeneric body is %T, want CallExpr", genericExpr.Kind)
	}
	genericField, ok := genericCall.F0.Kind.(ast2.ExprKind__FieldExpr)
	if !ok || genericField.F1 != "Value" {
		t.Fatalf("generic callee is %#v, want field Value", genericCall.F0.Kind)
	}
	if _, ok := genericField.F0.Kind.(ast2.ExprKind__GenericStructLitExpr); !ok {
		t.Fatalf("generic receiver is %T, want GenericStructLitExpr", genericField.F0.Kind)
	}

	fieldExpr := functionBodyExpr(t, file, "useField")
	sel, ok := fieldExpr.Kind.(ast2.ExprKind__FieldExpr)
	if !ok || sel.F1 != "x" {
		t.Fatalf("useField body is %#v, want field x", fieldExpr.Kind)
	}
	if _, ok := sel.F0.Kind.(ast2.ExprKind__StructLitExpr); !ok {
		t.Fatalf("field receiver is %T, want StructLitExpr", sel.F0.Kind)
	}
}
