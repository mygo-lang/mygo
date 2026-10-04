package codegen2

import (
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	goast "github.com/mygo-lang/mygo/internal/mygo/codegen2/goast"
	. "github.com/mygo-lang/mygo/prelude"
)

func TestTranslateDynamicBoxCallsRecordedConstructor(t *testing.T) {
	value := ast2.Expr{Kind: ast2.ExprKind__IdentExpr__Ctor("widget")}
	ctx := newEgCtx()
	translated := translateDynamicBoxAst(value, "example.com/contracts", "Show", "sha256:impl", "MygoDynamicShowFromBackendWidget", &ctx)
	result, ok := translated.(Result__Ok[AstExprResult, string])
	if !ok {
		t.Fatalf("translateDynamicBoxAst failed: %v", translated)
	}
	if got := goast.MustExprSource(result.F0.Expr); got != "MygoDynamicShowFromBackendWidget(widget)" {
		t.Fatalf("generated boxing expression = %q", got)
	}
}

func TestTranslateDynamicBoxRequiresConstructorEvidence(t *testing.T) {
	value := ast2.Expr{Kind: ast2.ExprKind__IdentExpr__Ctor("widget")}
	ctx := newEgCtx()
	translated := translateDynamicBoxAst(value, "example.com/contracts", "Show", "sha256:impl", "", &ctx)
	if _, ok := translated.(Result__Err[AstExprResult, string]); !ok {
		t.Fatalf("translateDynamicBoxAst without constructor = %T, want diagnostic", translated)
	}
}
