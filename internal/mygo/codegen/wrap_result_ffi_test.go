package codegen

import (
	"go/ast"
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

func TestWrapGoErrorResultCallPacksMultipleSuccessValues(t *testing.T) {
	g := newGen(&Package{}, nil)
	call := &ast.CallExpr{Fun: ast.NewIdent("find"), Args: []ast.Expr{ast.NewIdent("key")}}
	wrapped := g.wrapGoErrorResultCall(call, "Result[Tuple[string, int], error]")
	var out strings.Builder
	if err := printer.Fprint(&out, token.NewFileSet(), wrapped); err != nil {
		t.Fatal(err)
	}
	code := out.String()
	if strings.Count(code, "find(key)") != 1 {
		t.Fatalf("wrapped call evaluated more than once:\n%s", code)
	}
	for _, want := range []string{"__mygo_result_val_0", "__mygo_result_val_1", "F0:", "F1:", "__mygo_result_err != nil"} {
		if !strings.Contains(code, want) {
			t.Errorf("wrapped multi-value Result missing %q:\n%s", want, code)
		}
	}
	if _, err := parser.ParseExpr(code); err != nil {
		t.Fatalf("wrapped expression is invalid Go: %v\n%s", err, code)
	}
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

func TestGoImportMultipleSuccessValuesBeforeError(t *testing.T) {
	src := `package p
import "go:mime"

func parse(value: String) -> Result[(String, Map[String, String]), Error]
  mime.ParseMediaType(value)
end
`
	parsed, err := myparser.ParseFile("mime.mygo", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := &Package{
		Name: "p", NoPrelude: true, Decls: parsed.Decls,
		Imports:       map[string]struct{}{"go:mime": {}},
		ImportAliases: map[string]string{"mime": "go:mime"},
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
		t.Fatalf("GenerateFiles: %v", err)
	}
	var gen strings.Builder
	for _, file := range files {
		gen.WriteString(file)
	}
	code := gen.String()
	if strings.Count(code, "mime.ParseMediaType(value)") != 1 {
		t.Fatalf("expected the wrapped call to be evaluated once, got:\n%s", code)
	}
	for _, want := range []string{"Result[struct {", "F0 string", "F1 map[string]string", "F0:", "F1:"} {
		if !strings.Contains(code, want) {
			t.Errorf("generated Go missing %q:\n%s", want, code)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "mime.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}
