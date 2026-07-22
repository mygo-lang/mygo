package typeinference

import "testing"

func TestLoadGoPackageInfoResolvesCurrentModule(t *testing.T) {
	info, err := loadGoPackageInfo(
		"goast",
		"github.com/mygo-lang/mygo/internal/mygo/codegen2/goast",
		"",
	)
	if err != nil {
		t.Fatalf("loadGoPackageInfo() error = %v", err)
	}
	if _, ok := info.Funcs["Ident"]; !ok {
		t.Fatalf("loaded functions = %#v, want Ident", info.Funcs)
	}
	if _, ok := info.Funcs["StructDeclFromParts"]; !ok {
		t.Fatalf("loaded functions = %#v, want StructDeclFromParts", info.Funcs)
	}
}

func TestLoadGoPackageInfoPreservesExportedAliasName(t *testing.T) {
	info, err := loadGoPackageInfo(
		"goast",
		"github.com/mygo-lang/mygo/internal/mygo/codegen2/goast",
		"",
	)
	if err != nil {
		t.Fatalf("loadGoPackageInfo() error = %v", err)
	}
	got := info.Funcs["String"].Ret
	if got.String() != "goast.Expr" {
		t.Fatalf("String() return type = %s, aliases = %#v, want goast.Expr", got, info.Aliases)
	}
}
