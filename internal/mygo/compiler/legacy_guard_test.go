package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacySyncRejectsCaseGuards(t *testing.T) {
	dir := t.TempDir()
	source := `package sample

func choose(value: Int, flag: Bool) -> Int
  switch value
    case 0 if flag => 1
    case _ => 0
  end
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Sync(dir)
	if err == nil || !strings.Contains(err.Error(), "case guards are not supported by the legacy pipeline") {
		t.Fatalf("Sync() error = %v, want legacy guard diagnostic", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "zz_sample.gen.go")); !os.IsNotExist(statErr) {
		t.Fatalf("legacy sync wrote output despite rejecting guards: %v", statErr)
	}
}
