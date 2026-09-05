package codegen2

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
	. "github.com/mygo-lang/mygo/prelude"
)

func generateWithGoPackages(t *testing.T, src string, pkgs []typeinference2.GoPackageEntry) string {
	t.Helper()
	parsed := parseSourceAsAst2(src)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("parseSourceAsAst2 failed: %v", parsed)
	}
	fileWithIDs := ast2.AssignFileExprIDs(file.F0)
	path := "ffi-boundary.mygo"
	infoResult := typeinference2.InferPackageWithGoPackages(
		[]typeinference2.PkgDeclSource{{Path: path, Decls: fileWithIDs.Decls}},
		pkgs,
	)
	info, ok := infoResult.(Result__Ok[typeinference2.PackageInfo, string])
	if !ok {
		t.Fatalf("inference failed: %v", infoResult)
	}
	generated := GenerateFiles([]SourceFileInput{{Path: path, File: fileWithIDs}}, info.F0)
	res, ok := generated.(Result__Ok[map[string]string, string])
	if !ok {
		t.Fatalf("GenerateFiles failed: %v", generated)
	}
	return res.F0[sourceToGenName(path)]
}

func httpFixturePkg() typeinference2.GoPackageEntry {
	return typeinference2.GoPackageEntry{
		Alias: "http", Path: "go:net/http",
		Types: []typeinference2.GoTypeSignature{
			{TypeName: "Client", Methods: []typeinference2.GoFuncSignature{}, Fields: []typeinference2.GoFieldSignature{
				{Name: "Timeout", Type: "time.Duration"},
			}, Underlying: "struct{Timeout time.Duration}"},
			{TypeName: "Request", Methods: []typeinference2.GoFuncSignature{}, Fields: []typeinference2.GoFieldSignature{
				{Name: "Header", Type: "http.Header"},
			}, Underlying: "struct{Header http.Header}"},
			{TypeName: "Header", Methods: []typeinference2.GoFuncSignature{
				{Name: "Set", Params: []string{"string", "string"}, Results: []string{}, Variadic: false, TypeParams: []string{}},
			}, Fields: []typeinference2.GoFieldSignature{}, Underlying: "map[string][]string"},
		},
	}
}

func contextFixturePkg() typeinference2.GoPackageEntry {
	return typeinference2.GoPackageEntry{
		Alias: "context", Path: "go:context",
		Funcs: []typeinference2.GoFuncSignature{
			{Name: "Background", Params: []string{}, Results: []string{"context.Context"}, Variadic: false, TypeParams: []string{}},
			{Name: "WithTimeout", Params: []string{"context.Context", "time.Duration"}, Results: []string{"context.Context", "context.CancelFunc"}, Variadic: false, TypeParams: []string{}},
		},
		Types: []typeinference2.GoTypeSignature{
			{TypeName: "Context", Methods: []typeinference2.GoFuncSignature{}, Fields: []typeinference2.GoFieldSignature{}, Underlying: "interface{}"},
			{TypeName: "CancelFunc", Methods: []typeinference2.GoFuncSignature{}, Fields: []typeinference2.GoFieldSignature{}, Underlying: "func()"},
		},
	}
}

func timeFixturePkg() typeinference2.GoPackageEntry {
	return typeinference2.GoPackageEntry{
		Alias: "time", Path: "go:time",
		Funcs: []typeinference2.GoFuncSignature{
			{Name: "Second", Params: []string{}, Results: []string{"time.Duration"}, Variadic: false, TypeParams: []string{}},
		},
		Types: []typeinference2.GoTypeSignature{
			{TypeName: "Duration", Methods: []typeinference2.GoFuncSignature{}, Fields: []typeinference2.GoFieldSignature{}, Underlying: "int64"},
		},
	}
}

