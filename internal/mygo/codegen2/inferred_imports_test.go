package codegen2

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/codegen2/goast"
	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
	. "github.com/mygo-lang/mygo/prelude"
)

func foreignType(path, name string) ast2.MonoType {
	inner := ast2.MonoType(ast2.MonoType__TCon{F0: name})
	return ast2.MonoType__TQualifiedName{F0: path, F1: &inner}
}

func TestInferredImportsCoverNestedTypesAndAliasConflicts(t *testing.T) {
	first := foreignType("example.com/a/model", "Identity")
	second := foreignType("example.com/b/model", "Identity")
	ctor := ast2.MonoType(ast2.MonoType__TCon{F0: "Result"})
	sliceCtor := ast2.MonoType(ast2.MonoType__TCon{F0: "Slice"})
	mapCtor := ast2.MonoType(ast2.MonoType__TCon{F0: "Map"})
	setCtor := ast2.MonoType(ast2.MonoType__TCon{F0: "Set"})
	shapes := []ast2.MonoType{
		ast2.MonoType__TApp{F0: &ctor, F1: []ast2.MonoType{first, second}},
		ast2.MonoType__TArray{F0: &first, F1: 2},
		ast2.MonoType__TTuple{F0: []ast2.MonoType{second, first}},
		ast2.MonoType__TFunc{F0: []ast2.MonoType{first}, F1: &second},
		ast2.MonoType__TVariadic{F0: []ast2.MonoType{second}, F1: &first},
		ast2.MonoType__TApp{F0: &sliceCtor, F1: []ast2.MonoType{first}},
		ast2.MonoType__TApp{F0: &mapCtor, F1: []ast2.MonoType{first, second}},
		ast2.MonoType__TApp{F0: &setCtor, F1: []ast2.MonoType{second}},
	}
	expected := [][]string{
		{"example.com/a/model", "example.com/b/model"},
		{"example.com/a/model"},
		{"example.com/b/model", "example.com/a/model"},
		{"example.com/a/model", "example.com/b/model"},
		{"example.com/b/model", "example.com/a/model"},
		{"example.com/a/model"},
		{"example.com/a/model", "example.com/b/model"},
		{"example.com/b/model"},
	}
	for index, shape := range shapes {
		out := &TypeImportPaths{Seen: map[string]bool{}}
		collectMonoImportPaths(shape, out)
		if !reflect.DeepEqual(out.Paths, expected[index]) {
			t.Fatalf("dependencies for %s: got %v, want %v", typeinference2.MonoStringFull(shape), out.Paths, expected[index])
		}
	}
	packages := []typeinference2.MyGoPackageInfo{
		{Alias: "model", Path: "example.com/a/model"},
		{Alias: "model", Path: "example.com/b/model"},
	}
	existing := []GoImportPart{{Alias: "model", Path: "strings"}}
	paths := []string{packages[0].Path, packages[1].Path, packages[0].Path}
	imports := inferredMyGoTypeImports(paths, packages, "example.com/server", existing)
	if len(imports) != 2 || imports[0].Alias != "model2" || imports[1].Alias != "model3" {
		t.Fatalf("aliases: %#v", imports)
	}
	if !reflect.DeepEqual(imports, inferredMyGoTypeImports(paths, packages, "example.com/server", existing)) {
		t.Fatal("unstable imports")
	}
	aliases := pathAliasesFromImports(imports)
	if got := monoTypeToGoStrIn(shapes[0], aliases); got != "Result[model2.Identity, model3.Identity]" {
		t.Fatal(got)
	}
	if got := goast.MustExprSource(monoTypeToGoAstWithParams(shapes[0], nil, aliases)); got != "Result[model2.Identity, model3.Identity]" {
		t.Fatal(got)
	}
	existing = append(existing, GoImportPart{Alias: "identity", Path: packages[0].Path})
	imports = inferredMyGoTypeImports(paths, packages, packages[1].Path, existing)
	if len(imports) != 0 {
		t.Fatalf("duplicate or self import: %#v", imports)
	}
}

