package metadata

import (
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/abi"
	"github.com/mygo-lang/mygo/internal/mygo/ast2"
)

func TestExportedDeclsRebuildsPackageValues(t *testing.T) {
	value := &PackageMetadata{
		Package: PackageIdentity{Name: "dep", ImportPath: "example.com/dep"},
		Exports: Exports{Values: []ValueExport{
			{Name: "DefaultPort", Type: Type{Kind: "builtin", Name: "int"}},
			{Name: "Default", Type: Type{Kind: "named", Name: "Box", Package: "example.com/dep"}},
			{Name: "Active", Type: Type{Kind: "builtin", Name: "bool"}, Mutable: true},
		}},
	}
	decls, err := value.ExportedDecls()
	if err != nil {
		t.Fatalf("ExportedDecls failed: %v", err)
	}
	found := map[string]ast2.Bind{}
	for _, decl := range decls {
		switch {
		case ast2.DeclIsLet(decl):
			found[ast2.DeclLetBind(decl).Name] = ast2.DeclLetBind(decl)
		case ast2.DeclIsVar(decl):
			found[ast2.DeclVarBind(decl).Name] = ast2.DeclVarBind(decl)
		}
	}
	port, ok := found["DefaultPort"]
	if !ok || !abi.HasSome[ast2.TypeExpr](port.Type) {
		t.Fatalf("DefaultPort was not rebuilt with a type")
	}
	portType := abi.ValueOr[ast2.TypeExpr](port.Type, ast2.UnitTypeExpr())
	if !ast2.TypeIsNamed(portType) || ast2.TypeNamedName(portType) != "Int" {
		t.Fatalf("DefaultPort type = %#v, want Int", portType)
	}
	def, ok := found["Default"]
	if !ok || !abi.HasSome[ast2.TypeExpr](def.Type) {
		t.Fatalf("Default was not rebuilt with a type")
	}
	defType := abi.ValueOr[ast2.TypeExpr](def.Type, ast2.UnitTypeExpr())
	if !ast2.TypeIsNamed(defType) || ast2.TypeNamedName(defType) != "Box" {
		t.Fatalf("Default type = %#v, want Box", defType)
	}
	active, ok := found["Active"]
	if !ok || !abi.HasSome[ast2.TypeExpr](active.Type) {
		t.Fatalf("Active was not rebuilt as a var with a type")
	}
}

func TestExportedDeclsWithoutValuesKeepsOldBehavior(t *testing.T) {
	value := &PackageMetadata{
		Package: PackageIdentity{Name: "dep", ImportPath: "example.com/dep"},
		Exports: Exports{},
	}
	decls, err := value.ExportedDecls()
	if err != nil {
		t.Fatalf("ExportedDecls failed: %v", err)
	}
	for _, decl := range decls {
		if ast2.DeclIsLet(decl) || ast2.DeclIsVar(decl) {
			t.Fatalf("metadata without Values produced a package value declaration")
		}
	}
}
