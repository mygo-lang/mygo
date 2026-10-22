package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDiagnosticsUseGeneratedLineDirectives(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":    "module example.invalid/line-directive\n\ngo 1.24\n",
		"main.mygo": "package main\n\nfunc broken() -> Int\n  go[Int] { code: \"\\\"wrong\\\"\" }\nend\n\nfunc main() -> () => ()\n",
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	if _, err := SyncBootstrapNoPrelude(dir); err != nil {
		t.Fatalf("bootstrap sync: %v", err)
	}
	generated, err := os.ReadFile(filepath.Join(dir, "zz_main.gen.go"))
	if err != nil {
		t.Fatalf("read generated source: %v", err)
	}
	if !strings.Contains(string(generated), "/*line main.mygo:4:") {
		t.Fatalf("generated source lacks source line directive:\n%s", generated)
	}
	cmd := exec.Command("go", "build", "-buildvcs=false", ".")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	output, buildErr := cmd.CombinedOutput()
	if buildErr == nil {
		t.Fatal("invalid generated Go compiled successfully")
	}
	message := string(output)
	if !strings.Contains(message, "main.mygo:4:") {
		t.Fatalf("build diagnostic did not use source line directive:\n%s", message)
	}
	if strings.Contains(message, "zz_main.gen.go:") {
		t.Fatalf("build diagnostic retained a generated-file location:\n%s", message)
	}
}
