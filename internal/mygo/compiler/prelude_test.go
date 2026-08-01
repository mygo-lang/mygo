package compiler

import (
	"os"
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
	written, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	text := string(generated)
	if !strings.Contains(text, "var limit int = 10") || !strings.Contains(text, "var count int = 1") || !strings.Contains(text, "count = count + 1") {
		t.Fatalf("generated source missing package bindings or their use:\n%s", text)
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
