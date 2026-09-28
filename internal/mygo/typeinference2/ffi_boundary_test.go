package typeinference2

import (
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/parser2"
	. "github.com/mygo-lang/mygo/prelude"
)

func mustAssignFileExprIDsForTest(t *testing.T, file ast2.File) ast2.File {
	t.Helper()
	assigned := ast2.AssignFileExprIDs(file)
	if !MygoIN6OptionM6IsSome[ast2.File](assigned) {
		t.Fatal("AssignFileExprIDs returned None")
	}
	return MygoIN6OptionM6Unwrap[ast2.File](assigned)
}

// contextFixture mirrors what the real FFI loader returns for go:context
// (qualified result spellings), plus a bare package-local spelling used by the
// boundary paths under test.
func contextFixture() GoPackageEntry {
	return GoPackageEntry{
		Alias: "context", Path: "go:context",
		Funcs: []GoFuncSignature{
			{Name: "Background", Params: []string{}, Results: []string{"context.Context"}},
			{Name: "WithTimeout", Params: []string{"context.Context", "time.Duration"}, Results: []string{"context.Context", "CancelFunc"}},
		},
		Types: []GoTypeSignature{
			{TypeName: "Context", Methods: []GoFuncSignature{}, Fields: []GoFieldSignature{}, Underlying: "interface{Deadline() (time.Time, bool); Done() <-chan struct{}; Err() error; Value(key any) any}"},
			{TypeName: "CancelFunc", Methods: []GoFuncSignature{}, Fields: []GoFieldSignature{}, Underlying: "func()"},
		},
	}
}

func httpFixture() GoPackageEntry {
	return GoPackageEntry{
		Alias: "http", Path: "go:net/http",
		Types: []GoTypeSignature{
			{TypeName: "Client", Methods: []GoFuncSignature{}, Fields: []GoFieldSignature{
				{Name: "Timeout", Type: "time.Duration"},
			}, Underlying: "struct{Timeout time.Duration}"},
			{TypeName: "Request", Methods: []GoFuncSignature{}, Fields: []GoFieldSignature{
				{Name: "Header", Type: "http.Header"},
			}, Underlying: "struct{Header http.Header}"},
			{TypeName: "Header", Methods: []GoFuncSignature{
				{Name: "Set", Params: []string{"string", "string"}, Results: []string{}, Variadic: false, TypeParams: []string{}},
			}, Fields: []GoFieldSignature{}, Underlying: "map[string][]string"},
		},
	}
}

// TestGoTypeNameWithPackageNamedFuncType pins that a Go named type whose
// underlying is a function signature resolves to a callable TFunc, while
// non-callable named types keep their qualified identity.
func TestGoTypeNameWithPackageNamedFuncType(t *testing.T) {
	cancelResult := GoTypeNameWithPackage("CancelFunc", contextFixture())
	cancelFunc, ok := cancelResult.(Result__Ok[ast2.MonoType, string])
	if !ok {
		t.Fatalf("GoTypeNameWithPackage(CancelFunc) failed: %v", cancelResult)
	}
	if got := MonoStringFull(cancelFunc.F0); got != "func() -> ()" {
		t.Fatalf("CancelFunc resolved to %q, want \"func() -> ()\"", got)
	}
	contextResult := GoTypeNameWithPackage("Context", contextFixture())
	context, ok := contextResult.(Result__Ok[ast2.MonoType, string])
	if !ok {
		t.Fatalf("GoTypeNameWithPackage(Context) failed: %v", contextResult)
	}
	if got := MonoStringFull(context.F0); got != "go:context.Context" {
		t.Fatalf("Context resolved to %q, want go:context.Context", got)
	}
	clientResult := GoTypeNameWithPackage("Client", httpFixture())
	client, ok := clientResult.(Result__Ok[ast2.MonoType, string])
	if !ok {
		t.Fatalf("GoTypeNameWithPackage(Client) failed: %v", clientResult)
	}
	if got := MonoStringFull(client.F0); got != "go:net/http.Client" {
		t.Fatalf("Client resolved to %q, want go:net/http.Client", got)
	}
}

