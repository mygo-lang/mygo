package parser2

import (
	"testing"
)

func TestStructLitSimple(t *testing.T) {
	got := ParseFile(`package sample

func foo()
  Point { x: 1 }
end
`)
	t.Logf("result = %+v", got)
}
