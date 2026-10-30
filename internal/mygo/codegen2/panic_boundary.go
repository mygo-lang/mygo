package codegen2

import (
	"fmt"

	"github.com/mygo-lang/mygo/internal/mygo/abi"
	ast2 "github.com/mygo-lang/mygo/internal/mygo/ast2"
	goast "github.com/mygo-lang/mygo/internal/mygo/codegen2/goast"
	"github.com/mygo-lang/mygo/prelude"
)

// Panic protection for AST generation. `recover` has to run in a deferred Go
// frame, and the recovered error has to be wrapped into a MyGO Result. Both
// steps live here, on the Go side, so the wrapper never names an ADT shape and
// works for the legacy and canonical representations alike.

func protectStatementRun(
	expr ast2.Expr,
	work func() prelude.Result[[]goast.Stmt, ast2.Diagnostic],
) (result prelude.Result[[]goast.Stmt, ast2.Diagnostic]) {
	defer func() {
		if cause := recover(); cause != nil {
			result = abi.ErrOf[[]goast.Stmt, ast2.Diagnostic](
				expressionPanicDiagnostic(expr, fmt.Sprint(cause)),
			)
		}
	}()
	return work()
}

func protectExpressionRun(
	expr ast2.Expr,
	ctx *egCtx,
	expected ast2.MonoType,
) (result prelude.Result[AstExprResult, ast2.Diagnostic]) {
	defer func() {
		if cause := recover(); cause != nil {
			result = abi.ErrOf[AstExprResult, ast2.Diagnostic](
				expressionPanicDiagnostic(expr, fmt.Sprint(cause)),
			)
		}
	}()
	return translateExprAstExpectedInner(expr, ctx, expected)
}

func protectDeclRun(
	g *Generator2,
	decls []ast2.Decl,
	index int,
	out []goast.Decl,
) (result prelude.Result[[]goast.Decl, ast2.Diagnostic]) {
	defer func() {
		if cause := recover(); cause != nil {
			result = abi.ErrOf[[]goast.Decl, ast2.Diagnostic](
				ast2.NewDiagnostic("generate", "go-ast-declaration", "internal", fmt.Sprint(cause)),
			)
		}
	}()
	return translateDeclsAstInner(g, decls, index, out)
}
