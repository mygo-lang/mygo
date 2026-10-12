package typeinference2

import (
	"github.com/mygo-lang/mygo/internal/mygo/ast2"
	. "github.com/mygo-lang/mygo/prelude"
	"testing"
)

func TestArrayTypeTraversal(t *testing.T) {
	variable := ast2.MonoType__TVar__Ctor(7)
	array := ast2.MonoType__TArray__Ctor(&variable, 5)
	integer := ast2.MonoType__TCon__Ctor("Int")
	want := ast2.MonoType__TArray__Ctor(&integer, 5)
	got := applySubst(SubstFromEntries([]SubstEntry{{ID: 7, Type: integer}}), array)
	if !ast2.MonoEqual(got, want) {
		t.Fatalf("substitution = %v", got)
	}
	if !occursIn(7, array) || len(freeVars(array, nil)) != 1 || nextFreshVarID(array, 1) != 8 {
		t.Fatal("array element variables were not traversed")
	}
	other := ast2.MonoType__TArray__Ctor(&integer, 3)
	if _, ok := unify(want, other, emptySubst()).(Result__Err[Subst, string]); !ok {
		t.Fatal("different array lengths unified")
	}
	if MonoStringFull(want) != "Int[5]" {
		t.Fatal(MonoStringFull(want))
	}
}

func TestGoArraySignature(t *testing.T) {
	for _, name := range []string{"[5]int32", "[3][5]int32", "[][5]int32", "[0]*int"} {
		if _, ok := GoTypeName(name).(Result__Ok[ast2.MonoType, string]); !ok {
			t.Fatalf("could not decode %s: %v", name, GoTypeName(name))
		}
	}
}
