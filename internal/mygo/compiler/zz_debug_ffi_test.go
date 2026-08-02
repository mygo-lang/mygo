package compiler

import (
	"fmt"
	"testing"
)

func TestDebugGoastFFI(t *testing.T) {
	root := "/mnt/data-svr1-raid/xyh/code/go/mygo"
	path := "github.com/mygo-lang/mygo/internal/mygo/codegen2/goast"

	// Test the module importer directly.
	imp := newGoModuleImporter(root)
	dirOpt := bootstrapGoImportDir(root, path)
	fmt.Printf("bootstrapGoImportDir(%q, %q) = %#v\n", root, path, dirOpt)

	pkg, err := imp.Import(path)
	fmt.Printf("Import result: pkg=%v err=%v\n", pkg, err)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
}
