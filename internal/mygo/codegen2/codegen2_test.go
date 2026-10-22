package codegen2

import (
	"encoding/json"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	goast "github.com/mygo-lang/mygo/internal/mygo/codegen2/goast"
	"github.com/mygo-lang/mygo/internal/mygo/common2"
	"github.com/mygo-lang/mygo/internal/mygo/metadata"
	"github.com/mygo-lang/mygo/internal/mygo/parser2"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
	. "github.com/mygo-lang/mygo/prelude"
)

func TestDynamicDispatchSourceReferencesProducerAdapterDirectly(t *testing.T) {
	contract := metadata.PackageMetadata{
		Format: metadata.FormatName, SchemaVersion: metadata.SchemaVersion, CompilerABI: metadata.CompilerABI, AdapterABI: metadata.AdapterABI,
		Package: metadata.PackageIdentity{Name: "contracts", ImportPath: "example.com/contracts"},
		Exports: metadata.Exports{Interfaces: []metadata.InterfaceExport{{Name: "Show", TypeParams: []metadata.TypeParam{{Name: "A", Scope: "interface:Show", Index: 0}}, Methods: []metadata.Method{
			{Name: "Show", Parameters: []metadata.Field{{Name: "value", Type: metadata.Type{Kind: "typeParam", Name: "A", Scope: "interface:Show", Index: 0}}}, Results: []metadata.Type{{Kind: "builtin", Name: "string"}}},
			{Name: "Echo", TypeParams: []metadata.TypeParam{{Name: "B", Scope: "method:Show:Echo", Index: 0}}, Parameters: []metadata.Field{{Name: "value", Type: metadata.Type{Kind: "typeParam", Name: "A", Scope: "interface:Show", Index: 0}}, {Name: "item", Type: metadata.Type{Kind: "typeParam", Name: "B", Scope: "method:Show:Echo", Index: 0}}}, Results: []metadata.Type{{Kind: "typeParam", Name: "B", Scope: "method:Show:Echo", Index: 0}}},
		}}}},
	}
	backend := metadata.PackageMetadata{
		Format: metadata.FormatName, SchemaVersion: metadata.SchemaVersion, CompilerABI: metadata.CompilerABI, AdapterABI: metadata.AdapterABI,
		Package: metadata.PackageIdentity{Name: "backend", ImportPath: "example.com/backend"},
		Imports: []metadata.PackageImport{{Alias: "contracts", Path: "example.com/contracts"}},
		Implementations: []metadata.Implementation{{
			ID: "sha256:1234567890abcdef", Interface: metadata.TypeclassID{Package: "example.com/contracts", Name: "Show", Arguments: []metadata.Type{{Kind: "named", Name: "Widget", Package: "example.com/backend"}}},
			Target: metadata.Type{Kind: "named", Name: "Widget", Package: "example.com/backend"},
			Methods: []metadata.Adapter{
				{Method: "Show", Symbol: "MygoDynamicAdapterShowWidget", HiddenParameterIndex: 0, LogicalParameters: []metadata.Field{{Name: "value", Type: metadata.Type{Kind: "named", Name: "Widget", Package: "example.com/backend"}}}, LogicalResults: []metadata.Type{{Kind: "builtin", Name: "string"}}, GoParameters: []metadata.Type{{Kind: "builtin", Name: "any"}}, GoResults: []metadata.Type{{Kind: "builtin", Name: "string"}}, ABI: metadata.AdapterABI, Supported: true},
				{Method: "Echo", Symbol: "MygoDynamicAdapterEchoWidget", HiddenParameterIndex: 0, TypeParams: []metadata.TypeParam{{Name: "B", Scope: "method:Show:Echo", Index: 0}}, LogicalParameters: []metadata.Field{{Name: "value", Type: metadata.Type{Kind: "named", Name: "Widget", Package: "example.com/backend"}}, {Name: "item", Type: metadata.Type{Kind: "typeParam", Name: "B", Scope: "method:Show:Echo", Index: 0}}}, LogicalResults: []metadata.Type{{Kind: "typeParam", Name: "B", Scope: "method:Show:Echo", Index: 0}}, GoParameters: []metadata.Type{{Kind: "builtin", Name: "any"}, {Kind: "typeParam", Name: "B", Scope: "method:Show:Echo", Index: 0}}, GoResults: []metadata.Type{{Kind: "typeParam", Name: "B", Scope: "method:Show:Echo", Index: 0}}, ABI: metadata.AdapterABI, Supported: true},
			},
		}},
	}
	contractJSON, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	backendJSON, err := json.Marshal(backend)
	if err != nil {
		t.Fatal(err)
	}
	deps := []typeinference2.MyGoPackageInfo{
		{Alias: "contracts", Path: "example.com/contracts", DynamicMetadataJSON: string(contractJSON)},
		{Alias: "backend", Path: "example.com/backend", DynamicMetadataJSON: string(backendJSON)},
	}
	decls, err := dynamicDispatchDecls("app", nil, deps)
	if err != nil {
		t.Fatalf("dynamicDispatchDecls failed: %v", err)
	}
	if len(decls) != 6 { // imports, a wrapper alias, constructor, and two method APIs
		t.Fatalf("dynamic declarations = %d, want 6 (imports and APIs)", len(decls))
	}
	source, err := dynamicDispatchSource("app", nil, deps)
	if err != nil {
		t.Fatalf("dynamicDispatchSource failed: %v", err)
	}
	for _, want := range []string{
		"type MygoDynamicShow = contracts.MygoDynamicShow",
		"func MygoDynamicShowFromBackendWidget(payload backend.Widget) MygoDynamicShow",
		"contracts.NewMygoDynamicShow(payload, \"sha256:1234567890abcdef\")",
		"case \"sha256:1234567890abcdef\":",
		"backend.MygoDynamicAdapterShowWidget(contracts.MygoDynamicShowPayload(value))",
		"func MygoDynamicShowCallEcho[B any](value MygoDynamicShow, item B) B",
		"backend.MygoDynamicAdapterEchoWidget[B](contracts.MygoDynamicShowPayload(value), item)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("dynamic source missing %q:\n%s", want, source)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "dynamic.gen.go", source, parser.AllErrors); err != nil {
		t.Fatalf("dynamic source did not parse: %v\n%s", err, source)
	}
}

func TestDynamicInferenceImportPathRestoresGoFFIMarker(t *testing.T) {
	encoded, err := json.Marshal(metadata.PackageMetadata{
		Imports: []metadata.PackageImport{{Alias: "mail", Path: "net/mail", Go: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dependency := typeinference2.MyGoPackageInfo{DynamicMetadataJSON: string(encoded)}
	if got := dynamicInferenceImportPath("net/mail", nil, []typeinference2.MyGoPackageInfo{dependency}); got != "go:net/mail" {
		t.Fatalf("dynamic inference import path = %q, want go:net/mail", got)
	}
}

func TestDynamicAPIDiagnosticNamesUnsupportedDependencyMethod(t *testing.T) {
	parsed := parser2.ParseFileAt("caller.mygo", `package app
func Run() -> String
  MygoDynamicEqCallEquals(value)
end
`)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("parse caller: %v", parsed)
	}
	contracts := metadata.PackageMetadata{
		Format: metadata.FormatName, SchemaVersion: metadata.SchemaVersion, CompilerABI: metadata.CompilerABI, AdapterABI: metadata.AdapterABI,
		Package: metadata.PackageIdentity{Name: "contracts", ImportPath: "example.com/contracts"},
		Exports: metadata.Exports{Interfaces: []metadata.InterfaceExport{{Name: "Eq", TypeParams: []metadata.TypeParam{{Name: "A", Scope: "interface:Eq", Index: 0}}, Methods: []metadata.Method{{Name: "Equals"}}}}},
	}
	backend := metadata.PackageMetadata{
		Format: metadata.FormatName, SchemaVersion: metadata.SchemaVersion, CompilerABI: metadata.CompilerABI, AdapterABI: metadata.AdapterABI,
		Package: metadata.PackageIdentity{Name: "backend", ImportPath: "example.com/backend"},
		Implementations: []metadata.Implementation{{
			ID: "sha256:eq-widget", Interface: metadata.TypeclassID{Package: "example.com/contracts", Name: "Eq"},
			Target:  metadata.Type{Kind: "named", Name: "Widget", Package: "example.com/backend"},
			Methods: []metadata.Adapter{{Method: "Equals", Supported: false, UnsupportedReason: "requires a second hidden target value"}},
		}},
	}
	contractJSON, _ := json.Marshal(contracts)
	backendJSON, _ := json.Marshal(backend)
	files := []SourceFileInput{{Path: "caller.mygo", File: file.F0}}
	dependencies := []typeinference2.MyGoPackageInfo{
		{Alias: "contracts", Path: "example.com/contracts", DynamicMetadataJSON: string(contractJSON)},
		{Alias: "backend", Path: "example.com/backend", DynamicMetadataJSON: string(backendJSON)},
	}
	_, err := dynamicDispatchDecls("app", files, dependencies)
	if err == nil || !strings.Contains(err.Error(), "example.com/backend") || !strings.Contains(err.Error(), "second hidden target value") {
		t.Fatalf("dynamic diagnostic = %v, want dependency-specific unsupported-method error", err)
	}
}

func TestDynamicAPIDiagnosticReportsMissingMetadata(t *testing.T) {
	parsed := parser2.ParseFileAt("caller.mygo", `package app
func Run() -> String
  MygoDynamicShowFromBackendWidget(value)
end
`)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("parse caller: %v", parsed)
	}
	_, err := dynamicDispatchDecls("app", []SourceFileInput{{Path: "caller.mygo", File: file.F0}}, []typeinference2.MyGoPackageInfo{{Alias: "backend", Path: "example.com/backend"}})
	if err == nil || !strings.Contains(err.Error(), "example.com/backend") || !strings.Contains(err.Error(), "metadata is missing") {
		t.Fatalf("dynamic diagnostic = %v, want package-specific missing-metadata error", err)
	}
}

func TestDynamicAPIDiagnosticReportsMissingImplementation(t *testing.T) {
	parsed := parser2.ParseFileAt("caller.mygo", `package app
func Run() -> String
  MygoDynamicShowFromBackendWidgetShow(value)
end
`)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("parse caller: %v", parsed)
	}
	contract := metadata.PackageMetadata{
		Format: metadata.FormatName, SchemaVersion: metadata.SchemaVersion, CompilerABI: metadata.CompilerABI, AdapterABI: metadata.AdapterABI,
		Package: metadata.PackageIdentity{Name: "contracts", ImportPath: "example.com/contracts"},
		Exports: metadata.Exports{Interfaces: []metadata.InterfaceExport{{Name: "Show"}}},
	}
	encoded, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	_, err = dynamicDispatchDecls("app", []SourceFileInput{{Path: "caller.mygo", File: file.F0}}, []typeinference2.MyGoPackageInfo{{Alias: "contracts", Path: "example.com/contracts", DynamicMetadataJSON: string(encoded)}})
	if err == nil || !strings.Contains(err.Error(), "no implementation") || !strings.Contains(err.Error(), "example.com/contracts.Show") {
		t.Fatalf("dynamic diagnostic = %v, want package-specific missing-implementation error", err)
	}
}

func TestDynamicDispatchDiagnosesMissingInterfaceDependency(t *testing.T) {
	backend := metadata.PackageMetadata{
		Format: metadata.FormatName, SchemaVersion: metadata.SchemaVersion, CompilerABI: metadata.CompilerABI, AdapterABI: metadata.AdapterABI,
		Package: metadata.PackageIdentity{Name: "backend", ImportPath: "example.com/backend"},
		Implementations: []metadata.Implementation{{
			ID: "sha256:orphan", Interface: metadata.TypeclassID{Package: "example.com/contracts", Name: "Show"},
			Target: metadata.Type{Kind: "named", Name: "Widget", Package: "example.com/backend"},
		}},
	}
	encoded, err := json.Marshal(backend)
	if err != nil {
		t.Fatal(err)
	}
	_, err = dynamicDispatchSource("app", nil, []typeinference2.MyGoPackageInfo{{Alias: "backend", Path: "example.com/backend", DynamicMetadataJSON: string(encoded)}})
	if err == nil || !strings.Contains(err.Error(), "example.com/contracts") || !strings.Contains(err.Error(), "caller dependency graph") {
		t.Fatalf("dynamic dispatch diagnostic = %v, want missing interface dependency error", err)
	}
}

func TestGenerateSourceLowersNamedStructEnumVariant(t *testing.T) {
	src := `package sample

enum Shape
  Circle { radius: Float64 }
  Rectangle(Float64, Float64)
end

func area(shape: Shape) -> Float64
  switch shape
    case Circle { radius } => 3.14 * radius * radius
    case Rectangle(width, height) => width * height
  end
end

func CombinedArea() -> Float64
  let circle = Shape.Circle { radius: 2.0 }
  let rectangle = Shape.Rectangle(3.0, 4.0)
  area(circle) + area(rectangle)
end
`
	result := GenerateSource(src)
	code, ok := result.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", result)
	}
	t.Logf("Generated code:\n%s", code.F0)
	if !strings.Contains(code.F0, "Shape__Circle{") || !strings.Contains(code.F0, "radius: 2.0") {
		t.Fatalf("named enum construction was not lowered to Shape__Circle:\n%s", code.F0)
	}
}

