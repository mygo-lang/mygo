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
}
