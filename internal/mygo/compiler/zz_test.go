package compiler

import (
	"testing"
)

func TestCompilePrelude(t *testing.T) {
	pkg, err := loadPackage("../../../prelude")
	if err != nil {
		t.Fatal(err)
	}
	
	// Test optionMap only
	g := &generator{
		pkg:               pkg,
		importAliases:     pkg.ImportAliases,
		interfaceByMethod: map[string]string{},
		variantByName:     map[string]string{},
		goSigCache:        map[string]*goPackageSigs{},
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
	
	// Test optionMap
	fnDecl := pkg.Funcs["optionMap"]
	code, err := g.genFunc(fnDecl)
	if err != nil {
		t.Fatalf("genFunc(optionMap): %v", err)
	}
	t.Logf("optionMap generated: %T", code)
	
	// Also test optionFlatMap (same pattern)
	fn2 := pkg.Funcs["optionFlatMap"]
	code2, err := g.genFunc(fn2)
	if err != nil {
		t.Fatalf("genFunc(optionFlatMap): %v", err)
	}
	t.Logf("optionFlatMap generated: %T", code2)
}
