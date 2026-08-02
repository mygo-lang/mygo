package typeinference2

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/parser2"
	. "github.com/mygo-lang/mygo/prelude"
)

func TestInferFilePrelude(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test path")
	}
	sourcePath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "prelude", "prelude.mygo")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read %s: %v", sourcePath, err)
	}
	parsed := parser2.ParseFileAt(sourcePath, string(source))
	file, ok := parsed.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFileAt(%s) failed: %v", sourcePath, parsed)
	}
	if got := InferFile(file.F0); !isPackageInfo(got) {
		t.Fatalf("InferFile(%s) failed: %v", sourcePath, got)
	}
}

func TestInferErrorIncludesExpressionPosition(t *testing.T) {
	parsed := parser2.ParseFile(`package sample

func broken() -> Int
  missing
end
`)
	file, ok := parsed.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	got := InferFile(file.F0)
	err, ok := got.(ResultErr[PackageInfo, string])
	if !ok {
		t.Fatalf("InferFile unexpectedly succeeded: %v", got)
	}
	if !strings.Contains(err.F0, "<input>:4:3: unknown identifier missing") {
		t.Fatalf("inference error lacks expression position: %q", err.F0)
	}
}

func TestInferSuffixedNumericLiterals(t *testing.T) {
	tests := []struct {
		literal string
		want    string
	}{
		{"42i8", "Int8"},
		{"200u8", "UInt8"},
		{"-1000i16", "Int16"},
		{"60000u16", "UInt16"},
		{"-100000i32", "Int32"},
		{"3000000000u32", "UInt32"},
		{"9223372036854775807i64", "Int64"},
		{"18_446744073_709_551_615u", "UInt"},
		{"18_446744073_709_551_615u64", "UInt64"},
		{"3.14f32", "Float32"},
		{"2.718281828459045f64", "Float64"},
		{"3.14", "Float64"},
		{"42", "Int"},
	}
	for _, tt := range tests {
		parsed := parser2.ParseFile("package sample\n\nlet value = " + tt.literal + "\n")
		file, ok := parsed.(ResultOk[ast2.File, string])
		if !ok {
			t.Fatalf("ParseFile(%s) failed: %v", tt.literal, parsed)
		}
		result := InferFile(file.F0)
		info, ok := result.(ResultOk[PackageInfo, string])
		if !ok {
			t.Fatalf("InferFile(%s) failed: %v", tt.literal, result)
		}
		binding := info.F0.TypedDecls[0].(ast2.DeclLetDecl).F0
		got := binding.Value.Type.(OptionSome[ast2.MonoType]).F0
		if name, ok := got.(ast2.MonoTypeTCon); !ok || name.F0 != tt.want {
			t.Errorf("%s inferred as %v, want %s", tt.literal, got, tt.want)
		}
	}
}

func TestInferSetLiteralRejectsDuplicateLiteralElement(t *testing.T) {
	parsed := parser2.ParseFile(`package sample

func words() -> Set[String]
  {"hello", "world", "hello"}
end
`)
	file, ok := parsed.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	result := InferFile(file.F0)
	err, ok := result.(ResultErr[PackageInfo, string])
	if !ok {
		t.Fatalf("InferFile unexpectedly succeeded: %v", result)
	}
	if !strings.Contains(err.F0, "<input>:4:22: duplicate element in set literal") {
		t.Fatalf("unexpected duplicate-set error: %q", err.F0)
	}
}

