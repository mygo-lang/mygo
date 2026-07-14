package typeinference

import (
	"fmt"
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

func TestInferFieldCallHKT(t *testing.T) {
	// Simulate the full flow: inferField returns method type, then inferCall unifies it

	// IAssignable[C[A], K, A] interface
	iface := &InterfaceDecl{
		Name:       "IAssignable",
		TypeParams: []string{"C[A]", "K", "A"},
		Methods: []*FuncDecl{{
			Name: "Get",
			Params: []Param{
				{Name: "c", Type: &NamedType{Name: "C", Args: []TypeExpr{&NamedType{Name: "A"}}}},
				{Name: "index", Type: &NamedType{Name: "K"}},
			},
			Ret: &NamedType{Name: "Option", Args: []TypeExpr{&NamedType{Name: "A"}}},
		}},
	}

	// Simulate inferField: create fresh type args and substitute
	typeArgs := make([]MonoType, len(iface.TypeParams))
	for i := range iface.TypeParams {
		typeArgs[i] = TVar{ID: i + 1}
	}

	// Build method type for Get
	var paramTypes []MonoType
	for _, p := range iface.Methods[0].Params {
		paramTypes = append(paramTypes, substituteTypeParams(typeFromAST(p.Type), iface.TypeParams, typeArgs))
	}
	ret := substituteTypeParams(typeFromAST(iface.Methods[0].Ret), iface.TypeParams, typeArgs)

	methodType := TFunc{Args: paramTypes, Ret: ret}
	fmt.Printf("Method type from inferField: %s\n", methodType)

	// Simulate inferCall: receiverType = Slice[String], argType = Int
	receiverType := TCon{Name: "Slice", Args: []MonoType{TCon{Name: "String"}}}
	argTypes := []MonoType{TCon{Name: "Int"}}

	retVar := TVar{ID: 100}
	funcType := TFunc{Args: append([]MonoType{receiverType}, argTypes...), Ret: retVar}
	fmt.Printf("Expected func type: %s\n", funcType)

	// Unify calleeType (methodType) with funcType
	subst, err := Unify(methodType, funcType, make(Subst))
	if err != nil {
		t.Errorf("Unification failed: %v", err)
		return
	}

	fmt.Printf("Substitution: %v\n", subst)
	fmt.Printf("Return type after substitution: %s\n", subst.ApplyMT(retVar))

	// Check that the substitution correctly bound t1 to Slice[String]
	if slice, ok := subst[1]; !ok {
		t.Errorf("Expected t1 to be bound, got nil")
	} else if !eqType(slice, TCon{Name: "Slice", Args: []MonoType{TCon{Name: "String"}}}) {
		t.Errorf("Expected t1 = Slice[String], got %s", slice)
	}
}
