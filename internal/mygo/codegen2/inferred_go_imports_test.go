package codegen2

import (
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/typeinference2"
)

func TestInferredGoTypeImportsIgnoreUnreferencedPackages(t *testing.T) {
	packages := []typeinference2.GoPackageEntry{{Alias: "token", Path: "go:go/token"}}
	if got := inferredGoTypeImports(nil, packages, nil, 0, nil); len(got) != 0 {
		t.Fatalf("unreferenced dependency package produced imports %#v", got)
	}
}

func TestInferredGoTypeImportsCoverReferencedPaths(t *testing.T) {
	packages := []typeinference2.GoPackageEntry{{Alias: "token", Path: "go:go/token"}}
	got := inferredGoTypeImports([]string{"go:go/token"}, packages, nil, 0, nil)
	if len(got) != 1 || got[0].Alias != "token" || got[0].Path != "go/token" {
		t.Fatalf("imports = %#v, want token go/token", got)
	}
}

func TestInferredGoTypeImportsKeepExplicitAlias(t *testing.T) {
	packages := []typeinference2.GoPackageEntry{{Alias: "gofmt", Path: "go:fmt"}}
	got := inferredGoTypeImports([]string{"go:fmt"}, packages, nil, 0, nil)
	if len(got) != 1 || got[0].Alias != "gofmt" || got[0].Path != "fmt" {
		t.Fatalf("imports = %#v, want gofmt fmt", got)
	}
}
