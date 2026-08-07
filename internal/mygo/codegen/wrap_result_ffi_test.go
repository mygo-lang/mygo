package codegen

import (
	"go/printer"
	"go/token"
	"strings"
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

// TestTranslateGoImportCallResultWrapping exercises translateGoImportCall on a
// real Go FFI function returning `(T, error)` (os.ReadFile) and verifies the
// returned Result wrapping for the `let a = f(...)` binding form.
func TestTranslateGoImportCallResultWrapping(t *testing.T) {
	g := newGen(&Package{}, nil)
	g.importAliases = map[string]string{"myos": "go:os"}
	ctx := &egCtx{
		locals:      map[string]string{},
		bindings:    map[string]string{},
		mutable:     map[string]bool{},
		typeParams:  map[string]struct{}{},
		sourceTypes: map[string]string{},
	}

	arg := []Expr{&LiteralExpr{Kind: "string", Value: `"x"`}}
	node := &FieldExpr{Expr: &IdentExpr{Name: "myos"}, Field: "ReadFile"}

	t.Run("let a = f() passes go-style Result expected", func(t *testing.T) {
		code, _, err := g.translateGoImportCall("myos", "ReadFile", arg, ctx, "Result[[16]byte, error]", node)
		if err != nil {
			t.Fatalf("translateGoImportCall error: %v", err)
		}
		var sb strings.Builder
		if err := printer.Fprint(&sb, token.NewFileSet(), code); err != nil {
			t.Fatal(err)
		}
		out := sb.String()
		if !strings.Contains(out, "func()") {
			t.Fatalf("expected an IIFE Result wrapper, got:\n%s", out)
		}
		if !strings.Contains(out, "myos.ReadFile(") {
			t.Fatalf("expected raw call inside wrapper, got:\n%s", out)
		}
	})

	t.Run("let a = f() with empty expected still wraps", func(t *testing.T) {
		code, typ, err := g.translateGoImportCall("myos", "ReadFile", arg, ctx, "", node)
		if err != nil {
			t.Fatalf("translateGoImportCall error: %v", err)
		}
		if !strings.HasPrefix(typ, "Result[") {
			t.Fatalf("expected a Result[...] return type, got %q", typ)
		}
		var sb strings.Builder
		if err := printer.Fprint(&sb, token.NewFileSet(), code); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(sb.String(), "func()") {
			t.Fatalf("expected an IIFE Result wrapper, got:\n%s", sb.String())
		}
	})
}
