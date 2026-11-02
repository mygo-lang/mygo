package codegen2

import (
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	goast "github.com/mygo-lang/mygo/internal/mygo/codegen2/goast"
)

func TestTranslateDynamicBoxCallsRecordedConstructor(t *testing.T) {
	value := ast2.IdentExprNode("widget")
	ctx := newEgCtx()
	translated := translateDynamicBoxAst(value, "example.com/contracts", "Show", "sha256:impl", "MygoDynamicShowFromBackendWidget", &ctx)
	result, ok := resultOk[AstExprResult, string](translated)
	if !ok {
		t.Fatalf("translateDynamicBoxAst failed: %v", translated)
	}
	if got := goast.MustExprSource(result.F0.Expr); got != "MygoDynamicShowFromBackendWidget(widget)" {
		t.Fatalf("generated boxing expression = %q", got)
	}
}

func TestTranslateDynamicBoxRequiresConstructorEvidence(t *testing.T) {
	value := ast2.IdentExprNode("widget")
	ctx := newEgCtx()
	translated := translateDynamicBoxAst(value, "example.com/contracts", "Show", "sha256:impl", "", &ctx)
	if _, ok := resultErr[AstExprResult, string](translated); !ok {
		t.Fatalf("translateDynamicBoxAst without constructor = %T, want diagnostic", translated)
	}
}