func TestGenerateSourceLowersGenericNamedStructEnumVariant(t *testing.T) {
	result := GenerateSource(`package sample

enum Shape[A]
  Pair { left: A, right: A }
end

func make() -> Shape[Int]
  Shape.Pair { right: 2, left: 1 }
end
`)
	code, ok := result.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", result)
	}
	if !strings.Contains(code.F0, "Shape__Pair[int]{") {
		t.Fatalf("generic named enum construction was not lowered:\n%s", code.F0)
	}
}

func TestGenerateSourceLowersPipeOperators(t *testing.T) {
	result := GenerateSource(`package sample

func double(x: Int) -> Int
  x * 2
end

func inc(x: Int) -> Int
  x + 1
end

func identity[A](x: A) -> A
  x
end

func makeAdder(x: Int) -> func(Int) -> Int
  func(y: Int) -> Int
    x + y
  end
end

func applyForward(f: func(Int) -> Int, x: Int) -> Int
  x |> f
end

func sumInts(xs: Slice[Int]) -> String
  "ok"
end

func emptyArg() -> String
  [] |> sumInts
end

func forward() -> Int
  1 |> double |> inc
end

func backward() -> Int
  inc <| double <| 2
end

func mixed() -> Int
  inc <| 3 |> double
end

func generic() -> Int
  5 |> identity
end

func returned() -> Int
  5 |> makeAdder <| 3
end
`)
	code, ok := result.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", result)
	}
	for _, want := range []string{
		"f(x)",
		"inc(double(1))",
		"inc(double(2))",
		"inc(double(3))",
		"identity(5)",
		"makeAdder(5)(3)",
		"sumInts([]int{})",
	} {
		if !strings.Contains(code.F0, want) {
			t.Fatalf("generated pipe code missing %q:\n%s", want, code.F0)
		}
	}
}

func TestGenerateSourcePreservesPipeEvaluationOrder(t *testing.T) {
	result := GenerateSource(`package sample

func mark(label: String) -> Int
  go[Int] {
    code: "func() int { pipeOrder = append(pipeOrder, {label}); return 0 }()"
    in label = label
  }
end

func makeMarked(label: String) -> func(Int) -> Int
  let _ = mark(label)
  func(x: Int) -> Int
    x
  end
end

func markFn(label: String) -> func(Int) -> Int
  let _ = mark(label)
  func(x: Int) -> Int
    x
  end
end

func forwardOrder() -> Int
  let _ = mark("left") |> makeMarked("right")
  0
end

func backwardOrder(flag: Bool) -> Int
  let _ = markFn("left") <| (if flag then mark("right") else 0 end)
  0
end
`)
	code, ok := result.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", result)
	}
	left := strings.Index(code.F0, `mark("left")`)
	right := strings.Index(code.F0, `makeMarked("right")`)
	if left < 0 || right < 0 || left > right {
		t.Fatalf("forward pipe did not bind its left operand first:\n%s", code.F0)
	}
	backLeft := strings.Index(code.F0, `markFn("left")`)
	backRight := strings.Index(code.F0, "if flag")
	if backLeft < 0 || backRight < 0 || backLeft > backRight {
		t.Fatalf("backward pipe did not evaluate its function operand first:\n%s", code.F0)
	}
}

func TestSliceDropReturnsSuffixView(t *testing.T) {
	items := []int{1, 2, 3}
	got := common2.SliceDrop(items, 1)
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("common2.SliceDrop(items, 1) = %v, want [2 3]", got)
	}
	got[0] = 9
	if items[1] != 9 {
		t.Fatalf("SliceDrop copied its result: items = %v", items)
	}
	if got := common2.SliceDrop(items, -1); len(got) != len(items) {
		t.Fatalf("common2.SliceDrop(items, -1) length = %d, want %d", len(got), len(items))
	}
	if got := common2.SliceDrop(items, len(items)); len(got) != 0 {
		t.Fatalf("common2.SliceDrop(items, len(items)) = %v, want empty", got)
	}
}

func TestCanonicalMyGoTypeNameUsesSharedSpellingTable(t *testing.T) {
	cases := map[string]string{
		"int":     "Int",
		"uint8":   "UInt8",
		"string":  "String",
		"bool":    "Bool",
		"float64": "Float64",
		"byte":    "Byte",
		"rune":    "Rune",
		"error":   "Error",
		"any":     "Any",
	}
	for in, want := range cases {
		if got := canonicalMyGoTypeName(in); got != want {
			t.Fatalf("canonicalMyGoTypeName(%q) = %q, want %q", in, got, want)
		}
	}
	// Already-canonical names pass through untouched, and unknown names stay
	// unchanged rather than being rewritten by the shared table.
	for _, in := range []string{"Int", "Byte", "Rune", "Error", "MyType"} {
		if got := canonicalMyGoTypeName(in); got != in {
			t.Fatalf("canonicalMyGoTypeName(%q) = %q, want itself", in, got)
		}
	}
}

func TestGenerateSourceDispatchesMethodCallInsideAssignment(t *testing.T) {
	src := `package sample

impl[T] Slice[T]
  func Append(items: Slice[T], item: T) -> Slice[T]
    items
  end
end

func update() -> Slice[String]
  var items: Slice[String] = []
  items = items.Append("first")
  items
end
`
	generated := GenerateSource(src)
	code, ok := generated.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", generated)
	}
	if !strings.Contains(code.F0, `items = MygoIN5SliceM6Append(items, "first")`) {
		t.Fatalf("assignment did not dispatch Append through its impl helper:\n%s", code.F0)
	}
	if strings.Contains(code.F0, `items.Append("first")`) {
		t.Fatalf("assignment retained invalid Go selector call:\n%s", code.F0)
	}
}

func TestGenerateSourceDiscardsFFIReturnValuesInUnderscoreBinding(t *testing.T) {
	src := `package sample

import fmt "go:fmt"

func main() -> ()
  let _ = fmt.Println("ok")
end
`
	parsed := parseSourceAsAst2(src)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("parseSourceAsAst2 failed: %v", parsed)
	}
	main := file.F0.Decls[1].(ast2.Decl__FuncDecl)
	block := main.F4.Kind.(ast2.ExprKind__BlockExpr)
	bind := block.F0[0].(ast2.Stmt__LetStmt).F0
	ctx := &[]egCtx{newEgCtx()}[0]
	ctx.goPackages = []typeinference2.GoPackageEntry{{
		Alias: "fmt",
		Path:  "fmt",
		Funcs: []typeinference2.GoFuncSignature{{
			Name: "Println", Results: []string{"int", "error"}, Variadic: true,
		}},
	}}
	lowered := translateAstBinding(bind, ctx)
	result, ok := lowered.(Result__Ok[[]goast.Stmt, string])
	if !ok {
		t.Fatalf("discard lowering failed: %v", lowered)
	}
	if len(result.F0) != 1 {
		t.Fatalf("discard lowering emitted %d statements, want one direct call", len(result.F0))
	}
	exprStmt, ok := result.F0[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("discard lowering produced %T, want expression statement", result.F0[0])
	}
	if _, ok := exprStmt.X.(*ast.CallExpr); !ok {
		t.Fatalf("discard lowering produced %T, want direct Go call", exprStmt.X)
	}
}

