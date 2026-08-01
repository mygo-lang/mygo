package typeinference

import (
	"go/types"
	"os"
	"path/filepath"
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

func TestLoadMyGoPackageInfoQualifiesExportedTypeAliases(t *testing.T) {
	dir := t.TempDir()
	src := `package parsec

type Parser[A] = func(A) -> A

func Identity[A](p: Parser[A]) -> Parser[A]
  p
end
`
	if err := os.WriteFile(filepath.Join(dir, "parsec.mygo"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := loadMyGoPackageInfo("", dir, dir, "ps", nil)
	if err != nil {
		t.Fatalf("loadMyGoPackageInfo() error = %v", err)
	}
	if _, ok := info.Types["Parser"]; !ok {
		t.Fatal("exported type alias Parser was not registered as an imported type")
	}
	identity := Instantiate(info.Funcs["Identity"], NewInferState())
	got := qualifyMyGoType("ps", info.Types, identity)
	if got.String() != "t1 -> t1 -> t1 -> t1" {
		t.Fatalf("qualified Identity type = %s, want Parser aliases to be expanded", got)
	}
	state := NewInferState()
	state.MyGoPackages["ps"] = info
	got = expandImportedTypeAliases(state, TCon{Name: "ps.Parser", Args: []MonoType{TVar{ID: 1}}})
	if got.String() != "t1 -> t1" {
		t.Fatalf("expanded Parser type = %s, want function type", got)
	}
}

func TestParsecParserAliasIsLoaded(t *testing.T) {
	info, err := loadMyGoPackageInfo("", "../../../examples/parsec/json", "../../../lib/text/parsec", "ps", nil)
	if err != nil {
		t.Fatal(err)
	}
	if info.TypeAliases["Parser"] == nil {
		t.Fatal("parsec Parser alias was not loaded")
	}
}
