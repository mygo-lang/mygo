package parser

import (
	"strings"
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast"
)

func TestBacktickString(t *testing.T) {
	// Use strings.Join to build the source with backticks
	bt := "`"
	src := "let a = \"hello\\nworld\"\n" +
		"let b = " + bt + "raw" + bt + "\n" +
		"let c = " + bt + "new\nline" + bt + "\n" +
		`let d = ` + bt + `he said "hi"` + bt

	file, err := ParseFile(src)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(file.Decls) != 4 {
		t.Fatalf("Expected 4 decls, got %d", len(file.Decls))
	}

	tests := []struct {
		name  string
		kind  string
		value string
	}{
		{"a", "string", "hello\nworld"},         // escaped newline -> literal newline
		{"b", "string", "raw"},                  // backtick: raw, no escapes
		{"c", "string", "new\nline"},           // backtick: preserves newlines
		{"d", "string", `he said "hi"`},       // backtick: preserves quotes
	}

	for i, tt := range tests {
		decl := file.Decls[i]
		letDecl, ok := decl.(*LetStmt)
		if !ok {
			t.Fatalf("Decl %d: not LetStmt", i)
		}
		if letDecl.Name != tt.name {
			t.Errorf("Decl %d: name = %q, want %q", i, letDecl.Name, tt.name)
		}
		lit, ok := letDecl.Value.(*ast.LiteralExpr)
		if !ok {
			t.Fatalf("Decl %d: value not LiteralExpr", i)
		}
		if lit.Kind != tt.kind {
			t.Errorf("Decl %d: kind = %q, want %q", i, lit.Kind, tt.kind)
		}
		if lit.Value != tt.value {
			t.Errorf("Decl %d: value = %q, want %q", i, lit.Value, tt.value)
		}
	}
	// Verify that strings has no side effect
	_ = strings.Builder{}
}
