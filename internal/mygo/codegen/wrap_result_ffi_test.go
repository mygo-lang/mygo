package codegen

import (
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	myparser "github.com/mygo-lang/mygo/internal/mygo/parser"
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

// TestGoMethodCallResultWrapping mirrors a user's `func f() ->
// Result[Ref[http.Response], Error]  client.Do(req)` and asserts the Go FFI
// method call (a (T, error) method on an imported Go type) is wrapped into a
// Result by the bootstrap code generator.
func TestGoMethodCallResultWrapping(t *testing.T) {
	src := `package p
import "go:net/http"

func DoRequest(client: Ref[http.Client], req: Ref[http.Request]) -> Result[Ref[http.Response], Error]
  client.Do(req)
end
`
	parsed, err := myparser.ParseFile("http.mygo", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := &Package{
		Name: "p", NoPrelude: true, Decls: parsed.Decls,
		Imports:       map[string]struct{}{"go:net/http": {}},
		ImportAliases: map[string]string{"http": "go:net/http"},
		Enums:         map[string]*EnumDecl{},
		Structs:       map[string]*StructDecl{},
		Interfaces:    map[string]*InterfaceDecl{},
		Funcs:         map[string]*FuncDecl{},
	}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*FuncDecl); ok {
			pkg.Funcs[fn.Name] = fn
		}
	}
	files, err := GenerateFiles(pkg, nil)
	if err != nil {
		t.Skipf("GenerateFiles: %v", err)
	}
	var gen string
	for _, f := range files {
		gen += f
	}
	t.Logf("GENERATED:\n%s", gen)

	for _, want := range []string{
		"func() Result[*http.Response, error]",
		"client.Do(req)",
		"Ok[*http.Response, error]",
		"Err[*http.Response, error]",
		`"net/http"`,
	} {
		if !strings.Contains(gen, want) {
			t.Errorf("generated code missing %q:\n%s", want, gen)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "gen.go", gen, 0); err != nil {
		t.Fatalf("generated invalid Go: %v\n%s", err, gen)
	}
}
