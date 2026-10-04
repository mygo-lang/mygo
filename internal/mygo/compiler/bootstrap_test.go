package compiler

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func bootstrapTestModule(t *testing.T, dir string) {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	goMod := "module example.com/bootstrap-test\n\ngo 1.26\n\nrequire github.com/mygo-lang/mygo v0.0.0\n\nreplace github.com/mygo-lang/mygo => " + filepath.ToSlash(root) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompileDirBootstrapGeneratesParser2WithoutStaticInitCycles(t *testing.T) {
	dir := filepath.Join("..", "parser2")
	if _, err := CompileDirBootstrap(dir); err != nil {
		t.Fatalf("CompileDirBootstrap(%q) error = %v", dir, err)
	}
}

func TestCompileDirBootstrapUsesSelfHostedPipeline(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

enum Maybe[A]
  Have(A)
  Nothing
end

func unwrap(value: Maybe[Int]) -> Int
  switch value
    case Have(item) => item
    case Nothing => 0
  end
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(dir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	if len(written) != 1 {
		t.Fatalf("CompileDirBootstrap() wrote %d files, want 1", len(written))
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), written[0], generated, parser.AllErrors); err != nil {
		t.Fatalf("bootstrap output is invalid Go: %v\n%s", err, generated)
	}
}

func TestCompileDirBootstrapAcceptsPointerImplementingGoInterface(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("internal/smtp/smtp.go", `package smtp

type Backend interface { NewSession() }
type Server struct{}
func NewServer(backend Backend) *Server { return &Server{} }
func (*Server) Close() {}
`)
	write("internal/smtpserver/backend.go", `package smtpserver

type SMTPBackend struct{}
func (*SMTPBackend) NewSession() {}
func NewSMTPBackend() *SMTPBackend { return &SMTPBackend{} }
`)
	write("listener.mygo", `package listener

import smtp "go:example.com/bootstrap-test/internal/smtp"
import smtpserver "go:example.com/bootstrap-test/internal/smtpserver"

func Run() -> ()
  let backend = smtpserver.NewSMTPBackend()
  let server = smtp.NewServer(backend)
  server.Close()
  ()
end
`)
	written, err := CompileDirBootstrap(dir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() rejected pointer interface assignment: %v", err)
	}
	if len(written) == 0 {
		t.Fatal("CompileDirBootstrap() generated no files")
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated pointer-to-interface package did not type-check: %v\n%s", err, output)
	}
}

func TestCompileDirBootstrapMatchesUnitPayloadInGoErrorResult(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("internal/smtp/smtp.go", `package smtp

type Server struct{}
func (*Server) ListenAndServe() error { return nil }
`)
	write("listener.mygo", `package listener

import smtp "go:example.com/bootstrap-test/internal/smtp"

func Run(server: Ref[smtp.Server]) -> ()
  switch server.ListenAndServe()
    case Ok(()) => ()
    case Err(_) => ()
  end
end
`)
	written, err := SyncBootstrap(dir)
	if err != nil {
		t.Fatalf("SyncBootstrap() error = %v", err)
	}
	generated, err := os.ReadFile(filepath.Join(dir, "zz_listener.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "Result__Ok[struct{}, error]") {
		t.Fatalf("generated output did not match the unit Result payload:\n%s", generated)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated Go failed to compile: %v\n%s", err, output)
	}
	if len(written) == 0 {
		t.Fatal("SyncBootstrap() did not generate files")
	}
}

func TestCompileDirBootstrapLowersNestedTupleLetPattern(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

func pair() -> (Int, (Int, Int))
  (1, (2, 3))
end

func Result() -> Int
  let (first, (_, last)) = pair()
  first + last
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirBootstrap(dir); err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(`package sample

import "testing"

func TestResult(t *testing.T) {
	if got := Result(); got != 4 {
		t.Fatalf("Result() = %d, want 4", got)
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated nested tuple package failed: %v\n%s", err, output)
	}
}

func TestCompileDirBootstrapLowersTupleReturningSwitch(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

enum Signal
  Continue
  Stop
end

func reduce(signal: Signal) -> (Int, Slice[String])
  switch signal
    case Continue => (1, ["next"])
    case _ => (0, [])
  end
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirBootstrap(dir); err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(`package sample

import "testing"

func TestReduce(t *testing.T) {
	value, commands := reduce(Signal__Continue__Ctor())
	if value != 1 || len(commands) != 1 || commands[0] != "next" {
		t.Fatalf("reduce(Continue) = (%d, %v)", value, commands)
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated tuple-switch package failed: %v\n%s", err, output)
	}
}

func TestCompileDirBootstrapLowersTupleVariantBindingUnderReturnSwitch(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

enum State
  Idle
  Running
end

enum Command
  Noop
end

enum AgentEvent
  RunStarted { InitialMessage: String }
  Other
end

func helper(msg: String) -> Int
  msg.Len()
end

func step(state: State, nextState: State, event: AgentEvent) -> (State, Slice[Command])
  switch (state, event)
    case (Idle, RunStarted { InitialMessage }) =>
      let _ = helper(InitialMessage)
      (nextState, [Command.Noop])
    case _ => (state, [Command.Noop])
  end
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirBootstrap(dir); err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(`package sample

import "testing"

func TestBuilds(t *testing.T) {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated tuple variant binding package failed: %v\n%s", err, output)
	}
}

func TestCompileDirBootstrapAddsPreludeImportOnlyForEmittedHelpers(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	interfaceSource := `package sample

interface Store
  func Save(value: Result[Int, String]) -> Result[(), String]
end
`
	helperSource := `package sample

func Count(items: Slice[String]) -> Int
  let expanded = items.Append("next")
  var total = 0
  expanded.Each(func(value: String) -> ()
    total = total + value.Len()
  end)
  total
end
`
	if err := os.WriteFile(filepath.Join(dir, "store.mygo"), []byte(interfaceSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "use.mygo"), []byte(helperSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirBootstrap(dir); err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	storeGenerated, err := os.ReadFile(filepath.Join(dir, "zz_store.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(storeGenerated), `github.com/mygo-lang/mygo/prelude`) {
		t.Fatalf("erased interface-only output imported prelude:\n%s", storeGenerated)
	}
	useGenerated, err := os.ReadFile(filepath.Join(dir, "zz_use.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(useGenerated), `. "github.com/mygo-lang/mygo/prelude"`) {
		t.Fatalf("helper-using output did not import prelude:\n%s", useGenerated)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(`package sample

import "testing"

func TestCount(t *testing.T) {
	if got := Count([]string{"a"}); got != 5 {
		t.Fatalf("Count() = %d, want 5", got)
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated mixed-prelude package failed: %v\n%s", err, output)
	}
}

func TestCompileDirBootstrapLowersStatementSwitchPatterns(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

enum Marker
  Marked(Int)
  Empty
end

func Result() -> Int
  var total: Int = 0
  switch 1
    case 1 => total = total + 1
    case _ => total = 99
  end
  switch 2
    case item => total = total + item
  end
  switch (3, 4)
    case (left, right) => total = total + left + right
  end
  switch Marker.Marked(5)
    case Marked(value) => total = total + value
    case _ => total = 99
  end
  total
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirBootstrap(dir); err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(`package sample

import "testing"

func TestResult(t *testing.T) {
	if got := Result(); got != 15 {
		t.Fatalf("Result() = %d, want 15", got)
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated statement switch package failed: %v\n%s", err, output)
	}
}

func TestCompileDirBootstrapLowersTupleStructVariantPatterns(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

enum AgentEvent
  Timeout { Where: Int }
  Started
end

func timeoutWhere(state: Int, event: AgentEvent) -> Int
  switch (state, event)
    case (current, Timeout { Where }) => current + Where
    case (_, _) => -1
  end
end

func matched() -> Int
  timeoutWhere(3, AgentEvent.Timeout { Where: 4 })
end

func unmatched() -> Int
  timeoutWhere(3, AgentEvent.Started)
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirBootstrap(dir); err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(`package sample

import "testing"

func TestTupleStructVariant(t *testing.T) {
	if got := matched(); got != 7 {
		t.Fatalf("matched() = %d, want 7", got)
	}
	if got := unmatched(); got != -1 {
		t.Fatalf("unmatched() = %d, want -1", got)
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated tuple struct variant package failed: %v\n%s", err, output)
	}
}

func TestCompileDirBootstrapLowersNearestLoopControl(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

func Result() -> Int
  var outer: Int = 0
  var total: Int = 0
  while outer < 3
    outer = outer + 1
    var inner: Int = 0
    while inner < 3
      inner = inner + 1
      if inner == 2 then continue end
      if outer == 2 then break end
      total = total + 1
    end
  end
  total
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirBootstrap(dir); err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(`package sample

import "testing"

func TestResult(t *testing.T) {
	if got := Result(); got != 4 {
		t.Fatalf("Result() = %d, want 4", got)
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated loop-control package failed: %v\n%s", err, output)
	}
}

func TestBootstrapNoPreludeSyncDoesNotResolvePrelude(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/no-prelude\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte("package sample\n\nfunc Value() -> Int\n  42\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/mygo", "--bootstrap", "--no-prelude", "sync", dir)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bootstrap no-prelude sync failed: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(dir, "zz_sample.gen.go")); err != nil {
		t.Fatalf("bootstrap no-prelude output: %v", err)
	}
}

func TestSyncBootstrapBuildsExternalTestPackage(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte("package sample\n\nfunc Value() -> Int\n  42\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.mygo"), []byte("package sample_test\n\nimport testing \"go:testing\"\n\nfunc TestValue(t: Ref[testing.T]) -> ()\n  if Value() != 42 then t.Fatal(\"Value failed\") end\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "helper_test.mygo"), []byte("package sample_test\n\nfunc UsesPrelude() -> Int\n  Panic(\"unused helper\")\n  Value()\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := SyncBootstrap(dir)
	if err != nil {
		t.Fatalf("SyncBootstrap() error = %v", err)
	}
	for _, path := range written {
		generated, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		switch filepath.Base(path) {
		case "zz_sample.gen_test.go":
			if strings.Contains(string(generated), `github.com/mygo-lang/mygo/prelude`) {
				t.Fatalf("external test using main-package declarations imported prelude:\n%s", generated)
			}
		case "zz_helper.gen_test.go":
			if !strings.Contains(string(generated), `. "github.com/mygo-lang/mygo/prelude"`) {
				t.Fatalf("external test using Panic omitted the prelude import:\n%s", generated)
			}
		}
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated external test package failed: %v\n%s", err, output)
	}
}

func TestSyncBootstrapKeepsInternalTestPackageUnimported(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte("package sample\n\nfunc Value() -> Int\n  42\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.mygo"), []byte("package sample\n\nimport testing \"go:testing\"\n\nfunc TestValue(t: Ref[testing.T]) -> ()\n  if Value() != 42 then t.Fatal(\"Value failed\") end\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := SyncBootstrap(dir)
	if err != nil {
		t.Fatalf("SyncBootstrap() error = %v", err)
	}
	for _, path := range written {
		if strings.HasSuffix(path, ".gen_test.go") {
			generated, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(generated), `import . "example.com/bootstrap-test"`) {
				t.Fatalf("internal test has a self-import:\n%s", generated)
			}
		}
	}
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated internal test package failed: %v\n%s", err, output)
	}
}

func TestGenerateSourceLoadsPreludeForOption(t *testing.T) {
	src := `package sample

func Default(value: Option[Int]) -> Option[Int]
  value
end
`
	generated, err := GenerateSourceAt("option.mygo", src)
	if err != nil {
		t.Fatalf("GenerateSourceAt() error = %v", err)
	}
	if !strings.Contains(generated, `. "github.com/mygo-lang/mygo/prelude"`) {
		t.Fatalf("non-prelude source did not dot-import prelude:\n%s", generated)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "option.gen.go", generated, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, generated)
	}
}

func TestGenerateSourceUsesPreludeHKTDeclarations(t *testing.T) {
	src := `package sample

interface Enumerable[C[A], A]
  func First(value: C[A]) -> A
end

func Default(value: Option[Int]) -> Option[Int]
  value
end
`
	generated, err := GenerateSource(src)
	if err != nil {
		t.Fatalf("GenerateSource() error = %v", err)
	}
	if strings.Contains(generated, "type HKTType interface{}") {
		t.Fatalf("non-prelude package redeclared prelude HKT helpers:\n%s", generated)
	}
	if !strings.Contains(generated, `. "github.com/mygo-lang/mygo/prelude"`) {
		t.Fatalf("non-prelude package did not dot-import prelude:\n%s", generated)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "hkt.gen.go", generated, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, generated)
	}
}

func TestSyncBootstrapCompilesRootPackage(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(`package sample

func value() -> Int
  42
end
`), 0o644); err != nil {
		t.Fatal(err)
	}

	written, err := SyncBootstrap(dir)
	if err != nil {
		t.Fatalf("SyncBootstrap(%q): %v", dir, err)
	}
	want := filepath.Join(dir, "zz_sample.gen.go")
	if len(written) != 1 || written[0] != want {
		t.Fatalf("SyncBootstrap() wrote %v, want [%q]", written, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("generated root package file: %v", err)
	}
}

func TestSyncBootstrapReportIsDeterministicWithTestFiles(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte("package sample\n\nfunc Value() -> Int\n  42\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample_test.mygo"), []byte("package sample\n\nfunc TestValue() -> Int\n  Value()\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := SyncBootstrap(dir)
	if err != nil {
		t.Fatalf("first SyncBootstrap() error = %v", err)
	}
	second, err := SyncBootstrap(dir)
	if err != nil {
		t.Fatalf("second SyncBootstrap() error = %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("SyncBootstrap() lengths differ: first=%v second=%v", first, second)
	}
	testFile := false
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("SyncBootstrap() order differs at %d: first=%v second=%v", i, first, second)
		}
		if strings.HasSuffix(first[i], ".gen_test.go") {
			testFile = true
		}
	}
	if !testFile {
		t.Fatalf("SyncBootstrap() did not report a Go-recognized test file: %v", first)
	}
	if len(first) != 2 {
		t.Fatalf("SyncBootstrap() wrote %d files, want 2: %v", len(first), first)
	}
}

func TestCompileDirBootstrapUsesPreludeHKTDeclarations(t *testing.T) {
	root := t.TempDir()
	preludeDir := filepath.Join(root, "prelude")
	appDir := filepath.Join(root, "app")
	for _, dir := range []string{preludeDir, appDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	bootstrapTestModule(t, root)
	if err := os.WriteFile(filepath.Join(preludeDir, "prelude.mygo"), []byte(`package prelude

enum Option[A]
  Some(A)
  None
end

interface Enumerable[C[A], A]
  func First(value: C[A]) -> A
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.mygo"), []byte(`package app

func Keep(value: Option[Int]) -> Option[Int]
  value
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(appDir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	if len(written) != 1 {
		t.Fatalf("CompileDirBootstrap() wrote %d files, want 1", len(written))
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(generated), "type HKTType interface{}") {
		t.Fatalf("non-prelude bootstrap output redeclared prelude HKT helpers:\n%s", generated)
	}
	if !strings.Contains(string(generated), `. "github.com/mygo-lang/mygo/prelude"`) {
		t.Fatalf("non-prelude bootstrap output did not dot-import prelude:\n%s", generated)
	}
}

func TestCompileDirBootstrapSupportsGoFFI(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

import fmt "go:fmt"

func Render(value: Int) -> String
  fmt.Sprint(value)
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(dir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), `import "fmt"`) || !strings.Contains(string(generated), "fmt.Sprint") {
		t.Fatalf("generated Go did not preserve fmt FFI:\n%s", generated)
	}
}

func TestCompileDirBootstrapRegistersGoTypeMethodsWithImportAlias(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

import testing "go:testing"

func TestFatal(t: Ref[testing.T]) -> ()
  t.Fatal("expected failure")
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample_test.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(dir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	if len(written) != 1 {
		t.Fatalf("CompileDirBootstrap() wrote %d files, want 1", len(written))
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), `t.Fatal("expected failure")`) {
		t.Fatalf("generated Go did not preserve testing.T.Fatal:\n%s", generated)
	}
}

func TestCompileDirBootstrapRegistersGoTypeMethodsWithDotImport(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

import . "go:testing"

func TestFatal(t: Ref[T]) -> ()
  t.Fatal("expected failure")
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample_test.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(dir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), `. "testing"`) || !strings.Contains(string(generated), `t.Fatal("expected failure")`) {
		t.Fatalf("generated Go did not preserve dot-imported testing.T.Fatal:\n%s", generated)
	}
}

func TestCompileDirBootstrapRejectsUnknownGoFFISelector(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

import fmt "go:fmt"

func Render(value: Int) -> String
  fmt.NotAFunction(value)
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := CompileDirBootstrap(dir)
	if err == nil || !strings.Contains(err.Error(), "unknown Go package member NotAFunction") {
		t.Fatalf("CompileDirBootstrap() error = %v, want unknown Go package member", err)
	}
}

func TestCompileDirBootstrapRejectsWrongFixedGoFFIArity(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

import strconv "go:strconv"

func Render() -> String
  strconv.Itoa()
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := CompileDirBootstrap(dir)
	if err == nil || !strings.Contains(err.Error(), "Go function argument count mismatch") {
		t.Fatalf("CompileDirBootstrap() error = %v, want Go function argument count mismatch", err)
	}
}

func TestCompileDirBootstrapDecodesGoValueBoolAsOption(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

import strings "go:strings"

func TrimPrefix(value: String) -> Option[String]
  strings.CutPrefix(value, "pre")
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(dir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"strings.CutPrefix", "Some[string]", "None[string]"} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("generated Go missing %q:\n%s", want, generated)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), written[0], generated, parser.AllErrors); err != nil {
		t.Fatalf("bootstrap output is invalid Go: %v\n%s", err, generated)
	}
}

func TestCompileDirBootstrapDecodesGoValueErrorAsResult(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	source := `package sample

import strconv "go:strconv"

func Parse(value: String) -> Result[Int, Error]
  strconv.Atoi(value)
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(dir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"strconv.Atoi", "Ok[int, error]", "Err[int, error]"} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("generated Go missing %q:\n%s", want, generated)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), written[0], generated, parser.AllErrors); err != nil {
		t.Fatalf("bootstrap output is invalid Go: %v\n%s", err, generated)
	}
}

func TestCompileDirBootstrapCompilesMyGOImports(t *testing.T) {
	root := t.TempDir()
	bootstrapTestModule(t, root)
	libDir := filepath.Join(root, "lib")
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "lib.mygo"), []byte(`package lib

func Add(left: Int, right: Int) -> Int
  left + right
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.mygo"), []byte(`package app

import lib "example.com/bootstrap-test/lib"

func Run() -> Int
  lib.Add(1, 2)
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(appDir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	if len(written) != 2 {
		t.Fatalf("CompileDirBootstrap() wrote %d files, want app and dependency", len(written))
	}
	for _, path := range written {
		generated, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), path, generated, parser.AllErrors); err != nil {
			t.Fatalf("bootstrap output %s is invalid Go: %v\n%s", path, err, generated)
		}
	}
}

func TestCompileDirBootstrapUsesGeneratedMetadataWithoutDependencySources(t *testing.T) {
	root := t.TempDir()
	bootstrapTestModule(t, root)
	libDir := filepath.Join(root, "lib")
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	libSource := `package lib

import time "go:time"

struct Widget
  Name: String
end

func MakeWidget(name: String) -> Widget
  Widget { Name: name }
end

func Identity[A](value: A) -> A
  value
end

func Now() -> time.Time
  time.Now()
end

interface Render[A]
  func Render(value: A) -> String
end
`
	appSource := `package app

import lib "example.com/bootstrap-test/lib"

func Name() -> String
  lib.Identity[String](lib.MakeWidget("from metadata").Name)
end

func Timestamp() -> Int64
  lib.Now().Unix()
end
`
	if err := os.WriteFile(filepath.Join(libDir, "lib.mygo"), []byte(libSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.mygo"), []byte(appSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirBootstrap(appDir); err != nil {
		t.Fatalf("initial dependency build failed: %v", err)
	}
	if err := os.Remove(filepath.Join(libDir, "lib.mygo")); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(appDir)
	if err != nil {
		t.Fatalf("caller failed with dependency .mygo source removed: %v", err)
	}
	if len(written) != 1 || filepath.Base(written[0]) != "zz_app.gen.go" {
		t.Fatalf("caller generated files = %v, want only app output", written)
	}
}

func TestCompileDirBootstrapEmitsDynamicDispatchToProducerAdapter(t *testing.T) {
	root := t.TempDir()
	bootstrapTestModule(t, root)
	contractsDir := filepath.Join(root, "contracts")
	backendDir := filepath.Join(root, "backend")
	appDir := filepath.Join(root, "app")
	for _, dir := range []string{contractsDir, backendDir, appDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	contractSource := `package contracts

interface Show[A]
  func Show(value: A) -> String
  func Echo[B](value: A, item: B) -> B
end
`
	backendSource := `package backend

import contracts "example.com/bootstrap-test/contracts"

struct Widget
  Name: String
end

struct Gadget
  Name: String
end

impl WidgetShow: contracts.Show[Widget]
  func Show(value: Widget) -> String
    value.Name
  end
  func Echo[B](value: Widget, item: B) -> B
    item
  end
end

impl GadgetShow: contracts.Show[Gadget]
  func Show(value: Gadget) -> String
    value.Name
  end
  func Echo[B](value: Gadget, item: B) -> B
    item
  end
end
`
	appSource := `package app

import backend "example.com/bootstrap-test/backend"

func WidgetName() -> String
  let widget = backend.Widget { Name: "widget" }
  let gadget = backend.Gadget { Name: "gadget" }
  let boxedWidget = MygoDynamicShowFromBackendWidgetShow(widget)
  let boxedGadget = MygoDynamicShowFromBackendGadgetShow(gadget)
  let values = [boxedWidget, boxedGadget]
  switch values.Get(0)
    case Some(first) then
      switch values.Get(1)
        case Some(second) then
          let shown = MygoDynamicShowCallShow(first)
          MygoDynamicShowCallEcho[String](first, shown) + MygoDynamicShowCallShow(second)
        end
        case None => ""
      end
    end
    case None => ""
  end
end
`
	for name, source := range map[string]string{
		filepath.Join(contractsDir, "contracts.mygo"): contractSource,
		filepath.Join(backendDir, "backend.mygo"):     backendSource,
		filepath.Join(appDir, "app.mygo"):             appSource,
	} {
		if err := os.WriteFile(name, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := CompileDirBootstrap(appDir); err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	backendGeneratedPath := filepath.Join(backendDir, "zz_backend.gen.go")
	contractsGeneratedPath := filepath.Join(contractsDir, "zz_contracts.gen.go")
	backendBefore, err := os.ReadFile(backendGeneratedPath)
	if err != nil {
		t.Fatal(err)
	}
	backendMetadata, err := os.ReadFile(filepath.Join(backendDir, "zz_mygo_metadata.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(backendMetadata), "example.com/bootstrap-test/contracts") {
		t.Fatalf("backend metadata does not preserve its interface dependency:\n%s", backendMetadata)
	}
	contractsBefore, err := os.ReadFile(contractsGeneratedPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, sourcePath := range []string{filepath.Join(backendDir, "backend.mygo"), filepath.Join(contractsDir, "contracts.mygo")} {
		if err := os.Remove(sourcePath); err != nil {
			t.Fatalf("remove dependency MyGO source %s: %v", sourcePath, err)
		}
	}
	if _, err := CompileDirBootstrap(appDir); err != nil {
		t.Fatalf("caller rebuild with source-free dependencies failed: %v", err)
	}
	backendAfter, err := os.ReadFile(backendGeneratedPath)
	if err != nil {
		t.Fatal(err)
	}
	contractsAfter, err := os.ReadFile(contractsGeneratedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(backendAfter) != string(backendBefore) || string(contractsAfter) != string(contractsBefore) {
		t.Fatal("caller rebuild modified generated dependency files")
	}
	dispatchPath := filepath.Join(appDir, "zz_mygo_dynamic.gen.go")
	dispatch, err := os.ReadFile(dispatchPath)
	if err != nil {
		t.Fatalf("caller dynamic API file was not generated: %v", err)
	}
	dispatchSource := string(dispatch)
	if !strings.Contains(dispatchSource, "backend.MygoDynamicAdapter") || !strings.Contains(dispatchSource, "contracts.MygoDynamicShowPayload(value)") {
		t.Fatalf("caller dispatch does not directly reference the producer adapter:\n%s", dispatchSource)
	}
	testSource := `package app

import (
	"testing"
)

func TestDynamicShow(t *testing.T) {
	if got := WidgetName(); got != "widgetgadget" {
		t.Fatalf("dynamic Show = %q", got)
	}
}
`
	if err := os.WriteFile(filepath.Join(appDir, "app_test.go"), []byte(testSource), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "./app")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		producer, _ := os.ReadFile(filepath.Join(backendDir, "zz_backend.gen.go"))
		t.Fatalf("generated caller adapters failed Go build/runtime test: %v\n%s\nproducer:\n%s", err, output, producer)
	}
}

func TestCompileDirBootstrapCompilesImportedTypeclassMethod(t *testing.T) {
	root := t.TempDir()
	bootstrapTestModule(t, root)
	libDir := filepath.Join(root, "lib")
	appDir := filepath.Join(root, "app")
	for _, dir := range []string{libDir, appDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(libDir, "lib.mygo"), []byte(`package lib

interface Measure[A]
  func Measure(value: A) -> Int
end

struct Box[A]
  Value: A
end

impl[A] BoxMeasure[A]: Measure[Box[A]]
  func Measure(value: Box[A]) -> Int
    42
  end
end

func NewBox[A](value: A) -> Box[A]
  Box[A] { Value: value }
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.mygo"), []byte(`package app

import lib "example.com/bootstrap-test/lib"

func Run() -> Int
  lib.NewBox[Int](1).Measure()
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(appDir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	appGenerated := filepath.Join(appDir, "zz_app.gen.go")
	generated, err := os.ReadFile(appGenerated)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "lib.MygoIT") || !strings.Contains(string(generated), "M7Measure") {
		t.Fatalf("generated imported typeclass call is not alias-qualified:\n%s", generated)
	}
	if len(written) != 3 {
		t.Fatalf("CompileDirBootstrap() wrote %d files, want app, dependency, and dynamic dispatch", len(written))
	}
	if err := os.WriteFile(filepath.Join(appDir, "app_test.go"), []byte(`package app

import "testing"

func TestRun(t *testing.T) {
	if got := Run(); got != 42 {
		t.Fatalf("Run() = %d, want 42", got)
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated imported typeclass package failed: %v\n%s", err, output)
	}
}

func TestCompileDirBootstrapConstructsImportedNamedEnumVariant(t *testing.T) {
	root := t.TempDir()
	bootstrapTestModule(t, root)
	libDir := filepath.Join(root, "lib")
	appDir := filepath.Join(root, "app")
	for _, dir := range []string{libDir, appDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(libDir, "shape.mygo"), []byte(`package lib

enum Shape
  Pair { Left: Int, Right: Int }
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.mygo"), []byte(`package app

import lib "example.com/bootstrap-test/lib"

func Make() -> lib.Shape
  lib.Shape.Pair { Right: 2, Left: 1 }
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(appDir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	for _, path := range written {
		generated, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), path, generated, parser.AllErrors); err != nil {
			t.Fatalf("bootstrap output %s is invalid Go: %v\n%s", path, err, generated)
		}
	}
	if err := os.WriteFile(filepath.Join(appDir, "app_test.go"), []byte(`package app

import "testing"

func TestMake(t *testing.T) {
	if Make() == nil {
		t.Fatal("Make returned nil")
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated imported enum package failed: %v\n%s", err, output)
	}
}

func TestCompileDirBootstrapKeepsGeneratedImportsPerSourceFile(t *testing.T) {
	root := t.TempDir()
	bootstrapTestModule(t, root)
	sources := map[string]string{
		"signature.mygo": `package sample

import mail "go:net/mail"
import bytes "go:bytes"

interface MailProcessor[A]
  func Convert(self: A, message: mail.Message) -> String
end

interface IByteSize[A]
  func ByteSize(self: A) -> Int
end

func NewBuffer() -> bytes.Buffer
  bytes.Buffer {}
end
`,
		"impl.mygo": `package sample

import bytes "go:bytes"

impl bytes.Buffer: IByteSize[bytes.Buffer]
  func ByteSize(self: bytes.Buffer) -> Int
    self.Len()
  end
end
`,
		"other.mygo": `package sample

func Unrelated() -> Int
  1
end
`,
		"caller.mygo": `package sample

func BufferLen() -> Int
  NewBuffer().ByteSize()
end
`,
	}
	for name, source := range sources {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	written, err := CompileDirBootstrap(root)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	outputs := map[string]string{}
	for _, path := range written {
		generated, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		outputs[filepath.Base(path)] = string(generated)
	}
	signature := outputs["zz_signature.gen.go"]
	if strings.Contains(signature, "net/mail") || !strings.Contains(signature, `"bytes"`) {
		t.Fatalf("signature imports are not selected from emitted declarations:\n%s", signature)
	}
	if other := outputs["zz_other.gen.go"]; strings.Contains(other, `"bytes"`) || strings.Contains(other, `"net/mail"`) {
		t.Fatalf("unrelated file inherited imports:\n%s", other)
	}
	if impl := outputs["zz_impl.gen.go"]; !strings.Contains(impl, `"bytes"`) {
		t.Fatalf("impl file lost its receiver import:\n%s", impl)
	}
	if caller := outputs["zz_caller.gen.go"]; strings.Contains(caller, `"bytes"`) {
		t.Fatalf("caller inherited bytes through a returned value:\n%s", caller)
	}
	if err := os.WriteFile(filepath.Join(root, "sample_test.go"), []byte(`package sample

import "testing"

func TestCrossFileMethods(t *testing.T) {
	if BufferLen() != 0 {
		t.Fatal("new buffer methods returned unexpected lengths")
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated multi-file package failed to build: %v\n%s", err, output)
	}
}
