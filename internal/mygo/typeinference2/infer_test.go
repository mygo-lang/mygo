package typeinference2

import (
	"os"
	"path/filepath"
	"runtime"
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

func isPackageInfo(value Result[PackageInfo, string]) bool {
	_, ok := value.(ResultOk[PackageInfo, string])
	return ok
}