func TestGenerateFilesKeepsInferredFFIResultTypeForSwitch(t *testing.T) {
	src := `package sample

import strconv "go:strconv"

enum Result[A, E]
  Ok(A)
  Err(E)
end

func parse() -> Int
  let result = strconv.ParseInt("ff", 16, 32)
  switch result
    case Ok(value) => value as Int
    case Err(_) => 0
  end
end
`
	parsed := parseSourceAsAst2(src)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("parseSourceAsAst2 failed: %v", parsed)
	}
	fileWithIDs := mustAssignFileExprIDsForTest(t, file.F0)
	path := "ffi-result.mygo"
	infoResult := typeinference2.InferPackageWithGoPackages(
		[]typeinference2.PkgDeclSource{{Path: path, Decls: fileWithIDs.Decls}},
		[]typeinference2.GoPackageEntry{{
			Alias: "strconv",
			Path:  "go:strconv",
			Funcs: []typeinference2.GoFuncSignature{{
				Name: "ParseInt", Params: []string{"string", "int", "int"}, Results: []string{"int64", "error"},
			}},
		}},
	)
	info, ok := infoResult.(Result__Ok[typeinference2.PackageInfo, string])
	if !ok {
		t.Fatalf("InferPackageWithGoPackages failed: %v", infoResult)
	}
	generated := GenerateFiles([]SourceFileInput{{Path: path, File: fileWithIDs}}, info.F0)
	result, ok := generated.(Result__Ok[map[string]string, string])
	if !ok {
		t.Fatalf("GenerateFiles failed: %v", generated)
	}
	code := result.F0[sourceToGenName(path)]
	if !strings.Contains(code, "result :=") || !strings.Contains(code, "Result__Ok[int64, error]") {
		t.Fatalf("generated switch lost FFI Result type:\n%s", code)
	}
}

func TestGenerateFilesKeepsInferredFFIResultTypeInNestedFunction(t *testing.T) {
	src := `package sample

import strconv "go:strconv"

enum Result[A, E]
  Ok(A)
  Err(E)
end

func parser() -> func() -> Int
  func() -> Int
    let result = strconv.ParseInt("ff", 16, 32)
    switch result
      case Ok(value) => value as Int
      case Err(_) => 0
    end
  end
end
`
	parsed := parseSourceAsAst2(src)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("parseSourceAsAst2 failed: %v", parsed)
	}
	fileWithIDs := mustAssignFileExprIDsForTest(t, file.F0)
	path := "nested-ffi-result.mygo"
	infoResult := typeinference2.InferPackageWithGoPackages(
		[]typeinference2.PkgDeclSource{{Path: path, Decls: fileWithIDs.Decls}},
		[]typeinference2.GoPackageEntry{{
			Alias: "strconv", Path: "go:strconv",
			Funcs: []typeinference2.GoFuncSignature{{
				Name: "ParseInt", Params: []string{"string", "int", "int"}, Results: []string{"int64", "error"},
			}},
		}},
	)
	info, ok := infoResult.(Result__Ok[typeinference2.PackageInfo, string])
	if !ok {
		t.Fatalf("InferPackageWithGoPackages failed: %v", infoResult)
	}
	generated := GenerateFiles([]SourceFileInput{{Path: path, File: fileWithIDs}}, info.F0)
	if _, ok := generated.(Result__Ok[map[string]string, string]); !ok {
		t.Fatalf("GenerateFiles failed: %v", generated)
	}
}

func TestGenerateFilesKeepsFFIResultTypeWithExternalPrelude(t *testing.T) {
	mainSource := `package sample

import strconv "go:strconv"

func parse() -> Int
  let result = strconv.ParseInt("ff", 16, 32)
  switch result
    case Ok(value) => value as Int
    case Err(_) => 0
  end
end
`
	externalSource := `package prelude

enum Result[A, E]
  Ok(A)
  Err(E)
end
`
	mainParsed := parseSourceAsAst2(mainSource)
	externalParsed := parseSourceAsAst2(externalSource)
	mainResult, mainOK := mainParsed.(Result__Ok[ast2.File, string])
	externalResult, externalOK := externalParsed.(Result__Ok[ast2.File, string])
	if !mainOK || !externalOK {
		t.Fatalf("parse failed: main=%v external=%v", mainParsed, externalParsed)
	}
	mainFile := mustAssignFileExprIDsForTest(t, mainResult.F0)
	externalFile := mustAssignFileExprIDsForTest(t, externalResult.F0)
	path := "external-ffi-result.mygo"
	infoResult := typeinference2.InferPackageWithExternal(
		[]typeinference2.PkgDeclSource{{Path: path, Decls: mainFile.Decls}},
		[]typeinference2.PkgDeclSource{{Path: "prelude.mygo", Decls: externalFile.Decls}},
		[]typeinference2.GoPackageEntry{{
			Alias: "strconv", Path: "go:strconv",
			Funcs: []typeinference2.GoFuncSignature{{
				Name: "ParseInt", Params: []string{"string", "int", "int"}, Results: []string{"int64", "error"},
			}},
		}},
		[]typeinference2.MyGoPackageInfo{},
	)
	info, ok := infoResult.(Result__Ok[typeinference2.PackageInfo, string])
	if !ok {
		t.Fatalf("InferPackageWithExternal failed: %v", infoResult)
	}
	generated := GenerateFiles([]SourceFileInput{{Path: path, File: mainFile}}, info.F0)
	if _, ok := generated.(Result__Ok[map[string]string, string]); !ok {
		t.Fatalf("GenerateFiles failed: %v", generated)
	}
}

func TestGenerateFilesImportsPreludeForBuiltinFFIFunction(t *testing.T) {
	mainSource := `package sample

func Run() -> ()
  Panic("failed")
end
`
	externalSource := `package prelude

func Panic(msg: String) -> ()
  go[()] { code: "panic($msg)"; in msg = msg }
end
`
	unusedSource := `package sample

func RunOther() -> ()
  ()
end
`
	parse := func(source string) ast2.File {
		parsed := parseSourceAsAst2(source)
		file, ok := parsed.(Result__Ok[ast2.File, string])
		if !ok {
			t.Fatalf("parseSourceAsAst2 failed: %v", parsed)
		}
		return mustAssignFileExprIDsForTest(t, file.F0)
	}
	mainFile := parse(mainSource)
	externalFile := parse(externalSource)
	unusedFile := parse(unusedSource)
	infoResult := typeinference2.InferPackageWithExternal(
		[]typeinference2.PkgDeclSource{{Path: "run.mygo", Decls: mainFile.Decls}, {Path: "other.mygo", Decls: unusedFile.Decls}},
		[]typeinference2.PkgDeclSource{{Path: "prelude.mygo", Decls: externalFile.Decls}},
		[]typeinference2.GoPackageEntry{},
		[]typeinference2.MyGoPackageInfo{},
	)
	info, ok := infoResult.(Result__Ok[typeinference2.PackageInfo, string])
	if !ok {
		t.Fatalf("InferPackageWithExternal failed: %v", infoResult)
	}
	generated := GenerateFiles([]SourceFileInput{
		{Path: "run.mygo", File: mainFile},
		{Path: "other.mygo", File: unusedFile},
	}, info.F0)
	files, ok := generated.(Result__Ok[map[string]string, string])
	if !ok {
		t.Fatalf("GenerateFiles failed: %v", generated)
	}
	callFile := files.F0["zz_run.gen.go"]
	if !strings.Contains(callFile, `. "github.com/mygo-lang/mygo/prelude"`) {
		t.Fatalf("prelude call did not import prelude:\n%s", callFile)
	}
	noCallFile := files.F0["zz_other.gen.go"]
	if strings.Contains(noCallFile, `"github.com/mygo-lang/mygo/prelude"`) {
		t.Fatalf("file without a prelude reference imported prelude:\n%s", noCallFile)
	}
}

func TestGenerateFilesPreservesFFIMultiReturnForTupleLet(t *testing.T) {
	src := `package sample

import strconv "go:strconv"

func parse() -> Int
  let (value, _) = strconv.ParseInt("ff", 16, 32)
  value as Int
end
`
	parsed := parseSourceAsAst2(src)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("parseSourceAsAst2 failed: %v", parsed)
	}
	fileWithIDs := mustAssignFileExprIDsForTest(t, file.F0)
	path := "ffi-tuple.mygo"
	infoResult := typeinference2.InferPackageWithGoPackages(
		[]typeinference2.PkgDeclSource{{Path: path, Decls: fileWithIDs.Decls}},
		[]typeinference2.GoPackageEntry{{
			Alias: "strconv", Path: "go:strconv",
			Funcs: []typeinference2.GoFuncSignature{{
				Name: "ParseInt", Params: []string{"string", "int", "int"}, Results: []string{"int64", "error"},
			}},
		}},
	)
	info, ok := infoResult.(Result__Ok[typeinference2.PackageInfo, string])
	if !ok {
		t.Fatalf("InferPackageWithGoPackages failed: %v", infoResult)
	}
	generated := GenerateFiles([]SourceFileInput{{Path: path, File: fileWithIDs}}, info.F0)
	result, ok := generated.(Result__Ok[map[string]string, string])
	if !ok {
		t.Fatalf("GenerateFiles failed: %v", generated)
	}
	code := result.F0[sourceToGenName(path)]
	if !strings.Contains(code, "value, _ := strconv.ParseInt") {
		t.Fatalf("tuple binding did not preserve Go multi-return call:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "ffi-tuple.gen.go", code, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceUsesDictionaryForFunctionUsingConstraint(t *testing.T) {
	src := `package sample

interface Show[A]
  func Show(value: A, prefix: String) -> String
end

func render[A](value: A) -> String using Show[A]
  value.Show()
end
`
	generated := GenerateSource(src)
	code, ok := generated.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", generated)
	}
	if !strings.Contains(code.F0, "return ShowFn(value)") {
		t.Fatalf("using-constrained method call did not use its dictionary:\n%s", code.F0)
	}
	if strings.Contains(code.F0, "value.Show()") {
		t.Fatalf("using-constrained method call remained a Go selector:\n%s", code.F0)
	}
}

