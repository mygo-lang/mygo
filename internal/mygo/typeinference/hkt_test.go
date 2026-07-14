package typeinference

import (
	"testing"
)

func TestSubstituteTypeParamsHKT(t *testing.T) {
	// Simulate: interface IEnumerable[C[A], A]
	// typeParams = ["C[A]", "A"]
	// typeArgs = [Slice[String], String]
	typeParams := []string{"C[A]", "A"}
	typeArgs := []MonoType{
		TCon{Name: "Slice", Args: []MonoType{TCon{Name: "String"}}},
		TCon{Name: "String"},
	}

	// Test 1: TCon{Name: "C", Args: [TCon{Name: "A"}]} should be replaced by Slice[String]
	// This is what typeFromAST produces for "C" applied to "A" in method signatures.
	t1 := TCon{Name: "C", Args: []MonoType{TCon{Name: "A"}}}
	result1 := substituteTypeParams(t1, typeParams, typeArgs)
	expected1 := TCon{Name: "Slice", Args: []MonoType{TCon{Name: "String"}}}
	if !eqType(result1, expected1) {
		t.Errorf("HKT substitution failed: got %s, want %s", result1, expected1)
	}

	// Test 2: TCon{Name: "A"} (simple type param, no args) should be replaced by String
	t2 := TCon{Name: "A"}
	result2 := substituteTypeParams(t2, typeParams, typeArgs)
	expected2 := TCon{Name: "String"}
	if !eqType(result2, expected2) {
		t.Errorf("Simple param substitution failed: got %s, want %s", result2, expected2)
	}

	// Test 3: IAssignable[C[A], K, A] with typeParams = ["C[A]", "K", "A"]
	// typeArgs = [Slice[String], Int, String]
	typeParams3 := []string{"C[A]", "K", "A"}
	typeArgs3 := []MonoType{
		TCon{Name: "Slice", Args: []MonoType{TCon{Name: "String"}}},
		TCon{Name: "Int"},
		TCon{Name: "String"},
	}

	// TCon{Name: "C", Args: [TCon{Name: "A"}]} should be replaced by Slice[String]
	t3 := TCon{Name: "C", Args: []MonoType{TCon{Name: "A"}}}
	result3 := substituteTypeParams(t3, typeParams3, typeArgs3)
	if !eqType(result3, expected1) {
		t.Errorf("IAssignable HKT substitution failed: got %s, want %s", result3, expected1)
	}

	// TCon{Name: "K"} should be replaced by Int
	t4 := TCon{Name: "K"}
	result4 := substituteTypeParams(t4, typeParams3, typeArgs3)
	expected4 := TCon{Name: "Int"}
	if !eqType(result4, expected4) {
		t.Errorf("IAssignable K substitution failed: got %s, want %s", result4, expected4)
	}
}