// TestGenerateFilesChainedFFIMethodCall pins the Go-FFI field + method chain
// (r.Header.Set): inference accepts it and codegen emits a direct Go selector
// call on the field value.
func TestGenerateFilesChainedFFIMethodCall(t *testing.T) {
	src := `package sample

import http "go:net/http"

func use(r: Ref[http.Request]) -> ()
  r.Header.Set("X-Test", "1")
  ()
end
`
	code := generateWithGoPackages(t, src, []typeinference2.GoPackageEntry{httpFixturePkg()})
	if !strings.Contains(code, `r.Header.Set("X-Test", "1")`) {
		t.Fatalf("chained FFI method call lost its direct selector:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "ffi-boundary.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

// TestGenerateFilesPreservesFFIMultiReturnWithPackageLocalTypes pins a tuple
// destructuring binding against a Go FFI call whose results include a
// package-local named type keeping the native multi-value assignment.
func TestGenerateFilesPreservesFFIMultiReturnWithPackageLocalTypes(t *testing.T) {
	src := `package sample

import context "go:context"
import time "go:time"

func use(dur: time.Duration) -> ()
  let (ctx, cancel) = context.WithTimeout(context.Background(), dur)
  ctx
  cancel
  ()
end
`
	code := generateWithGoPackages(t, src, []typeinference2.GoPackageEntry{timeFixturePkg(), contextFixturePkg()})
	if !strings.Contains(code, "ctx, cancel := context.WithTimeout(context.Background(), dur)") {
		t.Fatalf("tuple binding did not preserve raw multi-return:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "ffi-boundary.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

// TestGenerateFilesPreservesFFIMultiReturnBarePackageLocalType pins the same
// behavior for signatures whose package-local result type is spelled without a
// qualifier (the shape that previously fell back to an anonymous tuple).
func TestGenerateFilesPreservesFFIMultiReturnBarePackageLocalType(t *testing.T) {
	ctx := contextFixturePkg()
	ctx.Funcs = []typeinference2.GoFuncSignature{
		{Name: "Background", Params: []string{}, Results: []string{"context.Context"}, Variadic: false, TypeParams: []string{}},
		{Name: "WithTimeout", Params: []string{"context.Context", "time.Duration"}, Results: []string{"context.Context", "CancelFunc"}, Variadic: false, TypeParams: []string{}},
	}
	src := `package sample

import context "go:context"
import time "go:time"

func use(dur: time.Duration) -> ()
  let (ctx, cancel) = context.WithTimeout(context.Background(), dur)
  ctx
  cancel()
  ()
end
`
	code := generateWithGoPackages(t, src, []typeinference2.GoPackageEntry{timeFixturePkg(), ctx})
	if !strings.Contains(code, "ctx, cancel := context.WithTimeout(context.Background(), dur)") {
		t.Fatalf("tuple binding with bare package-local result did not preserve raw multi-return:\n%s", code)
	}
	if !strings.Contains(code, "cancel()") {
		t.Fatalf("Go named func type value was not called directly:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "ffi-boundary.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

// TestGenerateFilesCallsGoFuncTypeValue pins the end-to-end call of a value
// whose Go type is a named function type (context.CancelFunc).
func TestGenerateFilesCallsGoFuncTypeValue(t *testing.T) {
	src := `package sample

import context "go:context"
import time "go:time"

func use(dur: time.Duration) -> ()
  let (_, cancel) = context.WithTimeout(context.Background(), dur)
  cancel()
  ()
end
`
	code := generateWithGoPackages(t, src, []typeinference2.GoPackageEntry{timeFixturePkg(), contextFixturePkg()})
	if !strings.Contains(code, "cancel()") {
		t.Fatalf("cancel() was not emitted as a direct call:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "ffi-boundary.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

// TestGenerateSourceQualifiedFFIStructLit pins the qualified Go-FFI struct
// literal spelling (http.Client{...}), both empty and with a field.
func TestGenerateSourceQualifiedFFIStructLit(t *testing.T) {
	src := `package sample

import http "go:net/http"
import time "go:time"

func makeEmpty() -> Ref[http.Client]
  let bare = http.Client { }
  let withField = http.Client { Timeout: time.Second() }
  Ref.new(if true then withField else bare end)
end
`
	code := generateWithGoPackages(t, src, []typeinference2.GoPackageEntry{httpFixturePkg(), timeFixturePkg()})
	if !strings.Contains(code, "http.Client{}") || !strings.Contains(code, "http.Client{Timeout: time.Second()}") {
		t.Fatalf("qualified struct literal lost its package qualifier:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "ffi-boundary.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

// TestGenerateSourceFFIMethodCallDirect pins direct Go-FFI method calls on a
// Ref receiver staying a Go selector call.
func TestGenerateSourceFFIMethodCallDirect(t *testing.T) {
	src := `package sample

import http "go:net/http"

func use(h: Ref[http.Header]) -> ()
  h.Set("X-Test", "1")
  ()
end
`
	code := generateWithGoPackages(t, src, []typeinference2.GoPackageEntry{httpFixturePkg()})
	if !strings.Contains(code, `h.Set("X-Test", "1")`) {
		t.Fatalf("direct FFI method call was mangled:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "ffi-boundary.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func osFixturePkg() typeinference2.GoPackageEntry {
	return typeinference2.GoPackageEntry{
		Alias: "os", Path: "go:os",
		Funcs: []typeinference2.GoFuncSignature{
			{Name: "Chdir", Params: []string{"string"}, Results: []string{"error"}, Variadic: false, TypeParams: []string{}},
		},
		Types: []typeinference2.GoTypeSignature{},
	}
}

// TestGenerateFilesLoneErrorFFIResultWrap pins a Go FFI call whose signature
// is a lone trailing error (os.Chdir -> error) lowering to Result[(), error]:
// the generated Go binds a single error, checks it, and produces Err with the
// error on non-nil or Ok with a struct{}{} unit payload on nil.
func TestGenerateFilesLoneErrorFFIResultWrap(t *testing.T) {
	src := `package sample

import os "go:os"

func run() -> Result[(), Error]
  os.Chdir("/tmp")
end
`
	code := generateWithGoPackages(t, src, []typeinference2.GoPackageEntry{osFixturePkg()})
	if !strings.Contains(code, `os.Chdir("/tmp")`) {
		t.Fatalf("lone-error FFI call missing direct call:\n%s", code)
	}
	if !strings.Contains(code, `!= nil`) {
		t.Fatalf("lone-error FFI call missing the error nil check:\n%s", code)
	}
	compact := strings.Join(strings.Fields(code), "")
	if !strings.Contains(compact, "struct{}{}") {
		t.Fatalf("lone-error FFI Ok arm did not synthesize a struct{}{} unit payload:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "ffi-boundary.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

// TestGenerateFilesLoneErrorFFIStatementDiscard pins a lone-error Go FFI call
// used only for its side effect emitting the raw call as a statement rather
// than building an unused Result value.
func TestGenerateFilesLoneErrorFFIStatementDiscard(t *testing.T) {
	src := `package sample

import os "go:os"

func run() -> ()
  os.Chdir("/tmp")
  ()
end
`
	code := generateWithGoPackages(t, src, []typeinference2.GoPackageEntry{osFixturePkg()})
	if !strings.Contains(code, `os.Chdir("/tmp")`) {
		t.Fatalf("lone-error FFI statement call missing direct call:\n%s", code)
	}
	compact := strings.Join(strings.Fields(code), "")
	if strings.Contains(compact, "struct{}{}") {
		t.Fatalf("lone-error FFI statement call was wrapped into a Result instead of discarded:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "ffi-boundary.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func serverFixturePkg() typeinference2.GoPackageEntry {
	return typeinference2.GoPackageEntry{
		Alias: "http", Path: "go:net/http",
		Types: []typeinference2.GoTypeSignature{
			{TypeName: "Server", Methods: []typeinference2.GoFuncSignature{
				{Name: "ListenAndServe", Params: []string{}, Results: []string{"error"}, Variadic: false, TypeParams: []string{}},
			}, Fields: []typeinference2.GoFieldSignature{
				{Name: "Addr", Type: "string"},
			}, Underlying: "struct{Addr string}"},
		},
	}
}

// TestGenerateFilesLoneErrorMethodOnLocalResult pins a lone-error Go FFI
// method call on a receiver bound from a local struct literal
// (`let srv = http.Server { ... }; srv.ListenAndServe()`) lowering to
// Result[(), error] exactly like a package-level lone-error function.
func TestGenerateFilesLoneErrorMethodOnLocalResult(t *testing.T) {
	src := `package sample

import http "go:net/http"

func RunServer(addr: string) -> Result[(), Error]
  let srv = http.Server { Addr: addr }
  srv.ListenAndServe()
end
`
	code := generateWithGoPackages(t, src, []typeinference2.GoPackageEntry{serverFixturePkg()})
	if _, err := parser.ParseFile(token.NewFileSet(), "ffi-boundary.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
	for _, want := range []string{
		`:= srv.ListenAndServe()`,
		`!= nil`,
		`Ok[struct{}, error]`,
		`Err[struct{}, error]`,
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("lone-error method on local receiver missing %q:\n%s", want, code)
		}
	}
}

// TestGenerateFilesLoneErrorMethodSwitchScrutinee pins the switch-pattern-match
// form: `switch srv.ListenAndServe()` (a lone-error method on a local struct
// literal receiver) must wrap the subject at the Go FFI boundary into
// Result[(), error] and pattern-match that same Result type, so the generated
// Go compiles and the Ok/Err arms are reached at runtime.
func TestGenerateFilesLoneErrorMethodSwitchScrutinee(t *testing.T) {
	src := `package sample

import http "go:net/http"

func RunServer(addr: string) -> Result[(), String]
  let srv = http.Server { Addr: addr }
  switch srv.ListenAndServe()
    case Ok(_) => Ok(())
    case Err(e) => Err(e.Error())
  end
end
`
	code := generateWithGoPackages(t, src, []typeinference2.GoPackageEntry{serverFixturePkg()})
	if _, err := parser.ParseFile(token.NewFileSet(), "ffi-boundary.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
	for _, want := range []string{
		`:= srv.ListenAndServe()`,
		`!= nil`,
		`Ok[struct{}, error]`,
		`Err[struct{}, error]`,
		`Result__Ok[struct{}, error]`,
		`Result__Err[struct{}, error]`,
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("lone-error switch scrutinee missing %q:\n%s", want, code)
		}
	}
}
