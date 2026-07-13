package compiler

import (
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
