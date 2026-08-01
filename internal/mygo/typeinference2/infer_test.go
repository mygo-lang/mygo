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
