package parser

import (
	"fmt"
	"os"
	"testing"
)

func TestParsePrelude(t *testing.T) {
	src, err := os.ReadFile("../../../prelude/prelude.mygo")
	if err != nil {
		t.Fatal(err)
	}
	f, err := ParseFile("test.mygo", string(src))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("OK: %d decls, package=%s\n", len(f.Decls), f.PackageName)
}

func TestParseGenericImpl(t *testing.T) {
	src := "package p\n\nimpl[T] List[T]: IEnumerable[List[T], T]\n  func Map[B](c: List[T], fn: func(T) -> B) -> List[B]\n    var headVal: B = fn(c.head)\n  end\nend\n"
	f, err := ParseFile("test.mygo", src)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("OK: %d decls, package=%s\n", len(f.Decls), f.PackageName)
	for _, d := range f.Decls {
		if impl, ok := d.(*ImplDecl); ok {
			fmt.Printf("impl %s: %d methods\n", impl.InterfaceName, len(impl.Methods))
			for _, m := range impl.Methods {
				fmt.Printf("  method %s[%v]: params=%v, typeparams=%v\n", m.Name, m.TypeParams, m.Params, m.TypeParams)
			}
		}
	}
}

func TestParseFullPrelude(t *testing.T) {
	src, err := os.ReadFile("../../../prelude/prelude.mygo")
	if err != nil {
		t.Fatal(err)
	}
	f, err := ParseFile("test.mygo", string(src))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("OK: %d decls, package=%s\n", len(f.Decls), f.PackageName)
}