func TestInferAssignmentRetainsTypedMethodCall(t *testing.T) {
	parsed := parser2.ParseFile(`package sample

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
`)
	file, ok := parsed.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	result := InferFile(file.F0)
	info, ok := result.(ResultOk[PackageInfo, string])
	if !ok {
		t.Fatalf("InferFile failed: %v", result)
	}
	var body ast2.Expr
	for _, decl := range info.F0.TypedDecls {
		if fn, ok := decl.(ast2.DeclFuncDecl); ok && fn.F0 == "update" {
			body = fn.F4
		}
	}
	block, ok := body.Kind.(ast2.ExprKindBlockExpr)
	if !ok {
		t.Fatalf("update body = %#v, want block", body.Kind)
	}
	assignment, ok := block.F0[1].(ast2.StmtAssignStmt)
	if !ok {
		t.Fatalf("second statement = %#v, want assignment", block.F0[1])
	}
	call, ok := assignment.F1.Kind.(ast2.ExprKindCallExpr)
	if !ok {
		t.Fatalf("assignment RHS = %#v, want call", assignment.F1.Kind)
	}
	field, ok := call.F0.Kind.(ast2.ExprKindFieldExpr)
	if !ok {
		t.Fatalf("call callee = %#v, want field", call.F0.Kind)
	}
	if _, ok := field.F0.Type.(OptionSome[ast2.MonoType]); !ok {
		t.Fatalf("Append receiver lost its inferred type: %#v", field.F0.Type)
	}
}

func TestInferFunctionUsingMarksDictionaryCall(t *testing.T) {
	parsed := parser2.ParseFile(`package sample

interface Show[A]
  func Show(value: A) -> String
end

func render[A](value: A) -> String using Show[A]
  value.Show()
end
`)
	file, ok := parsed.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	result := InferFile(file.F0)
	info, ok := result.(ResultOk[PackageInfo, string])
	if !ok {
		t.Fatalf("InferFile failed: %v", result)
	}
	for _, decl := range info.F0.TypedDecls {
		if fn, ok := decl.(ast2.DeclFuncDecl); ok && fn.F0 == "render" {
			block, ok := fn.F4.Kind.(ast2.ExprKindBlockExpr)
			if !ok || len(block.F0) != 1 {
				t.Fatalf("render body = %#v, want single-expression block", fn.F4.Kind)
			}
			expr, ok := block.F0[0].(ast2.StmtExprStmt)
			if !ok {
				t.Fatalf("render statement = %#v, want expression", block.F0[0])
			}
			if _, ok := expr.F0.Kind.(ast2.ExprKindDictionaryCallExpr); !ok {
				t.Fatalf("render expression = %#v, want DictionaryCallExpr", expr.F0.Kind)
			}
			return
		}
	}
	t.Fatal("typed render declaration not found")
}

func TestInferImplUsingMarksDictionaryCall(t *testing.T) {
	parsed := parser2.ParseFile(`package sample

interface Eq[A]
  func Equals(left: A, right: A) -> Bool
end

struct Box[A]
  Value: A
end

impl[A] BoxEq[A]: Eq[Box[A]]
  func Equals(left: Box[A], right: Box[A]) -> Bool using Eq[A]
    left.Value.Equals(right.Value)
  end
end
`)
	file, ok := parsed.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	result := InferFile(file.F0)
	info, ok := result.(ResultOk[PackageInfo, string])
	if !ok {
		t.Fatalf("InferFile failed: %v", result)
	}
	if !typedDeclsContainDictionaryCall(info.F0.TypedDecls) {
		t.Fatal("typed impl body did not retain DictionaryCallExpr")
	}
}

func typedDeclsContainDictionaryCall(decls []ast2.Decl) bool {
	for _, decl := range decls {
		if impl, ok := decl.(ast2.DeclImplDecl); ok {
			for _, method := range impl.F3 {
				if typedExprContainsDictionaryCall(method.Body) {
					return true
				}
			}
		}
	}
	return false
}

func typedExprContainsDictionaryCall(expr ast2.Expr) bool {
	switch kind := expr.Kind.(type) {
	case ast2.ExprKindDictionaryCallExpr:
		return true
	case ast2.ExprKindCallExpr:
		if typedExprContainsDictionaryCall(kind.F0) {
			return true
		}
		for _, arg := range kind.F2 {
			if typedExprContainsDictionaryCall(arg) {
				return true
			}
		}
	case ast2.ExprKindFieldExpr:
		return typedExprContainsDictionaryCall(kind.F0)
	case ast2.ExprKindSwitchExpr:
		if typedExprContainsDictionaryCall(kind.F0) {
			return true
		}
		for _, item := range kind.F1 {
			if typedExprContainsDictionaryCall(item.Body) {
				return true
			}
		}
	case ast2.ExprKindBlockExpr:
		for _, stmt := range kind.F0 {
			if exprStmt, ok := stmt.(ast2.StmtExprStmt); ok && typedExprContainsDictionaryCall(exprStmt.F0) {
				return true
			}
		}
	}
	return false
}

