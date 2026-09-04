package parser

import (
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast"
)

func TestParseFuncLitBodyDoesNotLeakIntoAdditiveCall(t *testing.T) {
	src := `package p

func recurse(variants: Slice[Variant], out: String) -> String
  if out == "" then out else
    let variant = Variant { Name: "", Names: [] as Slice[String] }
    recurse(variants, out + "," + variant.Name + ":" + variant.Names.Fold("", func(names: String, field: String) -> String names + "." + field end))
  end
end
`
	file, err := ParseFile("p.mygo", src)
	if err != nil {
		t.Fatal(err)
	}
	var nameCount, fieldCount int
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			countIdents(fn.Body, "names", &nameCount)
			countIdents(fn.Body, "field", &fieldCount)
		}
	}
	if nameCount != 1 {
		t.Fatalf("names identifier count = %d, want 1 (lambda body leaked outside func literal)", nameCount)
	}
	if fieldCount != 1 {
		t.Fatalf("field identifier count = %d, want 1", fieldCount)
	}
}

func countIdents(e ast.Expr, name string, count *int) {
	if e == nil {
		return
	}
	switch n := e.(type) {
	case *ast.IdentExpr:
		if n.Name == name {
			*count++
		}
	case *ast.CallExpr:
		countIdents(n.Callee, name, count)
		for _, a := range n.Args {
			countIdents(a, name, count)
		}
	case *ast.FieldExpr:
		countIdents(n.Expr, name, count)
	case *ast.BinaryExpr:
		countIdents(n.Left, name, count)
		countIdents(n.Right, name, count)
	case *ast.FuncLitExpr:
		countIdents(n.Body, name, count)
	case *ast.BlockExpr:
		for _, stmt := range n.Stmts {
			switch s := stmt.(type) {
			case *ast.ExprStmt:
				countIdents(s.Expr, name, count)
			case *ast.LetStmt:
				countIdents(s.Value, name, count)
			}
		}
	case *ast.StructLitExpr:
		for _, f := range n.Fields {
			countIdents(f.Value, name, count)
		}
	case *ast.CastExpr:
		countIdents(n.Expr, name, count)
	case *ast.SliceLitExpr:
		for _, a := range n.Elems {
			countIdents(a, name, count)
		}
	case *ast.IfExpr:
		countIdents(n.Cond, name, count)
		countIdents(n.Then, name, count)
		countIdents(n.Else, name, count)
	}
}
