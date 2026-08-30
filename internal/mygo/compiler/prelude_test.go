package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

func TestLoadPreludePackageFromNestedModuleDir(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "lib", "text", "parsec")
	pkg := loadPreludePackage(dir, dir)
	if pkg == nil {
		t.Fatal("loadPreludePackage returned nil")
	}
	if pkg.Interfaces["IEnumerable"] == nil {
		t.Fatal("prelude package missing IEnumerable")
	}
	foundSlice := false
	for _, impl := range pkg.Impls {
		if impl.InterfaceName != "IEnumerable" {
			continue
		}
		if len(impl.InterfaceArgs) == 0 {
			continue
		}
		if nt, ok := impl.InterfaceArgs[0].(*NamedType); ok && nt.Name == "Slice" {
			foundSlice = true
			break
		}
	}
	if !foundSlice {
		t.Fatal("prelude package missing Slice IEnumerable impl")
	}
}

func TestCompileDirInfersStructLiteralFromSplitFileTypeDecl(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "types.mygo"), []byte(`package sample

struct Box
  Value: Int
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte(`package sample

func Make() -> Box
  Box { Value: 1 }
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDir(dir); err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
}

func TestCompileDirSupportsPackageLevelLetAndVar(t *testing.T) {
	dir := t.TempDir()
	src := `package sample

func Next() -> Int
  count = count + 1
  count + limit
end

let limit: Int = 10
var count: Int = 1
`
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirNoPrelude(dir)
	if err != nil {
		t.Fatalf("CompileDirNoPrelude() error = %v", err)
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	text := string(generated)
	if !strings.Contains(text, "var limit int") || !strings.Contains(text, "var count int") || !strings.Contains(text, "limit = 10") || !strings.Contains(text, "count = 1") || !strings.Contains(text, "count = count + 1") {
		t.Fatalf("generated source missing package bindings or their use:\n%s", text)
	}
}

func TestCompileDirSupportsLoopControlAndRejectsItOutsideLoops(t *testing.T) {
	valid := t.TempDir()
	if err := os.WriteFile(filepath.Join(valid, "main.mygo"), []byte(`package sample

func Loop() -> ()
  while false
    continue
    break
  end
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirNoPrelude(valid)
	if err != nil {
		t.Fatalf("CompileDirNoPrelude(valid) error = %v", err)
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "continue") || !strings.Contains(string(generated), "break") {
		t.Fatalf("generated loop control is missing:\n%s", generated)
	}

	invalid := t.TempDir()
	if err := os.WriteFile(filepath.Join(invalid, "main.mygo"), []byte(`package sample

func Invalid() -> ()
  break
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirNoPrelude(invalid); err == nil || !strings.Contains(err.Error(), "break is only valid inside a while loop") {
		t.Fatalf("CompileDirNoPrelude(invalid) error = %v", err)
	}

	nestedFunc := t.TempDir()
	if err := os.WriteFile(filepath.Join(nestedFunc, "main.mygo"), []byte(`package sample

func InvalidNested() -> ()
  while true
    let f = func() -> ()
      continue
    end
  end
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirNoPrelude(nestedFunc); err == nil || !strings.Contains(err.Error(), "continue is only valid inside a while loop") {
		t.Fatalf("CompileDirNoPrelude(nestedFunc) error = %v", err)
	}
}

func TestCompileDirRunsNestedLoopControl(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module sample\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte(`package sample

func Score() -> Int
  var outer: Int = 0
  var total: Int = 0
  while outer < 3
    outer = outer + 1
    var inner: Int = 0
    while inner < 4
      inner = inner + 1
      if inner == 2 then
        continue
      end
      if outer == 2 then
        break
      end
      total = total + 1
    end
  end
  total
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := CompileDirNoPrelude(dir)
	if err != nil {
		t.Fatalf("CompileDirNoPrelude() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "loop_test.go"), []byte(`package sample

import "testing"

func TestScore(t *testing.T) {
	if got := Score(); got != 6 {
		t.Fatalf("Score() = %d, want 6", got)
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".", "-timeout=3s")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated nested-loop package failed: %v\n%s", err, output)
	}
}

func TestCompileDirRunsNestedTupleBindingAndStatementSwitch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module sample\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte(`package sample

func Pair() -> (Int, (Int, Int))
  (1, (2, 3))
end

func Result() -> Int
  let (a, (_, c)) = Pair()
  var value: Int = 0
  switch a
  case 1 then
    value = c
  end
  case _ then
    value = 99
  end
  end
  value
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDirNoPrelude(dir); err != nil {
		t.Fatalf("CompileDirNoPrelude() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tuple_switch_test.go"), []byte(`package sample

import "testing"

func TestResult(t *testing.T) {
	if got := Result(); got != 3 {
		t.Fatalf("Result() = %d, want 3", got)
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", ".", "-timeout=3s")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-bootstrap-gocache")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated tuple/switch package failed: %v\n%s", err, output)
	}
}

func TestCompileDirPreservesGoFFIPackageSelector(t *testing.T) {
	dir := t.TempDir()
	src := `package sample

import fmt "go:fmt"

func Render(value: Int) -> String
  fmt.Sprint(value)
end
`
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirNoPrelude(dir)
	if err != nil {
		t.Fatalf("CompileDirNoPrelude() error = %v", err)
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "fmt.Sprint(value)") {
		t.Fatalf("generated source lost Go FFI selector:\n%s", generated)
	}
}
func TestCompileDirRejectsAssignmentToPackageLevelLet(t *testing.T) {
	dir := t.TempDir()
	src := `package sample

let limit: Int = 10

func Change() -> Int
  limit = 20
  limit
end
`
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDir(dir); err == nil || !strings.Contains(err.Error(), "immutable binding \"limit\"") {
		t.Fatalf("CompileDir() error = %v, want immutable package binding error", err)
	}
}

func TestCompileDirSupportsTypeAlias(t *testing.T) {
	dir := t.TempDir()
	src := `package sample
type UserID = Int

func Next(id: UserID) -> UserID
  id + 1
end
`
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "type UserID = int") {
		t.Fatalf("generated source missing Go type alias:\n%s", generated)
	}
}

func TestCompileDirSupportsDistinctTypeDeclaration(t *testing.T) {
	dir := t.TempDir()
	src := `package sample
type UserID Int

func Identity(id: UserID) -> UserID
  id
end
`
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "type UserID int") || strings.Contains(string(generated), "type UserID = int") {
		t.Fatalf("generated source did not preserve a distinct Go type:\n%s", generated)
	}
}

func TestDistinctTypeIsNotInterchangeableWithUnderlyingType(t *testing.T) {
	dir := t.TempDir()
	src := `package sample
type UserID Int

func AsInt(id: UserID) -> Int
  id
end
`
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileDir(dir); err == nil || !strings.Contains(err.Error(), "cannot unify UserID with Int") {
		t.Fatalf("CompileDir() error = %v, want distinct-type mismatch", err)
	}
}

func TestCompileDirSupportsGenericTypeAlias(t *testing.T) {
	dir := t.TempDir()
	src := `package sample
type Items[A] = Slice[A]

func Identity[A](items: Items[A]) -> Items[A]
  items
end

func AsSlice(items: Items[Int]) -> Slice[Int]
  items
end
`
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "type Items[A any] = []A") {
		t.Fatalf("generated source missing generic Go type alias:\n%s", generated)
	}
}

func TestCompileDirSupportsGenericDistinctTypeDeclaration(t *testing.T) {
	dir := t.TempDir()
	src := `package sample
type Box[A] Slice[A]

func Identity[A](box: Box[A]) -> Box[A]
  box
end
`
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "type Box[A any] []A") {
		t.Fatalf("generated source missing generic distinct type:\n%s", generated)
	}
}
