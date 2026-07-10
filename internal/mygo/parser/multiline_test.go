package parser_test

import (
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/parser"
)

func TestParseFileMultilineFuncDecl(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "cross_line_params",
			src: `package main
func add(
  x: Int,
  y: Int
) -> Int
  x + y
end
`,
		},
		{
			name: "cross_line_return",
			src: `package main
func add(x: Int, y: Int)
  -> Int
  x + y
end
`,
		},
		{
			name: "cross_line_using",
			src: `package main
func foo(x: Int) -> String
  using Show
  show(x)
end
`,
		},
		{
			name: "cross_line_param_list",
			src: `package main
func add(x: Int,
  y: Int) -> Int
  x + y
end
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.ParseFile(tt.src)
			if err != nil {
				t.Fatalf("expected success, got: %v", err)
			}
		})
	}
}
