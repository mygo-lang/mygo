package compiler

import (
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference"
)

func TestCompilePrelude(t *testing.T) {
	pkg, err := loadPackage("../../../prelude", true)
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

	// Find the Option Enumerable impl and test genImpl
	var optionEnumImpl *ImplDecl
	for _, impl := range pkg.Impls {
		if impl.InterfaceName == "Enumerable" && impl.Type != nil {
			if namedType, ok := impl.Type.(*NamedType); ok && namedType.Name == "Option" {
				optionEnumImpl = impl
				break
			}
		}
	}
	if optionEnumImpl == nil {
		t.Fatal("Option Enumerable impl not found in prelude")
	}

	code, err := g.genImpl(optionEnumImpl)
	if err != nil {
		t.Fatalf("genImpl(Option Enumerable): %v", err)
	}
	t.Logf("Option Enumerable impl generated: %d items", len(code))

	// Also test optionFlatMap (standalone function)
	fnDecl := pkg.Funcs["optionFlatMap"]
	code2, err := g.genFunc(fnDecl)
	if err != nil {
		t.Fatalf("genFunc(optionFlatMap): %v", err)
	}
	t.Logf("optionFlatMap generated: %T", code2)
}
