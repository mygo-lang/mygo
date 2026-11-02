package parser2

import (
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
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
	file, ok := resultOk[ast2.File, string](LowerSyntax(tree))
	if !ok {
		t.Fatalf("lowering failed: %v", LowerSyntax(tree))
	}
	return file.F0
}

func functionBodyExpr(t *testing.T, file ast2.File, name string) ast2.Expr {
	t.Helper()
	for _, decl := range file.Decls {
		if !ast2.DeclIsFunc(decl) || ast2.DeclFuncName(decl) != name {
			continue
		}
		body := ast2.DeclFuncBody(decl)
		if !ast2.ExprIsBlock(body) || len(ast2.ExprBlockStatements(body)) != 1 {
			t.Fatalf("%s body is not a single-statement block: %#v", name, body)
		}
		statement := ast2.ExprBlockStatements(body)[0]
		if !ast2.StmtIsExpr(statement) {
			t.Fatalf("%s statement is not an expression: %#v", name, statement)
		}
		return ast2.StmtExpr(statement)
	}
	t.Fatalf("function %s not found", name)
	return ast2.Expr{}
}

func TestStructLiteralPostfixChainParsesAsMethodCall(t *testing.T) {
	file := parseStructLiteralPostfixFixture(t)

	// Return position: the call node must span the literal and its suffix.
	expr := functionBodyExpr(t, file, "useReturn")
	if !ast2.ExprIsCall(expr) {
		t.Fatalf("useReturn body is not a call: %#v", expr)
	}
	call := ast2.ExprCall(expr)
	if !ast2.ExprIsField(call.Callee) || ast2.ExprField(call.Callee).Name != "Sum" {
		t.Fatalf("callee is %#v, want field Sum", call.Callee)
	}
	field := ast2.ExprField(call.Callee)
	if !ast2.ExprIsStructLit(field.Receiver) {
		t.Fatalf("receiver is not a struct literal: %#v", field.Receiver)
	}
	if expr.Span.End.Column <= field.Receiver.Span.End.Column {
		t.Fatalf("call span %v does not extend past the struct literal %v", expr.Span.End, field.Receiver.Span.End)
	}

	// Argument position: the call on the literal is the inner call.
	argExpr := functionBodyExpr(t, file, "useArgument")
	if !ast2.ExprIsCall(argExpr) {
		t.Fatalf("useArgument body is not a call: %#v", argExpr)
	}
	argCall := ast2.ExprCall(argExpr)
	if len(argCall.Args) != 1 || !ast2.ExprIsCall(argCall.Args[0]) {
		t.Fatalf("useArgument body is %#v, want a one-argument call containing a call", argExpr)
	}
	inner := ast2.ExprCall(argCall.Args[0])
	if !ast2.ExprIsField(inner.Callee) || ast2.ExprField(inner.Callee).Name != "Sum" {
		t.Fatalf("argument callee is %#v, want field Sum", inner.Callee)
	}
	innerField := ast2.ExprField(inner.Callee)
	if !ast2.ExprIsStructLit(innerField.Receiver) {
		t.Fatalf("argument receiver is not a struct literal: %#v", innerField.Receiver)
	}
}

func TestStructLiteralPostfixChainGenericAndField(t *testing.T) {
	file := parseStructLiteralPostfixFixture(t)

	genericExpr := functionBodyExpr(t, file, "useGeneric")
	if !ast2.ExprIsCall(genericExpr) {
		t.Fatalf("useGeneric body is not a call: %#v", genericExpr)
	}
	genericCall := ast2.ExprCall(genericExpr)
	if !ast2.ExprIsField(genericCall.Callee) || ast2.ExprField(genericCall.Callee).Name != "Value" {
		t.Fatalf("generic callee is %#v, want field Value", genericCall.Callee)
	}
	genericField := ast2.ExprField(genericCall.Callee)
	if !ast2.ExprIsGenericStructLit(genericField.Receiver) {
		t.Fatalf("generic receiver is not a generic struct literal: %#v", genericField.Receiver)
	}

	fieldExpr := functionBodyExpr(t, file, "useField")
	if !ast2.ExprIsField(fieldExpr) {
		t.Fatalf("useField body is not a field expression: %#v", fieldExpr)
	}
	sel := ast2.ExprField(fieldExpr)
	if sel.Name != "x" {
		t.Fatalf("useField body is field %q, want x", sel.Name)
	}
	if !ast2.ExprIsStructLit(sel.Receiver) {
		t.Fatalf("field receiver is not a struct literal: %#v", sel.Receiver)
	}
}