// TestGoSignatureRawResultTypeWithPackage pins the package-aware raw result
// shape: a bare package-local result name (CancelFunc) resolves with package
// context instead of the plain entry point's unresolved-name error, and the
// qualified spelling keeps working too.
func TestGoSignatureRawResultTypeWithPackage(t *testing.T) {
	withTimeout := GoFuncSignature{
		Name: "WithTimeout", Params: []string{"context.Context", "time.Duration"},
		Results: []string{"context.Context", "CancelFunc"}, Variadic: false, TypeParams: []string{},
	}
	rawResult := GoSignatureRawResultTypeWithPackage(withTimeout, contextFixture())
	raw, ok := rawResult.(Result__Ok[ast2.MonoType, string])
	if !ok {
		t.Fatalf("GoSignatureRawResultTypeWithPackage failed: %v", rawResult)
	}
	if got := MonoStringFull(raw.F0); got != "tuple(go:context.Context, func() -> ())" {
		t.Fatalf("raw result shape = %q, want tuple(go:context.Context, func() -> ())", got)
	}
	plainResult := GoSignatureRawResultType(GoFuncSignature{
		Name: "WithTimeout", Params: []string{"context.Context", "time.Duration"},
		Results: []string{"context.Context", "CancelFunc"}, Variadic: false, TypeParams: []string{},
	})
	if _, isOK := plainResult.(Result__Ok[ast2.MonoType, string]); isOK {
		t.Fatalf("plain GoSignatureRawResultType resolved a bare package-local name; want an error (package context is required)")
	}
	if err, isErr := plainResult.(Result__Err[ast2.MonoType, string]); !isErr || !strings.Contains(err.F0, "CancelFunc") {
		t.Fatalf("unexpected plain result: %v", plainResult)
	}
}

// TestGoSignatureForCalleeWithPackage pins the callee walker returning the
// matched signature together with its owning package.
func TestGoSignatureForCalleeWithPackage(t *testing.T) {
	callee := ast2.Expr{Kind: ast2.ExprKind__FieldExpr__Ctor(ast2.Expr{Kind: ast2.ExprKind__IdentExpr__Ctor("context")}, "WithTimeout")}
	result := GoSignatureForCalleeWithPackage(callee, []GoPackageEntry{contextFixture()}, ffiMultiResultPredicate)
	some, ok := result.(Option__Some[struct {
		F0 GoFuncSignature
		F1 GoPackageEntry
	}])
	if !ok {
		t.Fatalf("GoSignatureForCalleeWithPackage returned no match for context.WithTimeout")
	}
	if some.F0.F0.Name != "WithTimeout" || len(some.F0.F0.Results) != 2 {
		t.Fatalf("unexpected matched signature: %+v", some.F0)
	}
	if some.F0.F1.Alias != "context" {
		t.Fatalf("matched package alias = %q, want context", some.F0.F1.Alias)
	}
}

// TestInferFFIFieldAccess pins that a field selector on a Go-FFI struct
// resolves to the field's declared type (r.Header : http.Header), so chained
// member access type-checks instead of failing with `unknown field`.
func TestInferFFIFieldAccess(t *testing.T) {
	src := `package sample

import http "go:net/http"

func headerOf(r: Ref[http.Request]) -> http.Header
  let h = r.Header
  h
end

func setHeader(r: Ref[http.Request]) -> ()
  r.Header.Set("X-Test", "1")
  ()
end
`
	parsed := parser2.ParseFile(src)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	fileWithIDs := mustAssignFileExprIDsForTest(t, file.F0)
	result := InferPackageWithGoPackages(
		[]PkgDeclSource{{Path: "ffi-field.mygo", Decls: fileWithIDs.Decls}},
		[]GoPackageEntry{httpFixture()},
	)
	if _, ok := result.(Result__Ok[PackageInfo, string]); !ok {
		t.Fatalf("inference rejected Go-FFI field access: %v", result)
	}
}

// TestInferGoMethodReturnsPackageLocalType pins the package-aware GoMethod
// instantiation: a method whose result is a bare package-local named type is
// resolved with the owning package and yields a callable value.
func TestInferGoMethodReturnsPackageLocalType(t *testing.T) {
	pkg := contextFixture()
	pkg.Types = append(pkg.Types, GoTypeSignature{
		TypeName: "CancelContext",
		Methods: []GoFuncSignature{
			{Name: "CancelFn", Params: []string{}, Results: []string{"CancelFunc"}, Variadic: false, TypeParams: []string{}},
		},
		Fields:     []GoFieldSignature{},
		Underlying: "struct{}",
	})
	src := `package sample

import context "go:context"

func use(c: Ref[context.CancelContext]) -> ()
  let f = c.CancelFn()
  f()
  ()
end
`
	parsed := parser2.ParseFile(src)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	fileWithIDs := mustAssignFileExprIDsForTest(t, file.F0)
	result := InferPackageWithGoPackages(
		[]PkgDeclSource{{Path: "ffi-method.mygo", Decls: fileWithIDs.Decls}},
		[]GoPackageEntry{pkg},
	)
	if _, ok := result.(Result__Ok[PackageInfo, string]); !ok {
		t.Fatalf("inference rejected method returning a package-local func type: %v", result)
	}
}
