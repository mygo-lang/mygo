package typeinference

import (
	"go/types"
	"testing"
)

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

func TestGoSignatureCanonicalizesError(t *testing.T) {
	errorType := types.Universe.Lookup("error").Type()
	sig := types.NewSignatureType(
		nil,
		nil,
		nil,
		types.NewTuple(),
		types.NewTuple(types.NewVar(0, nil, "err", errorType)),
		false,
	)

	got := goSignatureType(sig)
	result, ok := got.Ret.(TCon)
	if !ok || result.Name != "Error" {
		t.Fatalf("Go error return type = %s, want Error", got.Ret)
	}
}
