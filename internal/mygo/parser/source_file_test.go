package parser

import "testing"

func TestParseFileSetsSourceFileRecursively(t *testing.T) {
	const filename = "sample.mygo"
	const src = `package main

func main() -> Int
  let x = 1
  x
end
`
	f, err := ParseFile(filename, src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if len(f.Decls) != 1 {
		t.Fatalf("len(Decls) = %d, want 1", len(f.Decls))
	}
	fn, ok := f.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("Decls[0] = %T, want *FuncDecl", f.Decls[0])
	}
	if fn.SourceFile != filename {
		t.Fatalf("FuncDecl.SourceFile = %q, want %q", fn.SourceFile, filename)
	}
	body, ok := fn.Body.(*BlockExpr)
	if !ok {
		t.Fatalf("Body = %T, want *BlockExpr", fn.Body)
	}
	if body.SourceFile != filename {
		t.Fatalf("BlockExpr.SourceFile = %q, want %q", body.SourceFile, filename)
	}
	let, ok := body.Stmts[0].(*LetStmt)
	if !ok {
		t.Fatalf("Body.Stmts[0] = %T, want *LetStmt", body.Stmts[0])
	}
	if let.SourceFile != filename {
		t.Fatalf("LetStmt.SourceFile = %q, want %q", let.SourceFile, filename)
	}
	if let.Value.(*LiteralExpr).SourceFile != filename {
		t.Fatalf("LiteralExpr.SourceFile = %q, want %q", let.Value.(*LiteralExpr).SourceFile, filename)
	}
}
