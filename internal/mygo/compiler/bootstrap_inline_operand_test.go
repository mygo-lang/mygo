package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapInlineGoOperandInference(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "go.mod", "module example.invalid/inline-operand\n\ngo 1.24\n")
	writeFixture(t, dir, "main.mygo", `package main

import fmt "go:fmt"

func main() -> ()
  let result = go[Int] {
    code: "{callback}(3)"
    in callback = func(x: Int) -> Int x + 4 end
  }
  let _ = fmt.Println(result)
end
`)
	if _, err := SyncBootstrapNoPrelude(dir); err != nil {
		t.Fatalf("sync: %v", err)
	}
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-buildvcs=false")
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "7" {
		t.Fatalf("go run: %v, output %s", err, out)
	}
}

func TestBootstrapInlineGoOperandRejectsTypeError(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "go.mod", "module example.invalid/inline-operand-error\n\ngo 1.24\n")
	writeFixture(t, dir, "main.mygo", `package main
func main() -> ()
  let _ = go[Int] {
    code: "{callback}(3)"
    in callback = func(x: Int) -> Int x + true end
  }
end
`)
	if _, err := SyncBootstrapNoPrelude(dir); err == nil {
		t.Fatal("expected an error for the invalid callback body")
	}
}

func TestBootstrapInlineGoCallbackCapturesConstraint(t *testing.T) {
	dir := t.TempDir()
	bootstrapTestModule(t, dir)
	writeFixture(t, dir, "main.mygo", `package sample
func equal[T](a: T, b: T) -> Bool using Eq[T]
  go[Bool] {
    code: "{equal}({a}, {b})"
    in a = a
    in b = b
    in equal = func(left: T, right: T) -> Bool left.Equals(right) end
  }
end
`)
	if _, err := SyncBootstrap(dir); err != nil {
		t.Fatalf("sync: %v", err)
	}
	out, err := os.ReadFile(filepath.Join(dir, "zz_main.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "left.Equals(") || !strings.Contains(string(out), "EqualsFn(left, right)") {
		t.Fatalf("callback did not capture its Eq dictionary:\n%s", out)
	}
}

func TestCodegen3InlineGoCallbackCapturesConstraint(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "go.mod", "module example.invalid/callback-codegen3\n\ngo 1.24\n")
	writeFixture(t, dir, "main.mygo", `package main
interface Eq[A]
  func Equals(left: A, right: A) -> Bool
end
func equal[T](a: T, b: T) -> Bool using Eq[T]
  go[Bool] {
    code: "{equal}({a}, {b})"
    in a = a
    in b = b
    in equal = func(left: T, right: T) -> Bool left.Equals(right) end
  }
end
`)
	writeFixture(t, dir, "main.go", `package main
import "fmt"
func main() { fmt.Println(equal(2, 2, func(a, b int) bool { return a == b })) }
`)
	binary := filepath.Join(dir, "app")
	if err := BuildCodegen3(dir, PipelineOptions{Codegen3: true, NoPrelude: true}, false, []string{"-o", binary}); err != nil {
		t.Fatalf("codegen3 build: %v", err)
	}
	out, err := exec.Command(binary).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		t.Fatalf("run: %v, output %s", err, out)
	}
}

func TestCodegen3MultipleConstraintsUseTheirOwnDictionaries(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "go.mod", "module example.invalid/multiple-dictionaries\n\ngo 1.24\n")
	writeFixture(t, dir, "main.mygo", `package main
interface Eq[A]
  func Equals(left: A, right: A) -> Bool
end
func both[A, E](a: A, b: A, e: E, f: E) -> Bool using Eq[A], Eq[E]
  a.Equals(b) && e.Equals(f)
end
`)
	writeFixture(t, dir, "main.go", `package main
import "fmt"
func main() {
  fmt.Println(both(1, 1, "x", "x", func(a, b int) bool { return a == b }, func(a, b string) bool { return a == b }))
  fmt.Println(both(1, 1, "x", "y", func(a, b int) bool { return a == b }, func(a, b string) bool { return a == b }))
}
`)
	binary := filepath.Join(dir, "app")
	if err := BuildCodegen3(dir, PipelineOptions{Codegen3: true, NoPrelude: true}, false, []string{"-o", binary}); err != nil {
		t.Fatalf("codegen3 build: %v", err)
	}
	out, err := exec.Command(binary).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "true\nfalse" {
		t.Fatalf("run: %v, output %s", err, out)
	}
}