func TestGenerateSourcePassesDictionaryToReceiverTypeclassCall(t *testing.T) {
	src := `package sample

interface Eq[A]
  func Equals(left: A, right: A) -> Bool
end

impl Int: Eq[Int]
  func Equals(left: Int, right: Int) -> Bool
    left == right
  end
end

interface Ord[A]
  func Compare(left: A, right: A) -> Int
end

impl String: Ord[String]
  func Compare(left: String, right: String) -> Int
    0
  end
end

interface IEnumerable[C[A], A]
  func Contains(c: C[A], item: A) -> Bool using Eq[A], Ord[A]
end

impl String: Eq[String]
  func Equals(left: String, right: String) -> Bool
    left == right
  end
end

struct Box[A]
  Value: A
end

impl[A] BoxEq[A]: Eq[Box[A]]
  func Equals(left: Box[A], right: Box[A]) -> Bool
    true
  end
end

impl[A] SetIEnumerable[A]: IEnumerable[Set[A], A]
  func Contains(c: Set[A], item: A) -> Bool using Eq[A], Ord[A]
    true
  end
end

interface IBoxEnumerable[C[A], A]
  func BoxContains(c: C[A], item: A) -> Bool using Eq[A]
end

impl[A] BoxSetEnumerable[A]: IBoxEnumerable[Set[A], A]
  func BoxContains(c: Set[A], item: A) -> Bool using Eq[A]
    true
  end
end

func containsHeader(headers: Set[String], key: String) -> Bool
  headers.Contains(key)
end

func containsBox(headers: Set[Box[String]], key: Box[String]) -> Bool
  headers.BoxContains(key)
end
`
	generated := GenerateSource(src)
	code, ok := generated.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", generated)
	}
	want := "Contains(headers, key, MygoIT2EqFN6StringGN6StringEM6Equals, MygoIT3OrdFN6StringGN6StringEM7Compare)"
	if !strings.Contains(code.F0, want) {
		t.Fatalf("receiver method call omitted or reordered its Eq/Ord dictionaries:\n%s", code.F0)
	}
	if !strings.Contains(code.F0, "BoxContains(headers, key, MygoIT2EqFN5BoxEq") {
		t.Fatalf("nested inferred receiver type did not select Eq[Box[String]]:\n%s", code.F0)
	}
	if !strings.Contains(code.F0, "EqualsFn func(A, A) bool") || !strings.Contains(code.F0, "CompareFn func(A, A) int") {
		t.Fatalf("ignored using dictionaries are missing from the generated helper signature:\n%s", code.F0)
	}
	fset := token.NewFileSet()
	generatedFile, err := parser.ParseFile(fset, "receiver-dictionaries.gen.go", code.F0, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code.F0)
	}
	if _, err := (&types.Config{Importer: importer.Default()}).Check("sample", fset, []*ast.File{generatedFile}, nil); err != nil {
		t.Fatalf("generated Go does not type-check: %v\n%s", err, code.F0)
	}
}

func TestPackageIndexCollectsLoweringMetadata(t *testing.T) {
	src := `package sample

struct User
  Name: String
end

enum Flag
  Ready
end

interface Pretty[A]
  func Show(value: A) -> String
end

impl IntPretty: Pretty[Int]
  func Show(value: Int) -> String
    "pretty"
  end
end

func render[A](value: A) -> String using Pretty[A]
  value.Show()
end
`
	parsed := parseSourceAsAst2(src)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("parseSourceAsAst2 failed: %v", parsed)
	}
	index := newPackageIndex(file.F0.Decls)
	if len(index.StructFields["User"]) != 1 {
		t.Fatalf("struct fields = %#v, want User.Name", index.StructFields)
	}
	if index.EnumValueConstructors["Flag.Ready"] == "" || index.EnumVariantOwners["Ready"] != "Flag" {
		t.Fatalf("enum metadata is incomplete: constructors=%#v owners=%#v", index.EnumValueConstructors, index.EnumVariantOwners)
	}
	if len(index.InterfaceMethods["Pretty"]) != 1 || len(index.InterfaceTypeParams["Pretty"]) != 1 {
		t.Fatalf("interface metadata is incomplete: methods=%#v params=%#v", index.InterfaceMethods, index.InterfaceTypeParams)
	}
	if !index.NamedImpls["IntPretty"] {
		t.Fatalf("named impls = %#v, want IntPretty", index.NamedImpls)
	}
	if got := index.CallDictionaries["render"]; len(got) != 1 || got[0] != "Show" {
		t.Fatalf("call dictionaries = %#v, want render: Show", index.CallDictionaries)
	}
	if got := index.CallRequirements["render"]; len(got) != 1 || got[0].Interface != "Pretty" || got[0].Method != "Show" {
		t.Fatalf("call requirements = %#v, want Pretty.Show", index.CallRequirements)
	}
	if len(index.PackageCandidates["Pretty.Show"]) != 1 || len(index.TypeclassCandidates) != 1 {
		t.Fatalf("impl candidates are incomplete: %#v", index.PackageCandidates)
	}
}

func TestGenerateSourceLowersZeroArgumentEnumVariantSelector(t *testing.T) {
	src := `package sample

enum Flag
  Ready
  Failed(String)
end

func ready() -> Flag
  Flag.Ready
end
`
	got := GenerateSource(src)
	result, ok := got.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "Flag__Ready__Ctor()") {
		t.Fatalf("generated Go does not call zero-argument enum ctor:\n%s", result.F0)
	}
}

