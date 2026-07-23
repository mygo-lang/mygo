package compiler

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileDirBootstrapUsesSelfHostedPipeline(t *testing.T) {
	dir := t.TempDir()
	source := `package sample

enum Maybe[A]
  Some(A)
  None
end

func unwrap(value: Maybe[Int]) -> Int
  switch value
    case Some(item) => item
    case None => 0
  end
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(dir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	if len(written) != 1 {
		t.Fatalf("CompileDirBootstrap() wrote %d files, want 1", len(written))
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), written[0], generated, parser.AllErrors); err != nil {
		t.Fatalf("bootstrap output is invalid Go: %v\n%s", err, generated)
	}
}

func TestCompileDirBootstrapSupportsGoFFI(t *testing.T) {
	dir := t.TempDir()
	source := `package sample

import fmt "go:fmt"

func Render(value: Int) -> String
  fmt.Sprint(value)
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(dir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), `import "fmt"`) || !strings.Contains(string(generated), "fmt.Sprint") {
		t.Fatalf("generated Go did not preserve fmt FFI:\n%s", generated)
	}
}

func TestCompileDirBootstrapRejectsUnknownGoFFISelector(t *testing.T) {
	dir := t.TempDir()
	source := `package sample

import fmt "go:fmt"

func Render(value: Int) -> String
  fmt.NotAFunction(value)
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := CompileDirBootstrap(dir)
	if err == nil || !strings.Contains(err.Error(), "unknown Go package member NotAFunction") {
		t.Fatalf("CompileDirBootstrap() error = %v, want unknown Go package member", err)
	}
}

func TestCompileDirBootstrapRejectsWrongFixedGoFFIArity(t *testing.T) {
	dir := t.TempDir()
	source := `package sample

import strconv "go:strconv"

func Render() -> String
  strconv.Itoa()
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := CompileDirBootstrap(dir)
	if err == nil || !strings.Contains(err.Error(), "Go function argument count mismatch") {
		t.Fatalf("CompileDirBootstrap() error = %v, want Go function argument count mismatch", err)
	}
}

func TestCompileDirBootstrapDecodesGoValueBoolAsOption(t *testing.T) {
	dir := t.TempDir()
	source := `package sample

import strings "go:strings"

func TrimPrefix(value: String) -> Option[String]
  strings.CutPrefix(value, "pre")
end
`
	if err := os.WriteFile(filepath.Join(dir, "sample.mygo"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(dir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	generated, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"strings.CutPrefix", "Some[string]", "None[string]"} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("generated Go missing %q:\n%s", want, generated)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), written[0], generated, parser.AllErrors); err != nil {
		t.Fatalf("bootstrap output is invalid Go: %v\n%s", err, generated)
	}
}

func TestCompileDirBootstrapCompilesMyGOImports(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libDir, "lib.mygo"), []byte(`package lib

func Add(left: Int, right: Int) -> Int
  left + right
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.mygo"), []byte(`package app

import lib "../lib"

func Run() -> Int
  lib.Add(1, 2)
end
`), 0o644); err != nil {
		t.Fatal(err)
	}
	written, err := CompileDirBootstrap(appDir)
	if err != nil {
		t.Fatalf("CompileDirBootstrap() error = %v", err)
	}
	if len(written) != 2 {
		t.Fatalf("CompileDirBootstrap() wrote %d files, want app and dependency", len(written))
	}
	for _, path := range written {
		generated, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), path, generated, parser.AllErrors); err != nil {
			t.Fatalf("bootstrap output %s is invalid Go: %v\n%s", path, err, generated)
		}
	}
}
