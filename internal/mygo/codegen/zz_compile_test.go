package codegen

import (
	"go/ast"
	"go/importer"
	goparser "go/parser"
	"go/token"
	"go/types"
	"strconv"
	"strings"
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/codegen/goast"
	myparser "github.com/mygo-lang/mygo/internal/mygo/parser"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference"
)

func TestGenerateMutualTailcallTrampolinePreservesWrappers(t *testing.T) {
	src := `package p

func Even(n: Int) -> Bool
  if n == 0 => true else Odd(n - 1)
end

func Odd(n: Int) -> Bool
  if n == 0 => false else Even(n - 1)
end
`
	parsed, err := myparser.ParseFile("mutual.mygo", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := &Package{
		Name: "p", NoPrelude: true, Decls: parsed.Decls,
		Imports: map[string]struct{}{}, ImportAliases: map[string]string{},
		Enums: map[string]*EnumDecl{}, Structs: map[string]*StructDecl{},
		Interfaces: map[string]*InterfaceDecl{}, Funcs: map[string]*FuncDecl{},
	}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*FuncDecl); ok {
			pkg.Funcs[fn.Name] = fn
		}
	}
	planGen := newGen(pkg, nil)
	edgeCount := 0
	for _, fn := range pkg.Funcs {
		mtCollectExpr(fn.Body, true, pkg.Funcs, func(*FuncDecl) { edgeCount++ })
	}
	if edgeCount != 2 {
		t.Fatalf("tail edge count = %d, want 2", edgeCount)
	}
	planGen.buildMutualTailPlans()
	if len(planGen.mutualTail) != 2 {
		t.Fatalf("mutual-tail plan count = %d, want 2", len(planGen.mutualTail))
	}
	files, err := GenerateFiles(pkg, nil)
	if err != nil {
		t.Fatal(err)
	}
	var generated string
	for _, file := range files {
		generated += file
	}
	if !strings.Contains(generated, "func Even(n int) bool") || !strings.Contains(generated, "func Odd(n int) bool") {
		t.Fatalf("original function wrappers missing:\n%s", generated)
	}
	if strings.Count(generated, "func __mygo_mt_p_") != 1 || !strings.Contains(generated, "__mygo_state") {
		t.Fatalf("mutual trampoline missing:\n%s", generated)
	}
	if strings.Contains(generated, "= Odd(n-1)") || strings.Contains(generated, "= Even(n-1)") || !strings.Contains(generated, "continue") {
		t.Fatalf("tail calls were not rewritten inside the trampoline:\n%s", generated)
	}
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, "generated.go", generated, goparser.AllErrors)
	if err != nil {
		t.Fatalf("generated Go did not parse: %v\n%s", err, generated)
	}
	conf := types.Config{Importer: importer.Default()}
	if _, err := conf.Check("p", fset, []*ast.File{file}, nil); err != nil {
		t.Fatalf("generated Go did not type-check: %v\n%s", err, generated)
	}
}

func TestGenerateSelfTailcallTrampoline(t *testing.T) {
	src := `package p

func Countdown(n: Int, total: Int) -> Int
  if n == 0 => total else Countdown(n - 1, total + n)
end
`
	parsed, err := myparser.ParseFile("self_tail.mygo", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := &Package{Name: "p", NoPrelude: true, Decls: parsed.Decls, Imports: map[string]struct{}{}, ImportAliases: map[string]string{}, Enums: map[string]*EnumDecl{}, Structs: map[string]*StructDecl{}, Interfaces: map[string]*InterfaceDecl{}, Funcs: map[string]*FuncDecl{}}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*FuncDecl); ok {
			pkg.Funcs[fn.Name] = fn
		}
	}
	files, err := GenerateFiles(pkg, nil)
	if err != nil {
		t.Fatal(err)
	}
	var generated string
	for _, file := range files {
		generated += file
	}
	if !strings.Contains(generated, "func Countdown(n int, total int) int") || !strings.Contains(generated, "func __mygo_mt_p_countdown") {
		t.Fatalf("self-tail wrapper or trampoline missing:\n%s", generated)
	}
	if strings.Contains(generated, "= Countdown(n-1, total+n)") || !strings.Contains(generated, "continue") {
		t.Fatalf("self tail call was not rewritten to a loop:\n%s", generated)
	}
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, "generated.go", generated, goparser.AllErrors)
	if err != nil {
		t.Fatalf("generated Go did not parse: %v\n%s", err, generated)
	}
	if _, err := (&types.Config{Importer: importer.Default()}).Check("p", fset, []*ast.File{file}, nil); err != nil {
		t.Fatalf("generated Go did not type-check: %v\n%s", err, generated)
	}
}

