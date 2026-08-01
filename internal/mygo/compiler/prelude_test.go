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
