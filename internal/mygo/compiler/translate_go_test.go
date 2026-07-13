package compiler

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/codegen"
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

func TestInlineGoGeneratesPrimitiveConversion(t *testing.T) {
	src := `package main
func fromRunes(rs: Slice[rune]) -> String
  go[String] {
    code: """string({rs})"""
    in rs = rs
  }
end
`
	goSrc := compileInlineGoTestPackage(t, src)
	if !strings.Contains(goSrc, "return string(rs)") {
		t.Fatalf("generated source missing primitive conversion:\n%s", goSrc)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "inline.go", goSrc, 0); err != nil {
		t.Fatalf("generated source is not valid Go: %v\n%s", err, goSrc)
	}
}

func TestInherentStaticMethodCallOnType(t *testing.T) {
	src := `package main
import utf8 "go:unicode/utf8"

impl String
  func FromRunes(rs: Slice[rune]) -> String
    go[String] {
      code: ` + "`string(rs)`" + `
      in rs = rs
    }
  end

  func PeekRune(s: String) -> Option[Ref[rune]]
    if s.Len() == 0 then
      None
    else
      let (r, _) = utf8.DecodeRuneInString(s)
      Some(Ref.new(r))
    end
  end
end

func demo(rs: Slice[rune]) -> String
  String.FromRunes(rs)
end
`
	goSrc := compileInlineGoTestPackage(t, src)
	if !strings.Contains(goSrc, "func String_FromRunes(rs []rune) string") {
		t.Fatalf("generated source missing static inherent helper:\n%s", goSrc)
	}
	if !strings.Contains(goSrc, "return String_FromRunes(rs)") {
		t.Fatalf("generated source missing static inherent call:\n%s", goSrc)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "static_inherent.go", goSrc, 0); err != nil {
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
	_, err = codegen.Generate(pkg)
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
  let _ = go[()] {
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

func TestInlineGoUnitAcceptsGoStatement(t *testing.T) {
	src := `package main
func spawn(fn: func() -> ()) -> ()
  go[()] {
    code: "go fn()"
  }
end
`
	goSrc := compileInlineGoTestPackage(t, src)
	if !strings.Contains(goSrc, "go fn()") {
		t.Fatalf("generated source missing go statement:\n%s", goSrc)
	}
	if strings.Contains(goSrc, "func() {\n\t\tgo fn()\n\t}()") {
		t.Fatalf("generated source wrapped go statement in IIFE:\n%s", goSrc)
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
	goSrc, err := codegen.Generate(pkg)
	if err != nil {
		t.Fatal(err)
	}
	return goSrc
}

func TestInlineGoTypeOperandProducesValidGo(t *testing.T) {
	src := `package main
func demo(n: Int) -> String
  go[String] {
    code: "{T}({v})"
    in v = n
    type T = String
  }
end
`
	goSrc := compileInlineGoTestPackage(t, src)
	if !strings.Contains(goSrc, "string(n)") {
		t.Fatalf("generated source missing type-substituted expression:\n%s", goSrc)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "inline.go", goSrc, 0); err != nil {
		t.Fatalf("generated source is not valid Go: %v\n%s", err, goSrc)
	}
}

func TestInlineGoMixedOperands(t *testing.T) {
	src := `package main
func cast(n: Int) -> String
  go[String] {
    code: "{T}({v})"
    in v = n
    type T = String
  }
end
`
	goSrc := compileInlineGoTestPackage(t, src)
	if !strings.Contains(goSrc, "string(n)") {
		t.Fatalf("generated source missing mixed operand substitution:\n%s", goSrc)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "inline.go", goSrc, 0); err != nil {
		t.Fatalf("generated source is not valid Go: %v\n%s", err, goSrc)
	}
}

func TestInlineGoOptionAutoWraps(t *testing.T) {
	src := `package main
func maybe(n: Int) -> Option[Ref[Int]]
  go[Option[Ref[Int]]] {
    code: "func() *int { return &{x} }()"
    in x = n
  }
end
`
	goSrc := compileInlineGoTestPackage(t, src)
	if !strings.Contains(goSrc, "Some[*int]") {
		t.Fatalf("generated source missing Option wrapping:\n%s", goSrc)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "inline.go", goSrc, 0); err != nil {
		t.Fatalf("generated source is not valid Go: %v\n%s", err, goSrc)
	}
}

func TestInlineGoRefAutoWrapsAsOption(t *testing.T) {
	src := `package main
func maybe(ok: Bool, n: Int) -> Option[Ref[Int]]
  go[Ref[Int]] {
    code: "func() *int { if !{ok} { return nil }; return &{x} }()"
    in ok = ok
    in x = n
  }
end
`
	goSrc := compileInlineGoTestPackage(t, src)
	for _, want := range []string{
		"__mygo_go_ref := func() *int",
		"if __mygo_go_ref == nil",
		"return None[*int]()",
		"return Some[*int](__mygo_go_ref)",
	} {
		if !strings.Contains(goSrc, want) {
			t.Fatalf("generated source missing %q:\n%s", want, goSrc)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "inline.go", goSrc, 0); err != nil {
		t.Fatalf("generated source is not valid Go: %v\n%s", err, goSrc)
	}
}

func TestInlineGoResultAutoWraps(t *testing.T) {
	src := `package main
func writeMessage() -> Result[(), String]
  go[Result[(), String]] {
    code: "func() error { return nil }()"
  }
end
`
	goSrc := compileInlineGoTestPackage(t, src)
	if !strings.Contains(goSrc, "Ok[struct{}, string]") {
		t.Fatalf("generated source missing Result wrapping:\n%s", goSrc)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "inline.go", goSrc, 0); err != nil {
		t.Fatalf("generated source is not valid Go: %v\n%s", err, goSrc)
	}
}

func TestInlineGoChanTypeOperandProducesValidGo(t *testing.T) {
	src := `package main
func demo() -> Chan[Int]
  go[Chan[Int]] {
    code: "make(chan {T})"
    type T = Int
  }
end
`
	goSrc := compileInlineGoTestPackage(t, src)
	if !strings.Contains(goSrc, "chan int") {
		t.Fatalf("generated source missing chan type:\n%s", goSrc)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "inline.go", goSrc, 0); err != nil {
		t.Fatalf("generated source is not valid Go: %v\n%s", err, goSrc)
	}
}

func TestInlineGoDirectionalChanTypesProduceValidGo(t *testing.T) {
	src := `package main
func demo(ch: Chan[Int]) -> Int
  let sender: SendChan[Int] = go[SendChan[Int]] {
    code: "chan<- int({ch})"
    in ch = ch
  }
  let receiver: RecvChan[Int] = go[RecvChan[Int]] {
    code: "<-chan int({ch})"
    in ch = ch
  }
  0
end
`
	goSrc := compileInlineGoTestPackage(t, src)
	for _, want := range []string{"chan<- int", "<-chan int"} {
		if !strings.Contains(goSrc, want) {
			t.Fatalf("generated source missing %q:\n%s", want, goSrc)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "inline.go", goSrc, 0); err != nil {
		t.Fatalf("generated source is not valid Go: %v\n%s", err, goSrc)
	}
}

func writeInlineGoTestPackage(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.mygo"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
