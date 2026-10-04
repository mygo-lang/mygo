package codegen2

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
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

func generateWithGoPackages(t *testing.T, src string, pkgs []typeinference2.GoPackageEntry) string {
	t.Helper()
	parsed := parseSourceAsAst2(src)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("parseSourceAsAst2 failed: %v", parsed)
	}
	fileWithIDs := mustAssignFileExprIDsForTest(t, file.F0)
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

func TestGenerateImportedStaticInherentMethodUsesMangledHelper(t *testing.T) {
	depSource := `package backends

struct SendPulseBackend
  token: String
end

struct SMTPBackend
  token: String
end

struct DispatchBackend
  token: String
end

impl SendPulseBackend
  func New(token: String) -> SendPulseBackend
    SendPulseBackend { token: token }
  end
end

impl SMTPBackend
  func New(token: String) -> SMTPBackend
    SMTPBackend { token: token }
  end
end

impl DispatchBackend
  func New(token: String) -> DispatchBackend
    DispatchBackend { token: token }
  end
end
`
	parsedDep := parseSourceAsAst2(depSource)
	depFile, ok := parsedDep.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("dependency parse failed: %v", parsedDep)
	}
	depFileWithIDs := mustAssignFileExprIDsForTest(t, depFile.F0)
	myGoPackage := typeinference2.MyGoPackageInfo{
		Alias: "backends", Path: "example.test/backends", Decls: depFileWithIDs.Decls,
	}

	src := `package app
import backends "example.test/backends"

func NewBackend() -> backends.SendPulseBackend
  backends.SendPulseBackend.New("token")
end

func NewSMTPBackend() -> backends.SMTPBackend
  backends.SMTPBackend.New("token")
end

func NewDispatchBackend() -> backends.DispatchBackend
  backends.DispatchBackend.New("token")
end
`
	parsed := parseSourceAsAst2(src)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("caller parse failed: %v", parsed)
	}
	fileWithIDs := mustAssignFileExprIDsForTest(t, file.F0)
	path := "caller.mygo"
	infoResult := typeinference2.InferPackageWithExternal(
		[]typeinference2.PkgDeclSource{{Path: path, Decls: fileWithIDs.Decls}},
		[]typeinference2.PkgDeclSource{},
		[]typeinference2.GoPackageEntry{},
		[]typeinference2.MyGoPackageInfo{myGoPackage},
	)
	info, ok := infoResult.(Result__Ok[typeinference2.PackageInfo, string])
	if !ok {
		t.Fatalf("inference failed: %v", infoResult)
	}
	generated := GenerateFiles([]SourceFileInput{{Path: path, File: fileWithIDs}}, info.F0)
	result, ok := generated.(Result__Ok[map[string]string, string])
	if !ok {
		t.Fatalf("GenerateFiles failed: %v", generated)
	}
	code := result.F0[sourceToGenName(path)]
	for _, want := range []string{
		"backends.MygoIN16SendPulseBackendM3New(\"token\")",
		"backends.MygoIN11SMTPBackendM3New(\"token\")",
		"backends.MygoIN15DispatchBackendM3New(\"token\")",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("imported static method helper call %q missing:\n%s", want, code)
		}
	}
	if strings.Contains(code, "backends.SendPulseBackend.New") || strings.Contains(code, "backends.SMTPBackend.New") || strings.Contains(code, "backends.DispatchBackend.New") {
		t.Fatalf("imported static method remained a Go type selector:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "caller.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
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

func multiValueErrorFixturePkg() typeinference2.GoPackageEntry {
	return typeinference2.GoPackageEntry{
		Alias: "lookup", Path: "go:example.com/lookup",
		Funcs: []typeinference2.GoFuncSignature{
			{Name: "Find", Params: []string{"string"}, Results: []string{"Value", "int", "error"}, Variadic: false, TypeParams: []string{}},
		},
		Types: []typeinference2.GoTypeSignature{
			{TypeName: "Value", Methods: []typeinference2.GoFuncSignature{}, Fields: []typeinference2.GoFieldSignature{}, Underlying: "struct{}"},
			{TypeName: "Service", Methods: []typeinference2.GoFuncSignature{
				{Name: "Find", Params: []string{"string"}, Results: []string{"Value", "int", "error"}, Variadic: false, TypeParams: []string{}},
			}, Fields: []typeinference2.GoFieldSignature{}, Underlying: "struct{}"},
		},
	}
}

func callbackFixturePkg(name, underlying string) typeinference2.GoPackageEntry {
	return typeinference2.GoPackageEntry{
		Alias: "mailer", Path: "go:example.test/smtpserver", GoName: "listener",
		Funcs: []typeinference2.GoFuncSignature{
			{Name: "NewSMTPBackend", Params: []string{"listener." + name}, Results: []string{"*Backend"}, Variadic: false, TypeParams: []string{}},
			{Name: "DefaultSendFunc", Params: []string{}, Results: []string{"listener." + name}, Variadic: false, TypeParams: []string{}},
		},
		Types: []typeinference2.GoTypeSignature{
			{TypeName: "Backend", Methods: []typeinference2.GoFuncSignature{}, Fields: []typeinference2.GoFieldSignature{}, Underlying: "struct{}"},
			{TypeName: "Message", Methods: []typeinference2.GoFuncSignature{}, Fields: []typeinference2.GoFieldSignature{}, Underlying: "struct{}"},
			{TypeName: name, Methods: []typeinference2.GoFuncSignature{}, Fields: []typeinference2.GoFieldSignature{}, Underlying: underlying},
		},
	}
}

func TestGenerateFilesAdaptsResultCallbackToGoErrorABIs(t *testing.T) {
	for _, fixture := range []struct {
		name       string
		underlying string
		want       []string
	}{
		{name: "SendFunc", underlying: "func(string, string, *Message) error", want: []string{"func(string, string, *mailer.Message) error", "Result__Err[struct{}, error]", "return nil"}},
		{name: "SendFunc", underlying: "func(string, string, *Message) (struct{}, error)", want: []string{"func(string, string, *mailer.Message) (struct{}, error)", "Result__Err[struct{}, error]", "nil"}},
	} {
		t.Run(fixture.underlying, func(t *testing.T) {
			src := `package sample

import mailer "go:example.test/smtpserver"

func send(from: String, to: String, message: Ref[mailer.Message]) -> Result[(), Error]
  Ok(())
end

func New() -> Ref[mailer.Backend]
  mailer.NewSMTPBackend(send)
end

func NewFail(err: Error) -> Ref[mailer.Backend]
  mailer.NewSMTPBackend(func(from: String, to: String, message: Ref[mailer.Message]) -> Result[(), Error]
    Err(err)
  end)
end

func NewDirect() -> Ref[mailer.Backend]
  mailer.NewSMTPBackend(mailer.DefaultSendFunc())
end
`
			code := generateWithGoPackages(t, src, []typeinference2.GoPackageEntry{callbackFixturePkg(fixture.name, fixture.underlying)})
			if _, err := parser.ParseFile(token.NewFileSet(), "callback.gen.go", code, parser.AllErrors); err != nil {
				t.Fatalf("generated Go is invalid: %v\n%s", err, code)
			}
			for _, want := range fixture.want {
				if !strings.Contains(code, want) {
					t.Fatalf("generated callback adapter missing %q:\n%s", want, code)
				}
			}
			if strings.Count(code, "__mygo_callback(__mygo_callback_arg0, __mygo_callback_arg1, __mygo_callback_arg2)") != 2 {
				t.Fatalf("each generated adapter should invoke its original callback once:\n%s", code)
			}
			if !strings.Contains(code, "mailer.NewSMTPBackend(mailer.DefaultSendFunc())") {
				t.Fatalf("native Go callback ABI should pass through without an adapter:\n%s", code)
			}
			compileGeneratedCallbackFixture(t, code, fixture.underlying)
		})
	}
}

func compileGeneratedCallbackFixture(t *testing.T, generated, underlying string) {
	t.Helper()
	dir := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "go.mod"), "module example.test\n\ngo 1.22\n")
	write(filepath.Join(dir, "zz_fixture.go"), generated+`

type Result[A, E any] interface { isResult() }
type Result__Ok[A, E any] struct { F0 A }
func (Result__Ok[A, E]) isResult() {}
type Result__Err[A, E any] struct { F0 E }
func (Result__Err[A, E]) isResult() {}
func Ok[A, E any](a A) Result[A, E] { return Result__Ok[A, E]{F0: a} }
func Err[A, E any](e E) Result[A, E] { return Result__Err[A, E]{F0: e} }
`)
	mailerDir := filepath.Join(dir, "smtpserver")
	if err := os.MkdirAll(mailerDir, 0o700); err != nil {
		t.Fatal(err)
	}
	callback := `type SendFunc func(string, string, *Message) error
func invoke(send SendFunc) { callCount++; lastError = send("from", "to", &Message{}) }`
	defaultCallback := `func(string, string, *Message) error { return nil }`
	if strings.Contains(underlying, "(struct{}, error)") {
		callback = `type SendFunc func(string, string, *Message) (struct{}, error)
func invoke(send SendFunc) { callCount++; _, lastError = send("from", "to", &Message{}) }`
		defaultCallback = `func(string, string, *Message) (struct{}, error) { return struct{}{}, nil }`
	}
	write(filepath.Join(mailerDir, "mailer.go"), `package listener
type Backend struct{}
type Message struct{}
var callCount int
var lastError error
func Reset() { callCount = 0; lastError = nil }
func CallCount() int { return callCount }
func LastError() error { return lastError }
`+callback+`
func NewSMTPBackend(send SendFunc) *Backend { invoke(send); return &Backend{} }
func DefaultSendFunc() SendFunc { return `+defaultCallback+` }
`)
	write(filepath.Join(dir, "fixture_test.go"), `package sample
import (
  "errors"
  "testing"
  mailer "example.test/smtpserver"
)
func TestGeneratedCallbackAdapter(t *testing.T) {
  mailer.Reset()
  New()
  if mailer.CallCount() != 1 || mailer.LastError() != nil { t.Fatalf("success callback: count=%d err=%v", mailer.CallCount(), mailer.LastError()) }
  NewDirect()
  if mailer.CallCount() != 2 || mailer.LastError() != nil { t.Fatalf("native callback: count=%d err=%v", mailer.CallCount(), mailer.LastError()) }
  NewFail(errors.New("send failed"))
  if mailer.CallCount() != 3 || mailer.LastError() == nil || mailer.LastError().Error() != "send failed" { t.Fatalf("error callback: count=%d err=%v", mailer.CallCount(), mailer.LastError()) }
}
`)
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated callback Go did not compile and run: %v\n%s", err, out)
	}
}

