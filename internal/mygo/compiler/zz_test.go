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

	// Find the String IEnumerable impl and test genImpl.
	var stringEnumImpl *ImplDecl
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
		if namedType, ok := typeArgs[0].(*NamedType); ok && namedType.Name == "String" {
			stringEnumImpl = impl
			break
		}
	}
	if stringEnumImpl == nil {
		t.Fatal("String IEnumerable impl not found in prelude")
	}

	code, err := g.genImpl(stringEnumImpl)
	if err != nil {
		t.Fatalf("genImpl(String IEnumerable): %v", err)
	}
	t.Logf("String IEnumerable impl generated: %d items", len(code))

	// Also test String.PeekRune, which relies on String unifying as C[rune].
	var peekRuneImpl *ImplDecl
	for _, impl := range pkg.Impls {
		if impl.InterfaceName != "" || impl.Name != "" {
			continue
		}
		namedType, ok := impl.Type.(*NamedType)
		if !ok || namedType.Name != "String" {
			continue
		}
		for _, m := range impl.Methods {
			if m.Name == "PeekRune" {
				peekRuneImpl = impl
				break
			}
		}
		if peekRuneImpl != nil {
			break
		}
	}
	if peekRuneImpl == nil {
		t.Fatal("PeekRune method not found in String impl")
	}
	code2, err := g.genImpl(peekRuneImpl)
	if err != nil {
		t.Fatalf("genImpl(String.PeekRune): %v", err)
	}
	t.Logf("PeekRune generated: %d items", len(code2))
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
