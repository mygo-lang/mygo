package compiler

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInlineGoGeneratesValidGoExpr(t *testing.T) {
	src := `package main
func addOne(n: Int) -> Int
  go[Int] {
    code: "{x} + 1"
    in x = n
  }
end
`
	goSrc := compileInlineGoTestPackage(t, src)
	if !strings.Contains(goSrc, "return n + 1") {
		t.Fatalf("generated source missing substituted expression:\n%s", goSrc)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "inline.go", goSrc, 0); err != nil {
		t.Fatalf("generated source is not valid Go: %v\n%s", err, goSrc)
	}
}

func TestInlineGoUnknownPlaceholderErrors(t *testing.T) {
	src := `package main
func bad(n: Int) -> Int
  go[Int] {
    code: "{missing} + 1"
    in x = n
  }
end
`
	dir := writeInlineGoTestPackage(t, src)
	pkg, err := loadPackage(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pkg.Generate()
	if err == nil {
		t.Fatal("Generate() error = nil, want unknown operand error")
	}
	if !strings.Contains(err.Error(), `unknown operand "missing"`) {
		t.Fatalf("Generate() error = %v, want unknown operand", err)
	}
}

func TestInlineGoUnitGeneratesStatement(t *testing.T) {
	src := `package main
import fmt "go:fmt"
func printMessage(message: String) -> Int
  let _ = go[Unit] {
    code: "fmt.Println({msg})"
    in msg = message
  }
  0
end
`
	goSrc := compileInlineGoTestPackage(t, src)
	if !strings.Contains(goSrc, "fmt.Println(message)") {
		t.Fatalf("generated source missing unit statement:\n%s", goSrc)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "inline.go", goSrc, 0); err != nil {
		t.Fatalf("generated source is not valid Go: %v\n%s", err, goSrc)
	}
}

func compileInlineGoTestPackage(t *testing.T, src string) string {
	t.Helper()
	dir := writeInlineGoTestPackage(t, src)
	pkg, err := loadPackage(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	goSrc, err := pkg.Generate()
	if err != nil {
		t.Fatal(err)
	}
	return goSrc
}

func writeInlineGoTestPackage(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