func TestQualifiedTypeRenderingContexts(t *testing.T) {
	foreign := foreignType("example.com/auth", "Identity")
	for _, alias := range []string{"identity", ".", ""} {
		aliases := map[string]string{"$mygo$strict": "true", "example.com/auth": alias}
		want := "Identity"
		if alias == "identity" {
			want = "identity.Identity"
		}
		if got := monoTypeToGoStrIn(foreign, aliases); got != want {
			t.Fatal(got)
		}
		if got := goast.MustExprSource(monoTypeToGoAstWithParams(foreign, nil, aliases)); got != want {
			t.Fatal(got)
		}
		if aliases["$mygo$error"] != "" {
			t.Fatal(aliases["$mygo$error"])
		}
	}
	param := ast2.MonoType(ast2.MonoType__TParam{F0: -1})
	boxCtor := ast2.MonoType(ast2.MonoType__TCon{F0: "Box"})
	inner := ast2.MonoType(ast2.MonoType__TApp{F0: &boxCtor, F1: []ast2.MonoType{param}})
	box := ast2.MonoType__TQualifiedName{F0: "example.com/auth", F1: &inner}
	aliases := map[string]string{"example.com/auth": "auth"}
	if got := goast.MustExprSource(monoTypeToGoAstWithParams(box, []string{"A"}, aliases)); got != "auth.Box[A]" {
		t.Fatal(got)
	}
	if got := monoTypeToGoStr(foreign); got != "Identity" {
		t.Fatalf("diagnostic formatting changed: %s", got)
	}
	for _, structured := range []bool{false, true} {
		strict := map[string]string{"$mygo$strict": "true"}
		if structured {
			monoTypeToGoAstWithParams(foreign, nil, strict)
		} else {
			monoTypeToGoStrIn(foreign, strict)
		}
		if message := strict["$mygo$error"]; !strings.Contains(message, "Identity") || !strings.Contains(message, "example.com/auth") {
			t.Fatalf("missing diagnostic: %v", strict)
		}
	}
}

func TestGenerateFilesRejectsUnresolvedForeignType(t *testing.T) {
	foreign := foreignType("example.com/missing", "Identity")
	intType := ast2.MonoType(ast2.MonoType__TCon{F0: "Int"})
	tuple := ast2.MonoType(ast2.MonoType__TTuple{F0: []ast2.MonoType{foreign, intType}})
	body := ast2.Expr{Kind: ast2.ExprKind__BlockExpr{F0: []ast2.Stmt{
		ast2.Stmt__TupleLetStmt{F0: ast2.Pattern__TuplePattern{F0: []ast2.Pattern{ast2.Pattern__BindPattern{F0: "item"}, ast2.Pattern__WildcardPattern{}}}, F1: ast2.Expr{Kind: ast2.ExprKind__IdentExpr{F0: "input"}, Type: Some[ast2.MonoType](tuple)}},
		ast2.Stmt__ExprStmt{F0: ast2.Expr{Kind: ast2.ExprKind__UnitExpr{}, Type: Some[ast2.MonoType](ast2.MonoType__TUnit{})}},
	}}, Type: Some[ast2.MonoType](ast2.MonoType__TUnit{})}
	decl := ast2.Decl__FuncDecl{F0: "Use", F2: []ast2.Param{{Name: "input", Type: ast2.TypeExpr__NamedType{F0: "Identity"}}}, F3: Some[ast2.TypeExpr](ast2.TypeExpr__UnitType{}), F4: body}
	file := ast2.File{PackageName: "sample", Decls: []ast2.Decl{decl}}
	info := typeinference2.PackageInfo{TypedDecls: file.Decls, TypedDeclSources: []typeinference2.PkgDeclSource{{Path: "sample.mygo", Decls: file.Decls}}}
	result := GenerateFiles([]SourceFileInput{{Path: "sample.mygo", File: file, PackageImportPath: "example.com/sample"}}, info)
	err, ok := result.(Result__Err[map[string]string, string])
	if !ok || !strings.Contains(err.F0, "Identity") || !strings.Contains(err.F0, "example.com/missing") {
		t.Fatalf("expected unresolved type error, got %#v", result)
	}
}
