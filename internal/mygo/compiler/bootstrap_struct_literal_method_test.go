package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// structLiteralMethodFixture calls a method directly on a struct literal both
// on an assignment right-hand side and inside an argument list.  Both positions
// used to fail: the literal ended the expression, so the call was either dropped
// or reported as an unrelated identifier error.
const structLiteralMethodFixture = `package main

import fmt "go:fmt"

struct Point
  x: Int
  y: Int
end

impl Point
  func Sum(self: Point) -> Int
    self.x + self.y
  end
end

func main() -> ()
  let s = Point { x: 3, y: 4 }.Sum()
  let _ = fmt.Println(s)
  let _ = fmt.Println(Point { x: 1, y: 2 }.Sum())
end
`

func TestBootstrapStructLiteralMethodCallCodegen(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/structlit\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte(structLiteralMethodFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncBootstrapNoPrelude(dir); err != nil {
		t.Fatalf("SyncBootstrapNoPrelude() error = %v", err)
	}
	generated, err := os.ReadFile(filepath.Join(dir, "zz_main.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(generated), "M3Sum("); count < 2 {
		t.Fatalf("generated Go calls the method %d times, want 2:\n%s", count, generated)
	}
	if !strings.Contains(string(generated), "M3Sum(Point{") {
		t.Fatalf("generated Go never passes the struct literal to the method:\n%s", generated)
	}

	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/mygo-structlit-gocache")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running the generated program failed: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "7\n3" {
		t.Fatalf("program output = %q, want %q", got, "7\n3")
	}
}
