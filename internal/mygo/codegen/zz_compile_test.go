package codegen

import (
	"strings"
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/codegen/goast"
	myparser "github.com/mygo-lang/mygo/internal/mygo/parser"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference"
)

func TestGenerateResolvesGenericSliceFoldThroughTypeclass(t *testing.T) {
	src := `package parsec

interface IEnumerable[C[A], A]
  func Fold[B](c: C[A], initial: B, fn: func(B, A) -> B) -> B
end

impl[T] SliceIEnumerable[T]: IEnumerable[Slice[T], T]
  func Fold[B](c: Slice[T], initial: B, fn: func(B, T) -> B) -> B
    initial
  end
end

struct Parser[A]
  value: A
end

func PFail[A](message: String) -> Parser[A]
  Parser[A] { value: Zero() }
end

func POrElse[A](left: Parser[A], right: Parser[A]) -> Parser[A]
  left
end

func PChoice[A](parsers: Slice[Parser[A]]) -> Parser[A]
  parsers.Fold(PFail[A]("no parser matched"), func(acc: Parser[A], p: Parser[A]) -> Parser[A]
    POrElse(acc, p)
  end)
end
`
	parsed, err := myparser.ParseFile("parsec.mygo", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := &Package{
		Name:          "parsec",
		NoPrelude:     true,
		Imports:       map[string]struct{}{},
		ImportAliases: map[string]string{},
		Enums:         map[string]*EnumDecl{},
		Structs:       map[string]*StructDecl{},
		Interfaces:    map[string]*InterfaceDecl{},
		Funcs:         map[string]*FuncDecl{},
		Decls:         parsed.Decls,
	}
	for _, decl := range parsed.Decls {
		switch d := decl.(type) {
		case *StructDecl:
			pkg.Structs[d.Name] = d
		case *InterfaceDecl:
			pkg.Interfaces[d.Name] = d
		case *FuncDecl:
			pkg.Funcs[d.Name] = d
		case *ImplDecl:
			pkg.Impls = append(pkg.Impls, d)
		}
	}

	files, err := GenerateFiles(pkg, nil)
	if err != nil {
		t.Fatal(err)
	}
	var generated string
	for _, src := range files {
		generated += src
	}
	if strings.Contains(generated, "parsers.Fold") {
		t.Fatalf("PChoice generated direct selector call, want typeclass helper:\n%s", generated)
	}
	if !strings.Contains(generated, "MygoIT") ||
		!strings.Contains(generated, "16SliceIEnumerable") ||
		!strings.Contains(generated, "M4Fold") {
		t.Fatalf("PChoice did not generate SliceIEnumerable Fold helper call:\n%s", generated)
	}
}

func TestInterfaceImplMethodNamesIncludeImplIdentity(t *testing.T) {
	src := `package p

interface Pretty[A]
  func Show(value: A) -> String
end

interface Debug[A]
  func Show(value: A) -> String
end

impl IntPretty: Pretty[Int]
  func Show(value: Int) -> String
    "pretty"
  end
end

impl IntDebug: Debug[Int]
  func Show(value: Int) -> String
    "debug"
  end
end
`
	parsed, err := myparser.ParseFile("p.mygo", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := &Package{
		Name:          "p",
		NoPrelude:     true,
		Imports:       map[string]struct{}{},
		ImportAliases: map[string]string{},
		Enums:         map[string]*EnumDecl{},
		Structs:       map[string]*StructDecl{},
		Interfaces:    map[string]*InterfaceDecl{},
		Funcs:         map[string]*FuncDecl{},
		Decls:         parsed.Decls,
	}
	for _, decl := range parsed.Decls {
		switch d := decl.(type) {
		case *InterfaceDecl:
			pkg.Interfaces[d.Name] = d
		case *ImplDecl:
			pkg.Impls = append(pkg.Impls, d)
		}
	}

	generated, err := Generate(pkg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(generated, "func MygoIT") != 2 {
		t.Fatalf("generated code should contain two distinct impl helpers:\n%s", generated)
	}
	if !strings.Contains(generated, "6Pretty") || !strings.Contains(generated, "5Debug") ||
		!strings.Contains(generated, "9IntPretty") || !strings.Contains(generated, "8IntDebug") {
		t.Fatalf("generated impl helper names should include interface and impl identity:\n%s", generated)
	}
	if strings.Count(generated, "M4Show") != 2 {
		t.Fatalf("generated impl helper names should include the method name:\n%s", generated)
	}
}

func TestCompilePrelude(t *testing.T) {
	pkg := simpleLoadPackage("../../../prelude", true)
	if pkg == nil {
		t.Fatal("failed to load prelude package")
	}

	// Build SourceFiles mapping for error messages.
	sourceFiles := make(map[any]string)
	for _, decl := range pkg.Decls {
		// All declarations in this package come from files in pkg.Dir.
		// We'll use the directory path as a placeholder since simpleLoadPackage
		// doesn't track individual file paths.
		sourceFiles[decl] = pkg.Dir
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

	g := NewGenerator(pkg, typedInfo)

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
}

func TestLoadPreludeDoesNotDuplicatePreludeDecls(t *testing.T) {
	withPrelude := simpleLoadPackage("../../../prelude", false)
	if withPrelude == nil {
		t.Fatal("failed to load prelude with prelude")
	}
	withoutPrelude := simpleLoadPackage("../../../prelude", true)
	if withoutPrelude == nil {
		t.Fatal("failed to load prelude without prelude")
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

func TestGoTypeTranslatesByteAndRune(t *testing.T) {
	g := NewGenerator(&Package{Name: "main"}, nil).toGen()
	if got := g.goType(&NamedType{Name: "Byte"}, nil); got != "byte" {
		t.Fatalf("goType(Byte) = %q, want byte", got)
	}
	if got := g.goType(&NamedType{Name: "Rune"}, nil); got != "rune" {
		t.Fatalf("goType(Rune) = %q, want rune", got)
	}
}

func TestGoStringToMyGoPreservesRune(t *testing.T) {
	cases := []string{"string", "bool", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "byte", "rune", "float32", "float64", "any", "struct{}"}
	for _, tc := range cases {
		if got := goast.GoStringToMyGo(tc); got != tc {
			t.Fatalf("GoStringToMyGo(%s) = %q, want %q", tc, got, tc)
		}
	}
}