func TestParsecPManyUsesClosureContinuationStateMachine(t *testing.T) {
	pkg := simpleLoadPackage("../../../lib/text/parsec", false)
	if pkg == nil {
		t.Fatal("failed to load parsec package")
	}
	// simpleLoadPackage intentionally loads every .mygo file. The parsec test
	// source requires its test-only Go bindings, which are irrelevant here.
	decls := make([]Decl, 0, len(pkg.Decls))
	pkg.Funcs = map[string]*FuncDecl{}
	for _, decl := range pkg.Decls {
		if fn, ok := decl.(*FuncDecl); ok {
			if strings.HasSuffix(fn.SourceFile, "_test.mygo") {
				continue
			}
			pkg.Funcs[fn.Name] = fn
		}
		decls = append(decls, decl)
	}
	pkg.Decls = decls
	files, err := GenerateFiles(pkg, nil)
	if err != nil {
		t.Fatal(err)
	}
	var generated string
	for _, file := range files {
		generated += file
	}
	start := strings.Index(generated, "func PMany[")
	end := strings.Index(generated, "func PMany1[")
	if start < 0 || end < 0 {
		t.Fatalf("PMany declarations missing:\n%s", generated)
	}
	many := generated[start:end]
	if !strings.Contains(many, "__mygo_tcmc_stack") || !strings.Contains(many, "continue") {
		t.Fatalf("PMany was not lowered to a continuation state machine:\n%s", many)
	}
	if strings.Contains(many, "PMany(p)(") {
		t.Fatalf("PMany still recursively invokes its closure:\n%s", many)
	}
}

func TestMutualTailcallStagesAllArgumentsBeforeAssignments(t *testing.T) {
	src := `package p

func Left(a: Int, b: Int) -> Int
  if a == 0 => b else Right(b, a - 1)
end

func Right(a: Int, b: Int) -> Int
  if a == 0 => b else Left(b, a - 1)
end
`
	parsed, err := myparser.ParseFile("staging.mygo", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := &Package{Name: "p", NoPrelude: true, Decls: parsed.Decls, Imports: map[string]struct{}{}, ImportAliases: map[string]string{}, Enums: map[string]*EnumDecl{}, Structs: map[string]*StructDecl{}, Interfaces: map[string]*InterfaceDecl{}, Funcs: map[string]*FuncDecl{}}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*FuncDecl); ok {
			pkg.Funcs[fn.Name] = fn
		}
	}
	files, err := GenerateFiles(pkg, nil)
	if err != nil {
		t.Fatal(err)
	}
	var generated string
	for _, file := range files {
		generated += file
	}
	firstAssign := strings.Index(generated, "a = __mygo_next_0")
	secondTemp := strings.Index(generated, "__mygo_next_1 :=")
	if firstAssign < 0 || secondTemp < 0 || secondTemp > firstAssign {
		t.Fatalf("tail-call arguments were not fully staged before parameter updates:\n%s", generated)
	}
}

func TestGenerateGenericMutualTailcallTrampoline(t *testing.T) {
	src := `package p

func First[A](n: Int, value: A) -> A
  if n == 0 => value else Second[A](n - 1, value)
end

func Second[A](n: Int, value: A) -> A
  if n == 0 => value else First[A](n - 1, value)
end
`
	parsed, err := myparser.ParseFile("generic_mutual.mygo", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := &Package{Name: "p", NoPrelude: true, Decls: parsed.Decls, Imports: map[string]struct{}{}, ImportAliases: map[string]string{}, Enums: map[string]*EnumDecl{}, Structs: map[string]*StructDecl{}, Interfaces: map[string]*InterfaceDecl{}, Funcs: map[string]*FuncDecl{}}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*FuncDecl); ok {
			pkg.Funcs[fn.Name] = fn
		}
	}
	files, err := GenerateFiles(pkg, nil)
	if err != nil {
		t.Fatal(err)
	}
	var generated string
	for _, file := range files {
		generated += file
	}
	if !strings.Contains(generated, "func __mygo_mt_p_first_second[A any]") {
		t.Fatalf("generic mutual trampoline missing:\n%s", generated)
	}
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, "generated.go", generated, goparser.AllErrors)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&types.Config{Importer: importer.Default()}).Check("p", fset, []*ast.File{file}, nil); err != nil {
		t.Fatalf("generic generated Go did not type-check: %v\n%s", err, generated)
	}
}

