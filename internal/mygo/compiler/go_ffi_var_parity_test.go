package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

// varFixture selects one exported package-level const and one exported
// package-level var from the same go:-imported package.  os exports both
// shapes (O_RDONLY and ErrProcessDone) without pulling a third-party module
// into this package's test dependencies.
const varFixture = `package sample

import os "go:os"

func Both() -> Error
  let flag: Int = os.O_RDONLY
  let sentinel: Error = os.ErrProcessDone
  sentinel
end
`

// TestGoFfiVarParityAcrossPipelines pins that the bootstrap collector and the
// hand-written typeinference loader accept the same package-level surface.
// Before the var pass each loader dropped exported vars, so both rejected
// os.ErrProcessDone; a fix applied to only one pipeline would show up here as
// one pipeline failing and the other passing.
func TestGoFfiVarParityAcrossPipelines(t *testing.T) {
	t.Run("bootstrap", func(t *testing.T) {
		dir := t.TempDir()
		bootstrapTestModule(t, dir)
		if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(varFixture), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := CompileDirBootstrap(dir); err != nil {
			t.Fatalf("CompileDirBootstrap() rejected const+var selectors: %v", err)
		}
	})

	t.Run("handwritten", func(t *testing.T) {
		dir := t.TempDir()
		bootstrapTestModule(t, dir)
		if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(varFixture), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := CompileDir(dir); err != nil {
			t.Fatalf("CompileDir() rejected const+var selectors: %v", err)
		}
	})
}