func TestInferHKTApplicationRecoversElementType(t *testing.T) {
	parsed := parser2.ParseFile(`package sample

func Keep[C[A], A](c: C[A]) -> C[A]
  c
end

func Use() -> Slice[Int]
  Keep([1])
end
`)
	file, ok := parsed.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile failed: %v", parsed)
	}
	if got := InferFile(file.F0); !isPackageInfo(got) {
		t.Fatalf("HKT inference failed: %v", got)
	}
}

func TestInferTypeAliasIsInterchangeableButDefinedTypeIsNot(t *testing.T) {
	alias := parser2.ParseFile(`package sample

type UserID = Int

func accepts(value: UserID) -> Int
  value
end

func use() -> Int
  accepts(1)
end
`)
	aliasFile, ok := alias.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile alias failed: %v", alias)
	}
	if got := InferFile(aliasFile.F0); !isPackageInfo(got) {
		t.Fatalf("type alias did not unify with its target: %v", got)
	}

	defined := parser2.ParseFile(`package sample

type UserID Int

func accepts(value: UserID) -> Int
  0
end

func use() -> Int
  accepts(1)
end
`)
	definedFile, ok := defined.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile defined type failed: %v", defined)
	}
	if _, ok := InferFile(definedFile.F0).(ResultErr[PackageInfo, string]); !ok {
		t.Fatal("defined type unexpectedly unified with its underlying Int")
	}
}

func TestInferPackageWithExternalRetainsUserTypeAliasForCodegen(t *testing.T) {
	user := parser2.ParseFile(`package sample

type Parser[A] = func(Int) -> A
`)
	userFile, ok := user.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile user failed: %v", user)
	}
	external := parser2.ParseFile(`package prelude

struct External
  Value: Int
end
`)
	externalFile, ok := external.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile external failed: %v", external)
	}
	got := InferPackageWithExternal(
		[]PkgDeclSource{{Path: "parsec.mygo", Decls: userFile.F0.Decls}},
		[]PkgDeclSource{{Path: "prelude.mygo", Decls: externalFile.F0.Decls}},
		[]GoPackageEntry{},
		[]MyGoPackageInfo{},
	)
	info, ok := got.(ResultOk[PackageInfo, string])
	if !ok {
		t.Fatalf("InferPackageWithExternal failed: %v", got)
	}
	if !declsContainTypeAlias(info.F0.TypedDecls, "Parser") {
		t.Fatalf("user type alias absent from TypedDecls: %v", info.F0.TypedDecls)
	}
	if declsContainTypeAlias(info.F0.ExternalTypedDecls, "Parser") {
		t.Fatalf("user type alias was classified as external: %v", info.F0.ExternalTypedDecls)
	}
}

func declsContainTypeAlias(decls []ast2.Decl, name string) bool {
	for _, decl := range decls {
		if alias, ok := decl.(ast2.DeclTypeAliasDecl); ok && alias.F0 == name {
			return true
		}
	}
	return false
}

func isPackageInfo(value Result[PackageInfo, string]) bool {
	_, ok := value.(ResultOk[PackageInfo, string])
	return ok
}