func TestGenerateFilesWrapsMultipleSuccessValuesBeforeError(t *testing.T) {
	src := `package sample

import lookup "go:example.com/lookup"

func find(key: String) -> Result[(lookup.Value, Int), Error]
  lookup.Find(key)
end

func findMethod(service: Ref[lookup.Service], key: String) -> Result[(lookup.Value, Int), Error]
  service.Find(key)
end
`
	code := generateWithGoPackages(t, src, []typeinference2.GoPackageEntry{multiValueErrorFixturePkg()})
	if strings.Count(code, "lookup.Find(key)") != 1 || strings.Count(code, "service.Find(key)") != 1 {
		t.Fatalf("multi-value FFI calls were not emitted exactly once:\n%s", code)
	}
	for _, want := range []string{"lookup.Find(key)", "service.Find(key)", "F0:", "F1:", "F0 lookup.Value", "F1 int", "Result[", "!= nil"} {
		if !strings.Contains(code, want) {
			t.Fatalf("generated Go missing %q:\n%s", want, code)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "ffi-boundary.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateFilesPreservesRawThreeValueErrorBinding(t *testing.T) {
	src := `package sample

import lookup "go:example.com/lookup"

func find(key: String) -> ()
  let (value, count, err) = lookup.Find(key)
  value
  count
  err
  ()
end
`
	code := generateWithGoPackages(t, src, []typeinference2.GoPackageEntry{multiValueErrorFixturePkg()})
	if !strings.Contains(code, "value, count, err := lookup.Find(key)") {
		t.Fatalf("explicit tuple binding did not preserve the raw Go call:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "ffi-boundary.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateFilesRejectsWrongRawThreeValueErrorArity(t *testing.T) {
	src := `package sample

import lookup "go:example.com/lookup"

func bad(key: String) -> ()
  let (value, err) = lookup.Find(key)
  value
  err
  ()
end
`
	parsed := parseSourceAsAst2(src)
	fileResult, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("parseSourceAsAst2 failed: %v", parsed)
	}
	file := mustAssignFileExprIDsForTest(t, fileResult.F0)
	infoResult := typeinference2.InferPackageWithGoPackages(
		[]typeinference2.PkgDeclSource{{Path: "ffi-boundary.mygo", Decls: file.Decls}},
		[]typeinference2.GoPackageEntry{multiValueErrorFixturePkg()},
	)
	info, ok := infoResult.(Result__Ok[typeinference2.PackageInfo, string])
	if !ok {
		if inferErr, isErr := infoResult.(Result__Err[typeinference2.PackageInfo, string]); !isErr || !strings.Contains(inferErr.F0, "tuple") {
			t.Fatalf("wrong tuple binding failed without a tuple diagnostic: %v", infoResult)
		}
		return // Inference may reject the inconsistent pattern before codegen.
	}
	generated := GenerateFiles([]SourceFileInput{{Path: "ffi-boundary.mygo", File: file}}, info.F0)
	result, ok := generated.(Result__Err[map[string]string, string])
	if !ok || !strings.Contains(result.F0, "tuple binding count does not match Go function result count") {
		t.Fatalf("wrong tuple binding arity was not diagnosed: %v", generated)
	}
}

func TestGenerateFilesDiscardsMultiValueErrorCallAsStatement(t *testing.T) {
	src := `package sample

import lookup "go:example.com/lookup"

func run(key: String) -> ()
  lookup.Find(key)
  ()
end
`
	code := generateWithGoPackages(t, src, []typeinference2.GoPackageEntry{multiValueErrorFixturePkg()})
	if !strings.Contains(code, "lookup.Find(key)") || strings.Contains(code, "func() Result[") {
		t.Fatalf("discarded multi-value error call was wrapped instead of emitted as a statement:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "ffi-boundary.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
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
