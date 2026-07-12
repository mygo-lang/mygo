package compiler

import (
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference"
)

func TestCompilePrelude(t *testing.T) {
	pkg, err := loadPackage("../../../lib/prelude", true)
	if err != nil {
		t.Fatal(err)
	}
	typedInfo, err := typeinference.InferPackage(&typeinference.PkgInfo{
		Name:       pkg.Name,
		Decls:      pkg.Decls,
		Enums:      pkg.Enums,
		Structs:    pkg.Structs,
		Interfaces: pkg.Interfaces,
		Funcs:      pkg.Funcs,
		Impls:      pkg.Impls,
	}, typeinference.NewInferState())
	if err != nil {
		t.Fatal(err)
	}

	g := &generator{
		pkg:               pkg,
		importAliases:     pkg.ImportAliases,
		interfaceByMethod: map[string]string{},
		variantByName:     map[string]string{},
		goSigCache:        map[string]*goPackageSigs{},
		typedInfo:         typedInfo,
	}
	for name, iface := range pkg.Interfaces {
		for _, m := range iface.Methods {
			g.interfaceByMethod[m.Name] = name
		}
	}
	for enumName, enum := range pkg.Enums {
		for _, variant := range enum.Variants {
			g.variantByName[variant.Name] = enumName
		}
	}

	// Find the Option IEnumerable impl and test genImpl
	var optionEnumImpl *ImplDecl
	for _, impl := range pkg.Impls {
		if impl.InterfaceName != "IEnumerable" {
			continue
		}
		typeArgs := impl.InterfaceArgs
		if len(typeArgs) == 0 {
			typeArgs = impl.TypeArgs
		}
		if len(typeArgs) == 0 {
			continue
		}
		if namedType, ok := typeArgs[0].(*NamedType); ok && namedType.Name == "Option" {
			optionEnumImpl = impl
			break
		}
	}
	if optionEnumImpl == nil {
		t.Fatal("Option IEnumerable impl not found in prelude")
	}

	code, err := g.genImpl(optionEnumImpl)
	if err != nil {
		t.Fatalf("genImpl(Option IEnumerable): %v", err)
	}
	t.Logf("Option IEnumerable impl generated: %d items", len(code))

	// Also test OptionFlatMap (standalone function)
	fnDecl := pkg.Funcs["OptionFlatMap"]
	code2, err := g.genFunc(fnDecl)
	if err != nil {
		t.Fatalf("genFunc(OptionFlatMap): %v", err)
	}
	t.Logf("OptionFlatMap generated: %T", code2)
}

func TestLoadPreludeDoesNotDuplicatePreludeDecls(t *testing.T) {
	withPrelude, err := loadPackage("../../../lib/prelude", false)
	if err != nil {
		t.Fatal(err)
	}
	withoutPrelude, err := loadPackage("../../../lib/prelude", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(withPrelude.Decls) != len(withoutPrelude.Decls) {
		t.Fatalf("loadPackage(prelude, false) added extra decls: got %d, want %d", len(withPrelude.Decls), len(withoutPrelude.Decls))
	}
	if len(withPrelude.Funcs) != len(withoutPrelude.Funcs) {
		t.Fatalf("loadPackage(prelude, false) added extra funcs: got %d, want %d", len(withPrelude.Funcs), len(withoutPrelude.Funcs))
	}
	if len(withPrelude.Impls) != len(withoutPrelude.Impls) {
		t.Fatalf("loadPackage(prelude, false) added extra impls: got %d, want %d", len(withPrelude.Impls), len(withoutPrelude.Impls))
	}
}
