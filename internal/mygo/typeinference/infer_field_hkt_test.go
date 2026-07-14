package typeinference

import (
	"fmt"
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

func TestInferFieldHKTDebug(t *testing.T) {
	// Simulate IAssignable[C[A], K, A] interface
	iface := &InterfaceDecl{
		Name:       "IAssignable",
		TypeParams: []string{"C[A]", "K", "A"},
		Methods: []*FuncDecl{
			{
				Name: "Get",
				Params: []Param{
					{Name: "c", Type: &NamedType{Name: "C", Args: []TypeExpr{&NamedType{Name: "A"}}}},
					{Name: "index", Type: &NamedType{Name: "K"}},
				},
				Ret: &NamedType{Name: "Option", Args: []TypeExpr{&NamedType{Name: "A"}}},
			},
		},
	}

	// Simulate what inferField does
	typeArgs := make([]MonoType, len(iface.TypeParams))
	for i := range iface.TypeParams {
		typeArgs[i] = TVar{ID: i + 1}
	}

	fmt.Printf("typeArgs: %v\n", typeArgs)

	for _, m := range iface.Methods {
		for _, p := range m.Params {
			astType := p.Type
			monoType := typeFromAST(astType)
			substituted := substituteTypeParams(monoType, iface.TypeParams, typeArgs)
			fmt.Printf("Param %s: AST=%v, Mono=%s, Substituted=%s\n", p.Name, astType, monoType, substituted)
		}
	}
}
