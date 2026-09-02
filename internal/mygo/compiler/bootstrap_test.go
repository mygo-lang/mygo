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
	if _, err := SyncBootstrap(dir); err != nil {
		t.Fatalf("SyncBootstrap() error = %v", err)
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
