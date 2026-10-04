package ast2

import (
	"testing"

	. "github.com/mygo-lang/mygo/prelude"
)

func TestAssignFileExprIDsPreservesDynamicBoxEvidenceAndSpan(t *testing.T) {
	start := SourcePos{SourceName: "boxing.mygo", Line: 4, Column: 7}
	finish := SourcePos{SourceName: "boxing.mygo", Line: 4, Column: 28}
	span := SourceSpan{Start: start, End: finish}
	value := Expr{Pos: start, Span: span, Kind: ExprKind__IdentExpr__Ctor("widget")}
	box := Expr{
		Pos:  start,
		Span: span,
		Kind: ExprKind__DynamicBoxExpr__Ctor(value, "example.com/contracts", "Show", "sha256:stable", "MygoDynamicShowFromBackendWidget"),
	}
	function := Decl__FuncDecl__Ctor("box", nil, nil, None[TypeExpr](), box, nil)
	assigned := AssignFileExprIDs(File{PackageName: "sample", Decls: []Decl{function}})
	file, ok := assigned.(Option__Some[File])
	if !ok {
		t.Fatalf("AssignFileExprIDs returned %T, want Some", assigned)
	}
	decl, ok := file.F0.Decls[0].(Decl__FuncDecl)
	if !ok {
		t.Fatalf("assigned declaration = %T, want FuncDecl", file.F0.Decls[0])
	}
	boxed, ok := decl.F4.Kind.(ExprKind__DynamicBoxExpr)
	if !ok {
		t.Fatalf("assigned expression = %T, want DynamicBoxExpr", decl.F4.Kind)
	}
	if boxed.F1 != "example.com/contracts" || boxed.F2 != "Show" || boxed.F3 != "sha256:stable" || boxed.F4 != "MygoDynamicShowFromBackendWidget" {
		t.Fatalf("assigned boxing evidence = (%q, %q, %q, %q)", boxed.F1, boxed.F2, boxed.F3, boxed.F4)
	}
	if decl.F4.Span != span {
		t.Fatalf("assignment lost dynamic box source span: got=%+v want=%+v", decl.F4.Span, span)
	}
}
