package mygo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileDirSupportsLetVarAndDiscard(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `module Main
  import fmt "go:fmt"

  func add(x: Int, y: Int): Int
    x + y
  end

  func demo(): Int
    let msg: String = "abc"
    let _ = fmt.Println(msg)
    var n: Int = add(40, 2)
    n = n + 1
    n
  end

  func main(): ()
    demo()
  end
end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	if out != filepath.Join(dir, "zz_mygo.gen.go") {
		t.Fatalf("CompileDir() output path = %q, want %q", out, filepath.Join(dir, "zz_mygo.gen.go"))
	}
	got := readFile(t, out)
	for _, want := range []string{
		"func demo() int {",
		"var msg_1 string = \"abc\"",
		"fmt.Println(msg_1)",
		"var n_2 int = add(40, 2)",
		"n_2 = (n_2 + 1)",
		"return n_2",
		"func main() {",
		"demo()",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Go missing %q\n--- got ---\n%s", want, got)
		}
	}
}

func TestCompileDirAllowsLetShadowingAndInference(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `module Main
  func demo(): Int
    let x = 1
    let x = 2
    x
  end
end
`)

	out, err := CompileDir(dir)
	if err != nil {
		t.Fatalf("CompileDir() error = %v", err)
	}
	got := readFile(t, out)
	if !strings.Contains(got, "return x_2") {
		t.Fatalf("generated Go missing shadowed return\n--- got ---\n%s", got)
	}
	if !strings.Contains(got, "x_1 := 1") || !strings.Contains(got, "x_2 := 2") {
		t.Fatalf("generated Go missing shadowed bindings\n--- got ---\n%s", got)
	}
}

func TestCompileDirRejectsAssignmentToLet(t *testing.T) {
	dir := t.TempDir()
	writeMygoFile(t, dir, "main.mygo", `module Main
  func bad(): Int
    let x: Int = 1
    x = 2
    x
  end
end
`)

	_, err := CompileDir(dir)
	if err == nil {
		t.Fatal("CompileDir() error = nil, want immutable binding failure")
	}
	if !strings.Contains(err.Error(), "immutable binding") {
		t.Fatalf("CompileDir() error = %v, want immutable binding failure", err)
	}
}

func writeMygoFile(t *testing.T, dir, name, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(data)
}