func TestComposeSubstDefersResolutionAndPreservesPrecedence(t *testing.T) {
	older := SubstFromEntries([]SubstEntry{{ID: 1, Type: ast2.MonoTypeTVarCtor(2)}})
	newer := SubstFromEntries([]SubstEntry{{ID: 2, Type: ast2.MonoTypeTConCtor("Int")}})
	got := applySubst(composeSubst(newer, older), ast2.MonoTypeTVarCtor(1))
	if !ast2.MonoEqual(got, ast2.MonoTypeTConCtor("Int")) {
		t.Fatalf("lazy substitution chain resolved to %v, want Int", got)
	}

	// composeSubst historically gives the accumulated (older) mapping lookup
	// precedence. Keep that contract while making the composition lazy.
	older = SubstFromEntries([]SubstEntry{{ID: 1, Type: ast2.MonoTypeTConCtor("String")}})
	newer = SubstFromEntries([]SubstEntry{{ID: 1, Type: ast2.MonoTypeTConCtor("Int")}})
	got = applySubst(composeSubst(newer, older), ast2.MonoTypeTVarCtor(1))
	if !ast2.MonoEqual(got, ast2.MonoTypeTConCtor("String")) {
		t.Fatalf("composeSubst precedence changed: got %v, want String", got)
	}
}

// TestImportedTypeSurvivesGenericMethodInstantiation is a bootstrap
// regression test for typeExprListString in env.mygo.  Slice.Get instantiates
// its element variable with ast2.TypeExpr; that qualified identity must remain
// intact through receiver matching and the method result.  Losing the
// qualification here makes the following call compare bare TypeExpr with
// ast2.TypeExpr.
func TestImportedTypeSurvivesGenericMethodInstantiation(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test path")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	astDecls := parseMyGoDecls(t, filepath.Join(root, "internal", "mygo", "ast2"))
	preludeDecls := parseMyGoFiles(t,
		filepath.Join(root, "prelude", "prelude.mygo"),
		filepath.Join(root, "prelude", "slice.mygo"),
	)
	user := parser2.ParseFile(`package sample

import ast2 "github.com/mygo-lang/mygo/internal/mygo/ast2"

func typeExprString(t: ast2.TypeExpr) -> String
  ""
end

func typeExprListString(items: Slice[ast2.TypeExpr]) -> String
  let tail = ""
  switch items.Get(0)
    case Some(item) => typeExprString(item)
    case None => tail
  end
end

func emptyParam() -> ast2.Param
  ast2.Param { Name: "", Type: ast2.TypeExpr.UnitType, Tag: None }
end
`)
	file, ok := user.(ResultOk[ast2.File, string])
	if !ok {
		t.Fatalf("ParseFile user failed: %v", user)
	}
	allDecls := append(append([]ast2.Decl{}, file.F0.Decls...), preludeDecls...)
	imports := collectMyGoPackageImports(file.F0.Decls)
	seeded := seedMyGoPackageEnv(imports, []MyGoPackageInfo{{
		Alias: "ast2",
		Path:  "github.com/mygo-lang/mygo/internal/mygo/ast2",
		Decls: astDecls,
	}}, initialEnv())
	if _, ok := envGet(predeclareAllFunctions(allDecls, seeded), "typeExprListString").(OptionSome[Scheme]); !ok {
		t.Fatal("typeExprListString was not predeclared")
	}
	got := InferPackageWithExternal(
		[]PkgDeclSource{{Path: "sample.mygo", Decls: file.F0.Decls}},
		[]PkgDeclSource{{Path: "prelude.mygo", Decls: preludeDecls}},
		collectGoPackageImports(allDecls),
		[]MyGoPackageInfo{{
			Alias: "ast2",
			Path:  "github.com/mygo-lang/mygo/internal/mygo/ast2",
			Decls: astDecls,
		}},
	)
	if _, ok := got.(ResultOk[PackageInfo, string]); !ok {
		t.Fatalf("qualified Slice.Get result did not survive method instantiation: %v", got)
	}
}

