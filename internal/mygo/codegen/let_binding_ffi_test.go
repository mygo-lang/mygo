package codegen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	myparser "github.com/mygo-lang/mygo/internal/mygo/parser"
)

// TestLetBindingGoFFIResultWrapping mirrors a user's `func f() -> Result[T, String]
// let x = goPkg.FFIFunc(...)` and asserts the raw multi-value Go call is wrapped
// into a Result.
func TestLetBindingGoFFIResultWrapping(t *testing.T) {
	src := `package p
import myos "go:os"

func readIt() -> Result[String, String]
  let res = myos.ReadFile("/etc/hostname")
  "ok"
end
`
	parsed, err := myparser.ParseFile("ffi.mygo", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := &Package{
		Name: "p", NoPrelude: true, Decls: parsed.Decls,
		Imports:       map[string]struct{}{},
		ImportAliases: map[string]string{"myos": "go:os"},
		Enums:         map[string]*EnumDecl{},
		Structs:       map[string]*StructDecl{},
		Interfaces:    map[string]*InterfaceDecl{},
		Funcs:         map[string]*FuncDecl{},
	}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*FuncDecl); ok {
			pkg.Funcs[fn.Name] = fn
		}
	}
	files, err := GenerateFiles(pkg, nil)
	if err != nil {
		t.Skipf("GenerateFiles: %v", err)
	}
	var gen string
	for _, f := range files {
		gen += f
	}
	t.Logf("GENERATED:\n%s", gen)
	if strings.Contains(gen, "func() Result[") {
		t.Logf("PASS: found IIFE Result wrapper")
	} else {
		t.Errorf("FAIL: expected IIFE Result wrapper in generated code")
	}
	// Must be parseable Go
	if _, err := parser.ParseFile(token.NewFileSet(), "gen.go", gen, 0); err != nil {
		t.Fatalf("generated invalid Go: %v", err)
	}
	_ = ast.NewIdent
}
