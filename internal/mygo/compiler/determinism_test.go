package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBootstrapGeneratedFilesAreDeterministic(t *testing.T) {
	dir := prepareIR3BenchPackage(t, ir3BenchOptionListSource)
	if _, err := SyncBootstrap(dir); err != nil {
		t.Fatalf("first SyncBootstrap() error = %v", err)
	}
	first := readGeneratedFiles(t, dir)
	if _, err := SyncBootstrap(dir); err != nil {
		t.Fatalf("second SyncBootstrap() error = %v", err)
	}
	second := readGeneratedFiles(t, dir)
	if len(first) != len(second) {
		t.Fatalf("generated file count changed: first %d, second %d", len(first), len(second))
	}
	for name, content := range first {
		other, ok := second[name]
		if !ok {
			t.Fatalf("second build omitted %s", name)
		}
		if content != other {
			t.Fatalf("generated file %s is not deterministic", name)
		}
	}
}

func readGeneratedFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "zz_*.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]string, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(path)] = string(data)
	}
	return out
}