func TestMutualTailcallSkipsNonTailIntraGroupCall(t *testing.T) {
	src := `package p

func A(n: Int) -> Int
  if n == 0 => 0 else if n == 1 => B(n - 1) else B(n - 1) + 1
end

func B(n: Int) -> Int
  A(n)
end
`
	parsed, err := myparser.ParseFile("non_tail.mygo", src)
	if err != nil {
		t.Fatal(err)
	}
	pkg := &Package{Name: "p", NoPrelude: true, Decls: parsed.Decls, Imports: map[string]struct{}{}, ImportAliases: map[string]string{}, Enums: map[string]*EnumDecl{}, Structs: map[string]*StructDecl{}, Interfaces: map[string]*InterfaceDecl{}, Funcs: map[string]*FuncDecl{}}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*FuncDecl); ok {
			pkg.Funcs[fn.Name] = fn
		}
	}
	files, err := GenerateFiles(pkg, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.Contains(file, "func __mygo_mt_p_a_b") {
			t.Fatalf("non-tail intra-group call must retain direct lowering:\n%s", file)
		}
	}
}

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

func TestGenerateInfersGenericCallTypeArgsFromReturnType(t *testing.T) {
	src := `package p

func Zero[A]() -> A
  go[A] {
    code: """
      func() {A} {
        var zero {A}
        return zero
      }()
    """
    type A = A
  }
end

func MakeInt() -> Int
  Zero()
end

func MakeGeneric[A]() -> A
  Zero()
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
		if fn, ok := decl.(*FuncDecl); ok {
			pkg.Funcs[fn.Name] = fn
		}
	}

	generated, err := Generate(pkg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(generated, "return Zero[int]()") {
		t.Fatalf("generated code should infer Zero type arg from return type:\n%s", generated)
	}
	if !strings.Contains(generated, "return Zero[A]()") {
		t.Fatalf("generated code should infer Zero type arg from generic return type:\n%s", generated)
	}
}

func TestGenericCallInfersTupleTypeArgumentFromArgument(t *testing.T) {
	pairs := &IdentExpr{Name: "pairs"}
	fn := &FuncDecl{
		Name:       "sliceDrop",
		TypeParams: []string{"A"},
		Params: []Param{{Name: "items", Type: &NamedType{Name: "Slice", Args: []TypeExpr{
			&NamedType{Name: "A"},
		}}}},
	}
	g := &gen{typedInfo: &typeinference.TypedInfo{ExprTypes: map[Expr]typeinference.MonoType{
		pairs: typeinference.TCon{Name: "Slice", Args: []typeinference.MonoType{
			typeinference.TCon{Name: "Tuple", Args: []typeinference.MonoType{
				typeinference.TCon{Name: "ast2.Expr"},
				typeinference.TCon{Name: "ast2.Expr"},
			}},
		}},
	}}}

	typeArgs := g.funcTypeArgExprsFromArgs(fn, []Expr{pairs}, &egCtx{})
	if len(typeArgs) != 1 {
		t.Fatalf("got %d inferred type arguments, want 1", len(typeArgs))
	}
	tuple, ok := typeArgs[0].(*ast.StructType)
	if !ok || tuple.Fields == nil || len(tuple.Fields.List) != 2 {
		t.Fatalf("tuple type argument = %T, want two-field anonymous struct", typeArgs[0])
	}
	for i, field := range tuple.Fields.List {
		if len(field.Names) != 1 || field.Names[0].Name != "F"+strconv.Itoa(i) {
			t.Fatalf("tuple field %d = %#v, want F%d", i, field.Names, i)
		}
		selector, ok := field.Type.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Expr" {
			t.Fatalf("tuple field %d type = %T, want ast2.Expr", i, field.Type)
		}
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