func TestQualifiedTypeLookupFallsBackToImportCache(t *testing.T) {
	state := NewInferState()
	state.MyGoPackageCache = []MyGoPackageInfo{{
		Alias: "ast2",
		Path:  "github.com/mygo-lang/mygo/internal/mygo/ast2",
	}}
	got := typeFromASTInEnvWithParams(
		ast2.TypeExprNamedTypeCtor("ast2.MonoType", []ast2.TypeExpr{}),
		[]string{},
		initialEnv(),
		state,
	)
	inner := ast2.MonoTypeTConCtor("MonoType")
	want := ast2.MonoTypeTQualifiedNameCtor(
		"github.com/mygo-lang/mygo/internal/mygo/ast2",
		&inner,
	)
	if !ast2.MonoEqual(got, want) {
		t.Fatalf("cached imported type became %v, want %v", got, want)
	}
}

func TestImportedGenericTypeAliasExpandsInFunctionParameters(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test path")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	decls := parseMyGoFiles(t, filepath.Join(root, "lib", "text", "parsec", "parsec.mygo"))
	privateEnv := seedMyGoPackageDecls(MyGoPackageInfo{
		Alias: "ps",
		Path:  "github.com/mygo-lang/mygo/lib/text/parsec",
		Decls: decls,
	}, 0, initialEnv(), []MyGoPackageInfo{})
	privateBetween, ok := envGet(privateEnv, "PBetween").(OptionSome[Scheme])
	if !ok {
		t.Fatal("PBetween was not seeded in the package-private environment")
	}
	if _, ok := privateBetween.F0.Body.(ast2.MonoTypeTFunc); !ok {
		t.Fatalf("private PBetween body = %v, want bare function type", privateBetween.F0.Body)
	}
	if _, ok := envGet(privateEnv, "ps.PBetween").(OptionSome[Scheme]); ok {
		t.Fatal("qualified PBetween leaked into package-private environment")
	}
	env := seedMyGoPackageEnv(
		[]struct{ F0, F1 string }{{F0: "ps", F1: "github.com/mygo-lang/mygo/lib/text/parsec"}},
		[]MyGoPackageInfo{{Alias: "ps", Path: "github.com/mygo-lang/mygo/lib/text/parsec", Decls: decls}},
		initialEnv(),
	)
	parser, ok := envGet(env, "ps.Parser").(OptionSome[Scheme])
	if !ok {
		t.Fatal("ps.Parser was not seeded")
	}
	if _, ok := envGet(env, "Parser").(OptionSome[Scheme]); ok {
		t.Fatal("package-local Parser alias leaked into importing environment")
	}
	if _, ok := parser.F0.Body.(ast2.MonoTypeTFunc); !ok {
		t.Fatalf("ps.Parser body = %v, want expanded function type", parser.F0.Body)
	}
	between, ok := envGet(env, "ps.PBetween").(OptionSome[Scheme])
	if !ok {
		t.Fatal("ps.PBetween was not seeded")
	}
	fn, ok := between.F0.Body.(ast2.MonoTypeTQualifiedName)
	if !ok {
		t.Fatalf("ps.PBetween body = %v, want qualified function", between.F0.Body)
	}
	inner, ok := (*fn.F1).(ast2.MonoTypeTFunc)
	if !ok {
		t.Fatalf("ps.PBetween inner = %v, want function", fn.F1)
	}
	if _, ok := inner.F0[0].(ast2.MonoTypeTFunc); !ok {
		t.Fatalf("ps.PBetween first parameter = %v, want expanded Parser function", inner.F0[0])
	}
}

func parseMyGoDecls(t *testing.T, dir string) []ast2.Decl {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.mygo"))
	if err != nil {
		t.Fatalf("list %s: %v", dir, err)
	}
	return parseMyGoFiles(t, paths...)
}

func parseMyGoFiles(t *testing.T, paths ...string) []ast2.Decl {
	t.Helper()
	var decls []ast2.Decl
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		parsed := parser2.ParseFileAt(path, string(source))
		file, ok := parsed.(ResultOk[ast2.File, string])
		if !ok {
			t.Fatalf("parse %s: %v", path, parsed)
		}
		decls = append(decls, file.F0.Decls...)
	}
	return decls
}
