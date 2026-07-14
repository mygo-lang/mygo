package typeinference

import (
	"fmt"
	"testing"
)

func TestParseHKTDebug(t *testing.T) {
	name, inner := parseHKTTypeParam("C[A]")
	fmt.Printf("parseHKTTypeParam('C[A]') = ('%s', '%s')\n", name, inner)
	if name != "C" || inner != "A" {
		t.Errorf("Expected ('C', 'A'), got ('%s', '%s')", name, inner)
	}

	name2, inner2 := parseHKTTypeParam("K")
	fmt.Printf("parseHKTTypeParam('K') = ('%s', '%s')\n", name2, inner2)
	if name2 != "" || inner2 != "" {
		t.Errorf("Expected ('', ''), got ('%s', '%s')", name2, inner2)
	}
}

func TestSubstituteTypeParamsDebug(t *testing.T) {
	// Simulate IAssignable[C[A], K, A]
	typeParams := []string{"C[A]", "K", "A"}
	typeArgs := []MonoType{
		TCon{Name: "Slice", Args: []MonoType{TCon{Name: "String"}}},
		TCon{Name: "Int"},
		TCon{Name: "String"},
	}

	// Test: TCon{Name: "C", Args: [TCon{Name: "A"}]} should be replaced by Slice[String]
	// This is what typeFromAST produces for "C[A]" in the interface method param
	t1 := TCon{Name: "C", Args: []MonoType{TCon{Name: "A"}}}
	result1 := substituteTypeParams(t1, typeParams, typeArgs)
	expected1 := TCon{Name: "Slice", Args: []MonoType{TCon{Name: "String"}}}
	fmt.Printf("Input: %s\n", t1)
	fmt.Printf("Result: %s\n", result1)
	fmt.Printf("Expected: %s\n", expected1)
	if !eqType(result1, expected1) {
		t.Errorf("HKT substitution failed: got %s, want %s", result1, expected1)
	}

	// Test: TCon{Name: "K"} should be replaced by Int
	t2 := TCon{Name: "K"}
	result2 := substituteTypeParams(t2, typeParams, typeArgs)
	expected2 := TCon{Name: "Int"}
	fmt.Printf("\nInput: %s\n", t2)
	fmt.Printf("Result: %s\n", result2)
	fmt.Printf("Expected: %s\n", expected2)
	if !eqType(result2, expected2) {
		t.Errorf("Simple param substitution failed: got %s, want %s", result2, expected2)
	}
}