func TestGenerateSourceLowersPackageLetAndVar(t *testing.T) {
	src := `package sample

func Read() -> Int
  fixed + changing
end

let fixed: Int = 40
var changing: Int = 2

func Increment() -> Int
  changing = changing + 1
  changing
end
`
	got := GenerateSource(src)
	ok, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	// Package-level let/var lower to a bare Go var plus an init() assignment so
	// the initializer runs after the package's dependencies are initialized.
	for _, want := range []string{"var fixed int", "fixed = 40", "var changing int", "changing = 2", "func Read() int", "changing = changing + 1"} {
		if !strings.Contains(ok.F0, want) {
			t.Fatalf("generated source missing %q:\n%s", want, ok.F0)
		}
	}
	if !strings.Contains(ok.F0, "func init()") {
		t.Fatalf("package-level let/var did not generate an init function:\n%s", ok.F0)
	}
	if strings.Contains(ok.F0, "var fixed int = 40") {
		t.Fatalf("package-level let was not lowered through init():\n%s", ok.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "generated.go", ok.F0, parser.AllErrors); err != nil {
		t.Fatalf("generated source is invalid Go: %v\n%s", err, ok.F0)
	}
}

func TestGenerateSourcePreservesAliasAndDefinedTypeSyntax(t *testing.T) {
	src := `package sample

type UserID = Int
type AccountID Int

func use(value: UserID) -> AccountID
  go[AccountID]{code: "return AccountID(value)"}
end
`
	got := GenerateSource(src)
	result, ok := got.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "type UserID = int") || !strings.Contains(result.F0, "type AccountID int") {
		t.Fatalf("generated Go lost type declaration distinction:\n%s", result.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "types.gen.go", result.F0, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestGenerateSourceExpandsGenericTypeAliasDuringInference(t *testing.T) {
	src := `package sample

type Values[A] = Slice[A]

func echo(values: Values[Int]) -> Slice[Int]
  values
end
`
	got := GenerateSource(src)
	result, ok := got.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "type Values[A any] = []A") || !strings.Contains(result.F0, "func echo(values Values[int]) []int") {
		t.Fatalf("generated Go lost generic alias semantics:\n%s", result.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "generic-types.gen.go", result.F0, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestGenerateSourceReturnsFunctionLiteralThroughTypeAlias(t *testing.T) {
	src := `package sample

type Parser[A] = func(Int) -> A

func pure[A](value: A) -> Parser[A]
  func(state: Int) -> A
    value
  end
end
`
	got := GenerateSource(src)
	result, ok := got.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "return func(state int) A") {
		t.Fatalf("generated Go did not return the function literal:\n%s", result.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "function-alias.gen.go", result.F0, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestGenerateSourceLowersRefBoundaryOperations(t *testing.T) {
	src := `package sample

func copy(value: Ref[Int]) -> Int
  value.value()
end

func address(value: Int) -> Ref[Int]
  Ref.new(value)
end

func number() -> Int
  1
end

func address_call() -> Ref[Int]
  Ref.new(number())
end
`
	got := GenerateSource(src)
	result, ok := got.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	for _, want := range []string{"func copy(value *int) int", "return *value", "func address(value int) *int", "return &value", "return &[]int{number()}[0]"} {
		if !strings.Contains(result.F0, want) {
			t.Fatalf("generated Go missing %q:\n%s", want, result.F0)
		}
	}
}

func TestGenerateSourceAllowsExhaustiveVariantSwitchWithoutWildcard(t *testing.T) {
	src := `package sample

enum Maybe[A]
  Have(A)
  Nothing
end

func unwrap(value: Maybe[Int]) -> Int
  switch value
    case Have(item) => item
    case Nothing => 0
  end
end
`
	got := GenerateSource(src)
	result, ok := got.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "exhaustive-switch.gen.go", result.F0, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestGenerateSourceKeepsUsedVariantBindingsInTailSwitches(t *testing.T) {
	src := `package sample

enum Maybe[A]
  Have(A)
  Nothing
end

func pair(value: Maybe[Int]) -> (Int, Int)
  switch value
    case Have(item) => (item, 0)
    case Nothing => (0, 0)
  end
end
`
	got := GenerateSource(src)
	result, ok := got.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if strings.Contains(result.F0, "_.F") {
		t.Fatalf("used pattern binding was lowered through the blank identifier:\n%s", result.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "tail-switch.gen.go", result.F0, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestGenerateSourceRejectsNonExhaustiveVariantSwitch(t *testing.T) {
	src := `package sample

enum Color
  Red
  Green
  Blue
end

func describe(value: Color) -> Int
  switch value
    case Red => 1
    case Green => 2
  end
end
`
	got := GenerateSource(src)
	err, ok := got.(Result__Err[string, string])
	if !ok || !strings.Contains(err.F0, "non-exhaustive switch: missing variant(s) Blue") {
		t.Fatalf("GenerateSource() = %v, want missing Blue error", got)
	}
}

func TestGenerateSourceChecksNestedTupleExhaustiveness(t *testing.T) {
	src := `package sample

enum Color
  Red
  Green
  Blue
end

func describe(value: (Color, Int)) -> Int
  switch value
    case (Red, _) => 1
    case (Green, _) => 2
  end
end
`
	got := GenerateSource(src)
	err, ok := got.(Result__Err[string, string])
	if !ok || !strings.Contains(err.F0, "non-exhaustive switch: missing variant(s) Blue") {
		t.Fatalf("GenerateSource() = %v, want nested missing Blue error", got)
	}
}

func TestGenerateSourceAtIncludesSourceLocation(t *testing.T) {
	got := GenerateSourceAt("broken.mygo", "package sample\n\nfunc")
	err, ok := got.(Result__Err[string, string])
	if !ok {
		t.Fatalf("GenerateSourceAt() = %T, want parse error", got)
	}
	if !strings.Contains(err.F0, "broken.mygo:3:5: parse error") {
		t.Fatalf("GenerateSourceAt() error = %q, want source name, line, and column", err.F0)
	}
}

func TestGenerateSourceIncludesExpressionPositionInInferenceErrors(t *testing.T) {
	got := GenerateSource(`package sample

func broken() -> Int
  missing
end
`)
	err, ok := got.(Result__Err[string, string])
	if !ok {
		t.Fatalf("GenerateSource unexpectedly succeeded: %v", got)
	}
	if !strings.Contains(err.F0, "<input>:4:3: unknown identifier missing") {
		t.Fatalf("error lacks expression position: %q", err.F0)
	}
}

func assertBootstrapsMyGOFile(t *testing.T, relativePath string) {
	t.Helper()
	_, thisFile, _, found := runtime.Caller(0)
	if !found {
		t.Fatal("cannot determine codegen2 test path")
	}
	sourcePath := filepath.Join(filepath.Dir(thisFile), relativePath)
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read %s: %v", sourcePath, err)
	}
	got := GenerateSourceAt(sourcePath, string(source))
	result, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource(%s) failed: %v", sourcePath, got)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "ast2.bootstrap.go", result.F0, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestParseSourceAsAst2KeepsInterfaceAndImplDecls(t *testing.T) {
	src := `package sample

interface Show[A]
  func Show(value: A) -> String
end

impl IntShow: Show[Int]
  func Show(value: Int) -> String
    "show"
  end
end
`

	got := parseSourceAsAst2(src)
	ok, yes := got.(Result__Ok[ast2.File, string])
	if !yes {
		t.Fatalf("parseSourceAsAst2 failed: %v", got)
	}
	if len(ok.F0.Decls) != 2 {
		t.Fatalf("decl count = %d, want 2", len(ok.F0.Decls))
	}
	if _, yes := ok.F0.Decls[0].(ast2.Decl__InterfaceDecl); !yes {
		t.Fatalf("decl[0] = %T, want ast2.Decl__InterfaceDecl", ok.F0.Decls[0])
	}
	impl, yes := ok.F0.Decls[1].(ast2.Decl__ImplDecl)
	if !yes {
		t.Fatalf("decl[1] = %T, want ast2.Decl__ImplDecl", ok.F0.Decls[1])
	}
	if len(impl.F3) != 1 {
		t.Fatalf("impl method count = %d, want 1", len(impl.F3))
	}
	bodyExpr, yes := impl.F3[0].Body.Kind.(ast2.ExprKind__BlockExpr)
	if !yes || len(bodyExpr.F0) != 1 {
		t.Fatalf("impl method body = %T, want single-item ast2.ExprBlockExpr", impl.F3[0].Body)
	}
	first, yes := bodyExpr.F0[0].(ast2.Stmt__ExprStmt)
	if !yes {
		t.Fatalf("impl method body item = %T, want ast2.Stmt__ExprStmt", bodyExpr.F0[0])
	}
	if _, yes := first.F0.Kind.(ast2.ExprKind__StringExpr); !yes {
		t.Fatalf("impl method body expr = %T, want ast2.ExprStringExpr", first.F0)
	}
}

func TestGenerateDynamicAdapterCallsImplPackageHelper(t *testing.T) {
	src := `package sample

interface Show[A]
  func Show(value: A) -> String
end

struct Widget
  Name: String
end

impl WidgetShow: Show[Widget]
  func Show(value: Widget, prefix: String) -> String
    prefix + value.Name
  end
end
`
	generated := GenerateSource(src)
	code, ok := generated.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", generated)
	}
	if !strings.Contains(code.F0, "func MygoDynamicAdapter") || !strings.Contains(code.F0, "payload any") {
		t.Fatalf("dynamic package-level adapter missing:\n%s", code.F0)
	}
	if strings.Contains(code.F0, "func (Widget)") {
		t.Fatalf("dynamic adapter unexpectedly uses a Go receiver:\n%s", code.F0)
	}
	if !strings.Contains(code.F0, "WidgetShow") {
		t.Fatalf("expected impl helper reference in generated adapter:\n%s", code.F0)
	}
	fset := token.NewFileSet()
	generatedFile, err := parser.ParseFile(fset, "dynamic-adapter.gen.go", code.F0, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code.F0)
	}
	if _, err := (&types.Config{Importer: importer.Default()}).Check("sample", fset, []*ast.File{generatedFile}, nil); err != nil {
		t.Fatalf("generated adapter does not type-check: %v\n%s", err, code.F0)
	}
}

func TestGenerateDynamicAdapterOmitsCrossOperandMethod(t *testing.T) {
	src := `package sample

interface Eq[A]
  func Equals(left: A, right: A) -> Bool
end

impl Int: Eq[Int]
  func Equals(left: Int, right: Int) -> Bool
    left == right
  end
end
`
	generated := GenerateSource(src)
	code, ok := generated.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", generated)
	}
	if strings.Contains(code.F0, "MygoDynamicAdapter") {
		t.Fatalf("cross-operand Equals unexpectedly received a dynamic adapter:\n%s", code.F0)
	}
}

func TestDynamicCallbackMethodIsMarkedUnsupportedAndHasNoAdapter(t *testing.T) {
	src := `package sample

interface Transform[A]
  func Transform(value: A, f: func(A) -> String) -> String
end

impl IntTransform: Transform[Int]
  func Transform(value: Int, f: func(Int) -> String) -> String
    f(value)
  end
end
`
	generated := GenerateSource(src)
	code, ok := generated.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", generated)
	}
	if strings.Contains(code.F0, "MygoDynamicAdapter") {
		t.Fatalf("callback method unexpectedly received a dynamic adapter:\n%s", code.F0)
	}
	parsed := parseSourceAsAst2(src)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("parseSourceAsAst2 failed: %v", parsed)
	}
	metaSource, err := dynamicMetadataSource("sample", "example.com/project/sample", map[string]string{"zz_transform.gen.go": code.F0}, []SourceFileInput{{Path: "transform.mygo", File: file.F0}}, nil)
	if err != nil {
		t.Fatalf("dynamicMetadataSource failed: %v", err)
	}
	value, err := metadata.ExtractSource(metadata.MetadataFileName, []byte(metaSource))
	if err != nil {
		t.Fatalf("metadata ExtractSource failed: %v", err)
	}
	if len(value.Implementations) != 1 || len(value.Implementations[0].Methods) != 1 {
		t.Fatalf("implementation metadata = %#v", value.Implementations)
	}
	method := value.Implementations[0].Methods[0]
	if method.Supported || !strings.Contains(method.UnsupportedReason, "callback or nested") {
		t.Fatalf("callback method metadata = %+v, want explicit unsupported reason", method)
	}
}

func TestGenerateDynamicAdapterPreservesGenericMethodParameters(t *testing.T) {
	src := `package sample

interface Label[A]
  func Label[B](value: A, label: B) -> String
end

impl IntLabel: Label[Int]
  func Label[B](value: Int, label: B) -> String
    "label"
  end
end
`
	generated := GenerateSource(src)
	code, ok := generated.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", generated)
	}
	if !strings.Contains(code.F0, "func MygoDynamicAdapter") || !strings.Contains(code.F0, "[B any]") {
		t.Fatalf("generic dynamic adapter did not retain its method type parameter:\n%s", code.F0)
	}
	fset := token.NewFileSet()
	generatedFile, err := parser.ParseFile(fset, "generic-dynamic-adapter.gen.go", code.F0, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code.F0)
	}
	if _, err := (&types.Config{Importer: importer.Default()}).Check("sample", fset, []*ast.File{generatedFile}, nil); err != nil {
		t.Fatalf("generic adapter does not type-check: %v\n%s", err, code.F0)
	}
}

func TestDynamicMetadataDescribesExportedAPIAdaptersAndDigests(t *testing.T) {
	src := `package sample

interface Show[A]
  func Show(value: A, prefix: String) -> String
end

struct Widget
  Name: String
end

impl WidgetShow: Show[Widget]
  func Show(value: Widget, prefix: String) -> String
    prefix + value.Name
  end
end
`
	parsed := parseSourceAsAst2(src)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("parseSourceAsAst2 failed: %v", parsed)
	}
	typedFile := mustAssignFileExprIDsForTest(t, file.F0)
	inputPath := "widget.mygo"
	inferred := typeinference2.InferPackageWithExternal(
		[]typeinference2.PkgDeclSource{{Path: inputPath, Decls: typedFile.Decls}},
		[]typeinference2.PkgDeclSource{},
		[]typeinference2.GoPackageEntry{},
		[]typeinference2.MyGoPackageInfo{},
	)
	info, ok := inferred.(Result__Ok[typeinference2.PackageInfo, string])
	if !ok {
		t.Fatalf("InferPackageWithExternal failed: %v", inferred)
	}
	input := SourceFileInputWithPackageImportPath(NewSourceFileInput(inputPath, typedFile), "example.com/project/sample")
	generatedFiles := GenerateFiles([]SourceFileInput{input}, info.F0)
	generatedResult, ok := generatedFiles.(Result__Ok[map[string]string, string])
	if !ok {
		t.Fatalf("GenerateFiles failed: %v", generatedFiles)
	}
	metadataSource, exists := generatedResult.F0[metadata.MetadataFileName]
	if !exists {
		t.Fatalf("generated files lack %s: %#v", metadata.MetadataFileName, generatedResult.F0)
	}
	parsedMetadata, err := metadata.ExtractSource(metadata.MetadataFileName, []byte(metadataSource))
	if err != nil {
		t.Fatalf("generated metadata source is invalid: %v", err)
	}
	if len(parsedMetadata.Implementations) != 1 || len(parsedMetadata.Implementations[0].Methods) != 1 {
		t.Fatalf("generated metadata omitted implementation adapters: %#v", parsedMetadata.Implementations)
	}
	adapter := parsedMetadata.Implementations[0].Methods[0]
	if !adapter.Supported || adapter.Symbol == "" || !strings.Contains(generatedResult.F0[sourceToGenName(inputPath)], "func "+adapter.Symbol) {
		t.Fatalf("metadata adapter does not describe a generated package helper: %+v", adapter)
	}
	outputDir := t.TempDir()
	for name, source := range generatedResult.F0 {
		if err := os.WriteFile(filepath.Join(outputDir, name), []byte(source), 0o644); err != nil {
			t.Fatalf("write generated output %s: %v", name, err)
		}
	}
	if _, err := metadata.ReadDir(outputDir); err != nil {
		t.Fatalf("generated metadata digest verification failed: %v", err)
	}
	fset := token.NewFileSet()
	goFiles := make([]*ast.File, 0, len(generatedResult.F0))
	for name, source := range generatedResult.F0 {
		parsedGo, err := parser.ParseFile(fset, name, source, parser.AllErrors)
		if err != nil {
			t.Fatalf("generated Go file %s did not parse: %v", name, err)
		}
		goFiles = append(goFiles, parsedGo)
	}
	if _, err := (&types.Config{Importer: importer.Default()}).Check("example.com/project/sample", fset, goFiles, nil); err != nil {
		t.Fatalf("generated package with metadata does not type-check: %v", err)
	}
	generated := GenerateSource(src)
	code, ok := generated.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", generated)
	}
	source, err := dynamicMetadataSource("sample", "example.com/project/sample", map[string]string{"zz_widget.gen.go": code.F0}, []SourceFileInput{{Path: "widget.mygo", File: file.F0}}, nil)
	if err != nil {
		t.Fatalf("dynamicMetadataSource failed: %v", err)
	}
	value, err := metadata.ExtractSource(metadata.MetadataFileName, []byte(source))
	if err != nil {
		t.Fatalf("metadata source could not be read: %v", err)
	}
	if len(value.Exports.Interfaces) != 1 || value.Exports.Interfaces[0].Name != "Show" {
		t.Fatalf("exported interfaces = %#v, want Show", value.Exports.Interfaces)
	}
	if len(value.Exports.Types) != 2 || value.Exports.Types[0].Name != "MygoDynamicShow" || value.Exports.Types[1].Name != "Widget" {
		t.Fatalf("exported types = %#v, want MygoDynamicShow and Widget", value.Exports.Types)
	}
	if len(value.Implementations) != 1 || len(value.Implementations[0].Methods) != 1 {
		t.Fatalf("implementation metadata = %#v, want one implementation method", value.Implementations)
	}
	directAdapter := value.Implementations[0].Methods[0]
	if !directAdapter.Supported || directAdapter.Symbol == "" || !strings.Contains(code.F0, "func "+directAdapter.Symbol) {
		t.Fatalf("metadata adapter = %+v, but generated function is missing", directAdapter)
	}
	if len(value.GeneratedSource) != 1 || value.GeneratedSource[0].Path != "zz_widget.gen.go" {
		t.Fatalf("generated source digests = %#v", value.GeneratedSource)
	}
}

func TestGenerateSourceEncodesHigherKindedParameters(t *testing.T) {
	src := `package prelude

interface Enumerable[C[A], A]
  func First(value: C[A]) -> A
end
`
	got := GenerateSource(src)
	ok, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(ok.F0, "type HKT[F any, A any] interface{}") {
		t.Fatalf("HKT application was not preserved:\n%s", ok.F0)
	}
	if strings.Contains(ok.F0, "type Enumerable interface") {
		t.Fatalf("typeclass interface must not be emitted as a Go interface:\n%s", ok.F0)
	}
	for _, want := range []string{
		"type MygoDynamicEnumerable struct",
		"func NewMygoDynamicEnumerable(payload any, implementation string) MygoDynamicEnumerable",
		"func MygoDynamicEnumerablePayload(value MygoDynamicEnumerable) any",
		"func MygoDynamicEnumerableImplementation(value MygoDynamicEnumerable) string",
	} {
		if !strings.Contains(ok.F0, want) {
			t.Fatalf("generated source missing dynamic wrapper declaration %q:\n%s", want, ok.F0)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "hkt.gen.go", ok.F0, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, ok.F0)
	}
}

func TestGenerateFilesUsesExternalPreludeHKTDeclarations(t *testing.T) {
	src := `package sample

interface Enumerable[C[A], A]
  func First(value: C[A]) -> A
end

func Default(value: Option[Int]) -> Option[Int]
  value
end
`
	preludeSource := `package prelude

enum Option[A]
  Some(A)
  None
end
`
	parsed := parseSourceAsAst2(src)
	preludeParsed := parseSourceAsAst2(preludeSource)
	fileResult, fileOK := parsed.(Result__Ok[ast2.File, string])
	preludeResult, preludeOK := preludeParsed.(Result__Ok[ast2.File, string])
	if !fileOK || !preludeOK {
		t.Fatalf("parse failed: source=%v prelude=%v", parsed, preludeParsed)
	}
	file := mustAssignFileExprIDsForTest(t, fileResult.F0)
	preludeFile := mustAssignFileExprIDsForTest(t, preludeResult.F0)
	path := "sample.mygo"
	inferred := typeinference2.InferPackageWithExternal(
		[]typeinference2.PkgDeclSource{{Path: path, Decls: file.Decls}},
		[]typeinference2.PkgDeclSource{{Path: "prelude.mygo", Decls: preludeFile.Decls}},
		[]typeinference2.GoPackageEntry{},
		[]typeinference2.MyGoPackageInfo{},
	)
	info, infoOK := inferred.(Result__Ok[typeinference2.PackageInfo, string])
	if !infoOK {
		t.Fatalf("InferPackageWithExternal failed: %v", inferred)
	}
	generated := GenerateFiles([]SourceFileInput{{Path: path, File: file}}, info.F0)
	generatedResult, generatedOK := generated.(Result__Ok[map[string]string, string])
	if !generatedOK {
		t.Fatalf("GenerateFiles failed: %v", generated)
	}
	code := generatedResult.F0[sourceToGenName(path)]
	if strings.Contains(code, "type HKTType interface{}") {
		t.Fatalf("non-prelude package redeclared prelude HKT helpers:\n%s", code)
	}
	if !strings.Contains(code, `. "github.com/mygo-lang/mygo/prelude"`) {
		t.Fatalf("non-prelude package did not dot-import prelude:\n%s", code)
	}
}

func TestGenerateSourceLowersEmptySliceStructFieldWithGenericElement(t *testing.T) {
	src := `package sample

struct Reply[A]
  Value: A
end

func Empty[A]() -> Reply[Slice[A]]
  Reply[Slice[A]] { Value: [] }
end
`
	got := GenerateSource(src)
	ok, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(ok.F0, "Reply[[]A]{Value: []A{}}") {
		t.Fatalf("empty generic slice field was not lowered with its field type:\n%s", ok.F0)
	}
}

func TestGenerateSourceUsesCurrentImplMangling(t *testing.T) {
	src := `package sample

interface Pretty[A]
  func Show(value: A) -> String
end

impl IntPretty: Pretty[Int]
  func Show(value: Int) -> String
    "pretty"
  end
end
`

	got := GenerateSource(src)
	ok, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "func MygoIT6PrettyFN9IntPrettyGN3IntEM4Show(value int) string") {
		t.Fatalf("generated impl helper does not use current mangling:\n%s", code)
	}
	if strings.Contains(code, "impl_pretty") || strings.Contains(code, "Show_impl") {
		t.Fatalf("generated impl helper still uses legacy temporary naming:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateFilesUsesGoTestOutputName(t *testing.T) {
	file := ast2.File{PackageName: "sample", Decls: []ast2.Decl{}}
	got := GenerateFiles([]SourceFileInput{{Path: "math_test.mygo", File: file}}, typeinference2.PackageInfo{})
	ok, yes := got.(Result__Ok[map[string]string, string])
	if !yes {
		t.Fatalf("GenerateFiles failed: %v", got)
	}
	if _, exists := ok.F0["zz_math.gen_test.go"]; !exists {
		t.Fatalf("generated files = %#v, want zz_math.gen_test.go", ok.F0)
	}
	if _, exists := ok.F0["zz_math_test.gen.go"]; exists {
		t.Fatalf("generated legacy test filename: %#v", ok.F0)
	}
}

func TestSourceFileInputCarriesCanonicalPackageImportPath(t *testing.T) {
	input := NewSourceFileInput("widget.mygo", ast2.File{PackageName: "widget"})
	input = SourceFileInputWithPackageImportPath(input, "example.com/project/widget")
	input = SourceFileInputWithAutomaticImport(input, ".", "github.com/mygo-lang/mygo/prelude")
	if input.PackageImportPath != "example.com/project/widget" {
		t.Fatalf("PackageImportPath = %q, want canonical module import path", input.PackageImportPath)
	}
}

func TestGenerateSourceLowersExpressionIfToTemp(t *testing.T) {
	src := `package sample

func pick(flag: Bool) -> Int
  let value: Int = if flag then
    1
  else
    2
  end
  value
end
`

	got := GenerateSource(src)
	ok, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if strings.Contains(code, "func() any") {
		t.Fatalf("generated expression if still uses IIFE:\n%s", code)
	}
	if !strings.Contains(code, "var __mygo_expr_0 int") {
		t.Fatalf("generated expression if does not allocate typed temp:\n%s", code)
	}
	if !strings.Contains(code, "var value int = __mygo_expr_0") {
		t.Fatalf("typed let was not emitted as a Go var declaration:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceInfersStructFieldAccess(t *testing.T) {
	src := `package sample

struct Point
  x: Int
  name: String
end

func getX(p: Point) -> Int
  p.x
end
`

	got := GenerateSource(src)
	ok, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "type Point struct") || !strings.Contains(code, "x    int") || !strings.Contains(code, "name string") {
		t.Fatalf("struct fields were not generated as expected:\n%s", code)
	}
	if !strings.Contains(code, "return p.x") {
		t.Fatalf("field access was not lowered as expected:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceVarDeclaration(t *testing.T) {
	src := `package sample

func foo() -> Int
  var x: Int = 42
  x
end
`

	got := GenerateSource(src)
	ok, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "var x int = 42") {
		t.Fatalf("generated var declaration is missing:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceLetRecBindingGroup(t *testing.T) {
	src := `package sample

func parity(n: Int) -> Bool
  letrec
    even: func(Int) -> Bool = func(value: Int) -> Bool
      if value == 0 => true else odd(value - 1)
    end
    odd: func(Int) -> Bool = func(value: Int) -> Bool
      if value == 0 => false else even(value - 1)
    end
  end
  even(n)
end
`

	got := GenerateSource(src)
	ok, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	for _, want := range []string{
		"var even func(int) bool",
		"var odd func(int) bool",
		"even = func(value int) bool",
		"odd = func(",
		"return odd(value - 1)",
		"return even(value_1 - 1)",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("generated letrec is missing %q:\n%s", want, code)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceWhileLoop(t *testing.T) {
	src := `package sample

func count(n: Int) -> Int
  var i: Int = 0
  while i > 10
    i = i + 1
  end
  i
end
`

	got := GenerateSource(src)
	ok, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "for") {
		t.Fatalf("generated while loop is missing 'for':\n%s", code)
	}
	if !strings.Contains(code, ">") {
		t.Fatalf("generated while loop is missing condition operator '>':\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceInlineGoExpression(t *testing.T) {
	src := `package sample

import strconv "go:strconv"

func toString(n: Int) -> String
  go[String]{code: "return strconv.Itoa(n)"}
end
`

	got := GenerateSource(src)
	ok, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "return strconv.Itoa(n)") {
		t.Fatalf("generated inline go is missing body:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceInlineGoOperands(t *testing.T) {
	src := `package sample

func toString(n: Int) -> String
  go[String]{code: "{T}({v})" in v = n type T = String}
end
`

	got := GenerateSource(src)
	ok, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(ok.F0, "return string(n)") {
		t.Fatalf("inline operands were not substituted in generated AST:\n%s", ok.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", ok.F0, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, ok.F0)
	}
}

func TestGenerateSourceReturnStatement(t *testing.T) {
	src := `package sample

func earlyReturn(flag: Bool) -> Int
  if flag then
    return 1
  else
    return 2
  end
end
`

	got := GenerateSource(src)
	ok, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "return 1") {
		t.Fatalf("generated return statement is missing 'return 1':\n%s", code)
	}
	if !strings.Contains(code, "return 2") {
		t.Fatalf("generated return statement is missing 'return 2':\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceBareReturn(t *testing.T) {
	src := `package sample

func bareReturn(flag: Bool) -> ()
  if flag then
    return
  end
  ()
end
`

	got := GenerateSource(src)
	ok, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "return") {
		t.Fatalf("generated bare return is missing 'return':\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceLowersLiteralSwitch(t *testing.T) {
	src := `package sample

func classify(value: Int) -> String
  switch value
    case 0 => "zero"
    case _ => "other"
  end
end
`

	got := GenerateSource(src)
	ok, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "if value == 0") {
		t.Fatalf("generated switch is missing literal guard:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceLowersVariantSwitchWithBinding(t *testing.T) {
	src := `package sample

enum Maybe
  Have(Int)
  Nothing
end

func unwrap(value: Maybe) -> Int
  switch value
    case Have(item) => item
    case Nothing => 0
    case _ => 0
  end
end
`

	got := GenerateSource(src)
	ok, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	code := ok.F0
	if !strings.Contains(code, "value.(Maybe__Have)") || !strings.Contains(code, ".F0") {
		t.Fatalf("generated variant switch is missing assertion or field binding:\n%s", code)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", code, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, code)
	}
}

func TestGenerateSourceInfersResultVariantArgumentsInSwitchBranches(t *testing.T) {
	src := `package sample

enum Outcome[A, E]
  Success(A)
  Failure(E)
end

func parse(value: String) -> Outcome[Int, String]
  switch value
    case "ok" => Success(1)
    case _ => Failure("invalid")
  end
end
`

	got := GenerateSource(src)
	result, ok := got.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	for _, want := range []string{"Success[int, string](1)", "Failure[int, string](\"invalid\")"} {
		if !strings.Contains(result.F0, want) {
			t.Fatalf("generated Go is missing %q:\n%s", want, result.F0)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "result-switch.gen.go", result.F0, parser.AllErrors); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestGenerateSourceSupportsForwardFunctionReferences(t *testing.T) {
	src := `package sample

func first() -> Int
  second()
end

func second() -> Int
  42
end
`
	got := GenerateSource(src)
	result, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", result.F0, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestGenerateSourceLowersEscapedRune(t *testing.T) {
	src := `package sample

func newline() -> Rune
  '\n'
end
`
	got := GenerateSource(src)
	result, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "return '\\n'") {
		t.Fatalf("generated rune literal is missing:\n%s", result.F0)
	}
}

func TestGenerateSourceLowersTypedSliceLiteral(t *testing.T) {
	src := `package sample

func values() -> Slice[Int]
  [1, 2,] as Slice[Int]
end
`
	got := GenerateSource(src)
	result, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "return []int{1, 2}") {
		t.Fatalf("generated typed slice literal is missing:\n%s", result.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", result.F0, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestGenerateSourcePreservesGenericStructLiteralArguments(t *testing.T) {
	src := `package sample

struct Box[A]
  value: A
end

func makeBox() -> Box[Int]
  Box[Int] { value: 42 }
end
`
	got := GenerateSource(src)
	result, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "return Box[int]{value: 42}") {
		t.Fatalf("generated generic struct literal lost type arguments:\n%s", result.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", result.F0, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestGenerateSourceInfersOmittedGenericStructLiteralArguments(t *testing.T) {
	src := `package sample

struct Pair[A]
  first: A
  second: A
end

func makePair() -> Pair[Int]
  Pair { first: 1, second: 2 }
end
`
	got := GenerateSource(src)
	result, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "return Pair[int]{first: 1, second: 2}") {
		t.Fatalf("generated inferred generic struct literal lost type arguments:\n%s", result.F0)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sample.gen.go", result.F0, 0); err != nil {
		t.Fatalf("generated Go is invalid: %v\n%s", err, result.F0)
	}
}

func TestGenerateSourceInfersGenericStructLiteralArgumentsFromFields(t *testing.T) {
	src := `package sample

struct Pair[A]
  first: A
  second: A
end

func demo() -> ()
  let pair = Pair { first: 1, second: 2 }
  ()
end
`
	got := GenerateSource(src)
	result, yes := got.(Result__Ok[string, string])
	if !yes {
		t.Fatalf("GenerateSource failed: %v", got)
	}
	if !strings.Contains(result.F0, "pair := Pair[int]{first: 1, second: 2}") {
		t.Fatalf("field values did not infer Pair[Int]:\n%s", result.F0)
	}
}

// Both source spellings of the Go builtin error interface must lower to Go's
// `error` in declaration signatures: parameters, results, and tuple result
// components.
func TestGenerateFilesLowersErrorBuiltinAliasInSignatures(t *testing.T) {
	src := `package sample

func produceError(err: error) -> error
  err
end

func reportFailure(err: Error) -> Error
  err
end

func reportLower(err: error) -> error
  err
end

func reportPair(err: Error) -> (Error, Int)
  (err, 1)
end
`
	parsed := parseSourceAsAst2(src)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("parseSourceAsAst2 failed: %v", parsed)
	}
	fileWithIDs := mustAssignFileExprIDsForTest(t, file.F0)
	path := "error-alias.mygo"
	infoResult := typeinference2.InferPackageWithGoPackages(
		[]typeinference2.PkgDeclSource{{Path: path, Decls: fileWithIDs.Decls}},
		[]typeinference2.GoPackageEntry{},
	)
	info, ok := infoResult.(Result__Ok[typeinference2.PackageInfo, string])
	if !ok {
		t.Fatalf("InferPackageWithGoPackages failed: %v", infoResult)
	}
	generated := GenerateFiles([]SourceFileInput{{Path: path, File: fileWithIDs}}, info.F0)
	result, ok := generated.(Result__Ok[map[string]string, string])
	if !ok {
		t.Fatalf("GenerateFiles failed: %v", generated)
	}
	code := result.F0[sourceToGenName(path)]
	for _, want := range []string{
		"func produceError(err error) error",
		"func reportFailure(err error) error",
		"func reportLower(err error) error",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("generated code missing %q:\n%s", want, code)
		}
	}
	// The tuple result must lower its Error component to `error`, not emit a
	// bare `Error` identifier.
	if !strings.Contains(code, "reportPair(err error) (error, int)") {
		t.Fatalf("generated tuple signature did not lower Error to error:\n%s", code)
	}
}

// A generic declaration whose type parameter is named `Error` must keep the
// declared parameter; the builtin alias lowering must not rewrite it to Go's
// `error` builtin.
func TestGenerateFilesPreservesTypeParameterNamedError(t *testing.T) {
	src := `package sample

func Identity[Error](value: Error) -> Error
  value
end
`
	parsed := parseSourceAsAst2(src)
	file, ok := parsed.(Result__Ok[ast2.File, string])
	if !ok {
		t.Fatalf("parseSourceAsAst2 failed: %v", parsed)
	}
	fileWithIDs := mustAssignFileExprIDsForTest(t, file.F0)
	path := "error-typeparam.mygo"
	infoResult := typeinference2.InferPackageWithGoPackages(
		[]typeinference2.PkgDeclSource{{Path: path, Decls: fileWithIDs.Decls}},
		[]typeinference2.GoPackageEntry{},
	)
	info, ok := infoResult.(Result__Ok[typeinference2.PackageInfo, string])
	if !ok {
		t.Fatalf("InferPackageWithGoPackages failed: %v", infoResult)
	}
	generated := GenerateFiles([]SourceFileInput{{Path: path, File: fileWithIDs}}, info.F0)
	result, ok := generated.(Result__Ok[map[string]string, string])
	if !ok {
		t.Fatalf("GenerateFiles failed: %v", generated)
	}
	code := result.F0[sourceToGenName(path)]
	if !strings.Contains(code, "func Identity[Error any](value Error) Error") {
		t.Fatalf("generated signature replaced the Error type parameter with the error builtin:\n%s", code)
	}
}

// TestGenerateSourceHoistsTupleSwitchSubjectOnce checks that a switch whose
// subject is a tuple literal evaluates that subject once into a temp and lets
// every branch reference it, instead of re-evaluating (or losing) branches that
// assert on a tuple field.
func TestGenerateSourceHoistsTupleSwitchSubjectOnce(t *testing.T) {
	src := `package sample

enum AgentEvent
  Timeout { Where: Int }
  Started
end

func classify(state: Int, event: AgentEvent) -> Int
  switch (state, event)
    case (current, Timeout { Where }) => current + Where
    case (_, Started) => -1
    case (_, _) => -2
  end
end
`
	result := GenerateSource(src)
	code, ok := result.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", result)
	}
	if !strings.Contains(code.F0, "__mygo_expr_0 := struct {") {
		t.Fatalf("tuple switch subject was not hoisted into a single temp:\n%s", code.F0)
	}
	if strings.Count(code.F0, "F0: state, F1: event") != 1 {
		t.Fatalf("tuple switch subject was re-evaluated per branch (want exactly one literal):\n%s", code.F0)
	}
	if !strings.Contains(code.F0, "__mygo_expr_0.F1.(AgentEvent__Timeout)") {
		t.Fatalf("variant branch did not assert on the hoisted tuple field:\n%s", code.F0)
	}
	if !strings.Contains(code.F0, "__mygo_expr_0.F0 + __mygo_match") {
		t.Fatalf("the first variant branch was lost from the chain:\n%s", code.F0)
	}
}

// generatedGoUnusedBindings reports every local name a generated function
// defines with `:=` or `var` but never reads.  Go's own "declared and not
// used" rule is scope-precise; the generated code gives every lowering
// temporary a globally unique `__mygo_` name, so counting uses per function
// body is a faithful-enough approximation and catches the same defect.
func generatedGoUnusedBindings(t *testing.T, code string) map[string][]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "gen.go", code, 0)
	if err != nil {
		t.Fatalf("generated Go does not parse: %v\n%s", err, code)
	}
	unused := map[string][]string{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		defs := map[string]bool{}
		uses := map[string]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				if node.Tok == token.DEFINE {
					for _, lhs := range node.Lhs {
						if id, ok := lhs.(*ast.Ident); ok {
							defs[id.Name] = true
						}
					}
					// The defining occurrence is not a use.
					for _, rhs := range node.Rhs {
						ast.Inspect(rhs, countUses(uses))
					}
					return false
				}
			case *ast.ValueSpec:
				for _, id := range node.Names {
					defs[id.Name] = true
				}
				for _, v := range node.Values {
					ast.Inspect(v, countUses(uses))
				}
				return false
			case *ast.Ident:
				uses[node.Name] = true
			}
			return true
		})
		for name := range defs {
			if name == "_" || name == "ok" || uses[name] {
				continue
			}
			unused[fn.Name.Name] = append(unused[fn.Name.Name], name)
		}
	}
	return unused
}

func countUses(uses map[string]bool) func(ast.Node) bool {
	return func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			uses[id.Name] = true
		}
		return true
	}
}

func assertNoUnusedBindings(t *testing.T, code string) {
	t.Helper()
	if unused := generatedGoUnusedBindings(t, code); len(unused) > 0 {
		t.Fatalf("generated Go declares bindings it never reads (Go would report "+
			"\"declared and not used\"): %v\n%s", unused, code)
	}
}

// TestGenerateSourceNestedVariantSwitchBranchesDoNotLeakBinding covers the
// reported defect: `case Ok(Some(x))` next to `case Ok(None)`.  The outer
// assertion needs a real name because the inner assertion reads `<name>.F0`,
// but that name must not stay in scope inside the `None` branch, which does
// its own independent assertion.
func TestGenerateSourceNestedVariantSwitchBranchesDoNotLeakBinding(t *testing.T) {
	src := `package sample

func readExecution() -> Result[Option[Int], String]
  Ok(Some(1))
end

func read() -> Result[Int, String]
  Ok(2)
end

func toStepExpr() -> Result[Bool, String]
  switch read()
    case Err(e) => Err(e)
    case Ok(raw) =>
      switch readExecution()
        case Err(e) => Err(e)
        case Ok(Some(x)) => Ok(x > raw)
        case Ok(None) => Ok(false)
      end
  end
end
`
	result := GenerateSource(src)
	code, ok := result.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", result)
	}
	t.Logf("Generated code:\n%s", code.F0)
	assertNoUnusedBindings(t, code.F0)

	// The `None` branch must still assert on its own, and the scrutinee must
	// still be evaluated exactly once.
	// The call also appears in readExecution's own `return` statement, so count
	// only the assignment that hoists the scrutinee into a temporary.
	if strings.Count(code.F0, "__mygo_expr_3 := readExecution()") != 1 {
		t.Fatalf("switch subject was re-evaluated per branch:\n%s", code.F0)
	}
}

// TestGenerateSourceNestedVariantSwitchStatementFormHasNoUnusedBinding is the
// reported defect in statement position: the inner `switch` is a statement,
// so it lowers through the statement-form branch translators rather than the
// expression ones, and a sibling `Ok(None)` case is left holding a binding it
// never reads.
func TestGenerateSourceNestedVariantSwitchStatementFormHasNoUnusedBinding(t *testing.T) {
	src := `package sample

func readExecution() -> Result[Option[Int], String]
  Ok(Some(1))
end

func read() -> Result[Int, String]
  Ok(2)
end

func toStepStmt() -> Result[Bool, String]
  switch read()
    case Err(e) =>
      return Err(e)
    case Ok(raw) =>
      let r = readExecution()
      switch r
        case Err(e) =>
          return Err(e)
        case Ok(Some(x)) =>
          return Ok(x > raw)
        case Ok(None) =>
          return Ok(false)
      end
  end
end
`
	result := GenerateSource(src)
	code, ok := result.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", result)
	}
	t.Logf("Generated code:\n%s", code.F0)
	assertNoUnusedBindings(t, code.F0)
}

func TestDynamicGoTypeRendersReservedChannelTypes(t *testing.T) {
	imports := newDynamicImportTable("app", "", nil, nil)
	cases := []struct{ name, want string }{
		{"Chan", "chan string"},
		{"SendChan", "chan<- string"},
		{"RecvChan", "<-chan string"},
	}
	for _, tc := range cases {
		got, err := dynamicGoType(metadata.Type{Kind: "named", Name: tc.name, Package: "example.com/concurrency", Arguments: []metadata.Type{{Kind: "builtin", Name: "string"}}}, imports)
		if err != nil {
			t.Fatalf("dynamicGoType(%s) failed: %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("dynamicGoType(%s) = %q, want %q", tc.name, got, tc.want)
		}
		if strings.Contains(got, "concurrency") || strings.Contains(got, tc.name) {
			t.Fatalf("dynamicGoType(%s) = %q still names the channel type", tc.name, got)
		}
	}
}

func TestDynamicGoTypeExprToMygoReconstructsChannelDirections(t *testing.T) {
	cases := []struct{ src, name string }{
		{"chan string", "Chan"},
		{"chan<- string", "SendChan"},
		{"<-chan string", "RecvChan"},
	}
	for _, tc := range cases {
		expr, err := parser.ParseExpr(tc.src)
		if err != nil {
			t.Fatal(err)
		}
		typ, err := dynamicGoTypeExprToMygo(expr)
		if err != nil {
			t.Fatalf("dynamicGoTypeExprToMygo(%s) failed: %v", tc.src, err)
		}
		named, ok := typ.(ast2.TypeExpr__NamedType)
		if !ok {
			t.Fatalf("dynamicGoTypeExprToMygo(%s) = %T, want named type", tc.src, typ)
		}
		if named.F0 != tc.name || len(named.F1) != 1 {
			t.Fatalf("dynamicGoTypeExprToMygo(%s) = %s with %d args, want %s[...]", tc.src, named.F0, len(named.F1), tc.name)
		}
		element, ok := named.F1[0].(ast2.TypeExpr__NamedType)
		if !ok || element.F0 != "String" {
			t.Fatalf("dynamicGoTypeExprToMygo(%s) element = %#v, want String", tc.src, named.F1[0])
		}
	}
}

func TestDynamicDispatchSkipsUnusableUnreferencedImplementation(t *testing.T) {
	contracts := metadata.PackageMetadata{
		Format: metadata.FormatName, SchemaVersion: metadata.SchemaVersion, CompilerABI: metadata.CompilerABI, AdapterABI: metadata.AdapterABI,
		Package: metadata.PackageIdentity{Name: "contracts", ImportPath: "example.com/contracts"},
		Exports: metadata.Exports{Interfaces: []metadata.InterfaceExport{{Name: "IWritableChan", TypeParams: []metadata.TypeParam{{Name: "A", Scope: "interface:IWritableChan", Index: 0}}, Methods: []metadata.Method{{Name: "Send"}}}}},
	}
	producer := metadata.PackageMetadata{
		Format: metadata.FormatName, SchemaVersion: metadata.SchemaVersion, CompilerABI: metadata.CompilerABI, AdapterABI: metadata.AdapterABI,
		Package: metadata.PackageIdentity{Name: "producer", ImportPath: "example.com/producer"},
		Implementations: []metadata.Implementation{{
			ID:        "sha256:chan-unsupported",
			Interface: metadata.TypeclassID{Package: "example.com/contracts", Name: "IWritableChan"},
			Target:    metadata.Type{Kind: "named", Name: "Chan", Package: "example.com/producer", Arguments: []metadata.Type{{Kind: "builtin", Name: "int"}}},
			Methods:   []metadata.Adapter{{Method: "Send", Supported: false, UnsupportedReason: "adapter cannot capture implementation target type parameters"}},
		}},
	}
	contractsJSON, _ := json.Marshal(contracts)
	producerJSON, _ := json.Marshal(producer)
	deps := []typeinference2.MyGoPackageInfo{
		{Alias: "contracts", Path: "example.com/contracts", DynamicMetadataJSON: string(contractsJSON)},
		{Alias: "producer", Path: "example.com/producer", DynamicMetadataJSON: string(producerJSON)},
	}
	source, err := dynamicDispatchSource("app", nil, deps)
	if err != nil {
		t.Fatalf("dynamicDispatchSource failed: %v", err)
	}
	if source != "" {
		t.Fatalf("dynamicDispatchSource emitted dead declarations for an unusable interface:\n%s", source)
	}
}

// TestGenerateSourceDeeplyNestedVariantSwitchHasNoUnusedBinding covers the
// same defect one level deeper: a three-level pattern next to a bare nullary
// sibling.
func TestGenerateSourceDeeplyNestedVariantSwitchHasNoUnusedBinding(t *testing.T) {
	src := `package sample

func readDeep() -> Result[Option[Result[Int, String]], String]
  Ok(Some(Ok(1)))
end

func toStep() -> Result[Bool, String]
  switch readDeep()
    case Ok(Some(Ok(x))) => Ok(x > 0)
    case Ok(Some(Err(e))) => Err(e)
    case Ok(None) => Ok(false)
    case Err(e) => Err(e)
  end
end
`
	result := GenerateSource(src)
	code, ok := result.(Result__Ok[string, string])
	if !ok {
		t.Fatalf("GenerateSource failed: %v", result)
	}
	t.Logf("Generated code:\n%s", code.F0)
	assertNoUnusedBindings(t, code.F0)
}
