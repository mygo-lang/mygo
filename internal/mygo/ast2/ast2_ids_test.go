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

func TestAssignFileExprIDsIncludesCaseGuards(t *testing.T) {
	guard := Expr{Pos: SourcePos{SourceName: "guard.mygo", Line: 4, Column: 20}, Kind: ExprKind__IdentExpr__Ctor("flag")}
	body := Expr{Pos: SourcePos{SourceName: "guard.mygo", Line: 4, Column: 30}, Kind: ExprKind__NumberExpr__Ctor("1")}
	target := Expr{Pos: SourcePos{SourceName: "guard.mygo", Line: 4, Column: 10}, Kind: ExprKind__IdentExpr__Ctor("value")}
	switchExpr := Expr{Kind: ExprKind__SwitchExpr__Ctor(target, []SwitchCase{{
		Pattern: Pattern__WildcardPattern__Ctor(),
		Guard:   Some(guard),
		Body:    body,
	}}, None[MonoType]())}
	function := Decl__FuncDecl__Ctor("f", nil, nil, None[TypeExpr](), switchExpr, nil)
	assigned := AssignFileExprIDs(File{PackageName: "sample", Decls: []Decl{function}})
	file, ok := assigned.(Option__Some[File])
	if !ok {
		t.Fatalf("AssignFileExprIDs returned %T, want Some", assigned)
	}
	decl := file.F0.Decls[0].(Decl__FuncDecl)
	sw := decl.F4.Kind.(ExprKind__SwitchExpr)
	guarded, ok := sw.F1[0].Guard.(Option__Some[Expr])
	if !ok {
		t.Fatalf("guard = %T, want Some", sw.F1[0].Guard)
	}
	if guarded.F0.ID == 0 || sw.F1[0].Body.ID == 0 {
		t.Fatalf("guard/body IDs = %d/%d, want both assigned", guarded.F0.ID, sw.F1[0].Body.ID)
	}
	if guarded.F0.ID == sw.F1[0].Body.ID {
		t.Fatalf("guard and body share ID %d", guarded.F0.ID)
	}
}
