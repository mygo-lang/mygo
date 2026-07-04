package parser

import (
	"fmt"
	"testing"
)

func TestParseInterface(t *testing.T) {
	src := "package p\n\ninterface Show[A]\n  func show(value: A) -> String\nend\n"
	f, err := ParseFile(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		fmt.Printf("Decl type: %T\n", d)
		if iface, ok := d.(*InterfaceDecl); ok {
			fmt.Printf("  Interface: %s, TypeParams: %v, Methods: %d\n", iface.Name, iface.TypeParams, len(iface.Methods))
			for _, m := range iface.Methods {
				fmt.Printf("    Method: %s\n", m.Name)
			}
		}
	}
}
