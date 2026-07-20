package compiler

import (
	"os"
	"path/filepath"
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
