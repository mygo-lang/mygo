package typeinference2

import (
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	"github.com/mygo-lang/mygo/internal/mygo/parser2"
	. "github.com/mygo-lang/mygo/prelude"
)

func TestImportedMethodRetainsThirdPackageIdentity(t *testing.T) {
	parse := func(source string) []ast2.Decl {
		t.Helper()
		result := parser2.ParseFileAt("transitive.mygo", source)
		file, ok := result.(Result__Ok[ast2.File, string])
		if !ok {
			t.Fatalf("parse: %#v", result)
		}
		return file.F0.Decls
	}
	packages := []MyGoPackageInfo{
		{Alias: "auth", Path: "example.com/auth", Decls: parse("package auth\nstruct Identity\n Name: String\nend\n")},
		{Alias: "other", Path: "example.com/other", Decls: parse("package other\nstruct Identity\n ID: Int\nend\n")},
		{Alias: "storage", Path: "example.com/storage", Decls: parse(`package storage
import auth "example.com/auth"
import other "example.com/other"
struct Identity
  Local: Bool
end
struct Repository
  ID: Int
end
impl Repository
  func Create(self: Repository) -> Slice[auth.Identity]
    [] as Slice[auth.Identity]
  end
  func Other(self: Repository) -> Slice[other.Identity]
    [] as Slice[other.Identity]
  end
end
func Create() -> Slice[auth.Identity]
  [] as Slice[auth.Identity]
end
`)},
	}
	result := cachedImportedPackage(packages[2], "storage", packages, nil)
	cached, ok := result.(Result__Ok[ImportedPackageCacheEntry, string])
	if !ok {
		t.Fatalf("import: %#v", result)
	}
	identities := map[string]ast2.MonoType{}
	for _, symbol := range cached.F0.ImplMethodSymbols {
		method, ok := symbol.(Symbol__ImplMethod)
		if !ok {
			continue
		}
		fn := method.F2.Body.(ast2.MonoType__TFunc)
		app := (*fn.F1).(ast2.MonoType__TApp)
		qualified, ok := app.F1[0].(ast2.MonoType__TQualifiedName)
		if !ok {
			t.Fatalf("%s return lost package: %s", method.F1, MonoStringFull(method.F2.Body))
		}
		want := "example.com/auth"
		if method.F1 == "Other" {
			want = "example.com/other"
		}
		if qualified.F0 != want {
			t.Fatalf("%s path = %s, want %s", method.F1, qualified.F0, want)
		}
		if inner, ok := (*qualified.F1).(ast2.MonoType__TCon); !ok || inner.F0 != "Identity" {
			t.Fatalf("foreign nominal type was requalified by storage: %s", MonoStringFull(app.F1[0]))
		}
		identities[method.F1] = app.F1[0]
	}
	if len(identities) != 2 {
		t.Fatalf("missing methods: %v", identities)
	}
	if ast2.MonoEqual(identities["Create"], identities["Other"]) {
		t.Fatal("distinct nominal types compare equal")
	}
	for _, entry := range cached.F0.Exported {
		if entry.Name == "storage.Create" {
			qualified := entry.Scheme.Body.(ast2.MonoType__TQualifiedName)
			fn := (*qualified.F1).(ast2.MonoType__TFunc)
			app := (*fn.F1).(ast2.MonoType__TApp)
			if !ast2.MonoEqual(app.F1[0], identities["Create"]) {
				t.Fatal("function and method disagree on foreign identity")
			}
			return
		}
	}
	t.Fatal("missing exported Create function")
}

func TestQualifiedGenericConstructorKeepsOwnerDuringExport(t *testing.T) {
	ctor := ast2.MonoType(ast2.MonoType__TCon{F0: "Box"})
	local := ast2.MonoType(ast2.MonoType__TCon{F0: "Local"})
	app := ast2.MonoType(ast2.MonoType__TApp{F0: &ctor, F1: []ast2.MonoType{local}})
	qualified := ast2.MonoType__TQualifiedName{F0: "example.com/auth", F1: &app}
	result := wrapPkgTypesInMonoType(qualified, "example.com/storage", []string{"Box", "Local"})
	wrapped, ok := result.(Option__Some[ast2.MonoType])
	if !ok {
		t.Fatalf("wrapping failed: %#v", result)
	}
	localQualified := ast2.MonoType(ast2.MonoType__TQualifiedName{F0: "example.com/storage", F1: &local})
	wantApp := ast2.MonoType(ast2.MonoType__TApp{F0: &ctor, F1: []ast2.MonoType{localQualified}})
	want := ast2.MonoType__TQualifiedName{F0: "example.com/auth", F1: &wantApp}
	if !ast2.MonoEqual(wrapped.F0, want) {
		t.Fatalf("got %s, want %s", MonoStringFull(wrapped.F0), MonoStringFull(want))
	}
}
