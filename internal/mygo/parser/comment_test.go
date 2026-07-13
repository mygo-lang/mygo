package parser

import (
	"testing"
)

func TestParseWithHashComment(t *testing.T) {
	src := `package main
# this is a comment
func demo() -> Int
  42
end
`
	file, err := ParseFile("test.mygo", src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if file.PackageName != "main" {
		t.Fatalf("PackageName = %q, want %q", file.PackageName, "main")
	}
	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(file.Decls))
	}
}

func TestParseWithHashCommentInFunction(t *testing.T) {
	src := `package main
func demo() -> Int
  # comment inside function
  let x = 42
  x
end
`
	_, err := ParseFile("test.mygo", src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
}

func TestParseWithHashCommentInline(t *testing.T) {
	src := `package main
func demo() -> Int
  # inline comment
  let x = 42
  x
end
`
	_, err := ParseFile("test.mygo", src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
}

func TestParseMultipleHashComments(t *testing.T) {
	src := `package main
# comment 1
# comment 2
# comment 3
func demo() -> Int
  # inline comment
  let a = 1
  # another
  let b = 2
  a + b
end
`
	_, err := ParseFile("test.mygo", src)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
}
