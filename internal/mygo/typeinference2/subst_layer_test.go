package typeinference2

import (
	"testing"

	"github.com/mygo-lang/mygo/internal/mygo/ast2"
)

func TestComposeSubstFlattensLookupLayers(t *testing.T) {
	older := SubstFromEntries([]SubstEntry{{ID: 1, Type: ast2.MonoType__TVar__Ctor(2)}})
	newer := SubstFromEntries([]SubstEntry{{ID: 2, Type: ast2.MonoType__TCon__Ctor("Int")}})

	composed := composeSubst(newer, older)
	if len(composed.Entries) != 2 {
		t.Fatalf("composed Entries = %v, want two ordered entries", composed.Entries)
	}
	if got := applySubst(composed, ast2.MonoType__TVar__Ctor(1)); !ast2.MonoEqual(got, ast2.MonoType__TCon__Ctor("Int")) {
		t.Fatalf("composed substitution resolved to %v, want Int", got)
	}

	updated := substPrepend(composed, SubstEntry{ID: 3, Type: ast2.MonoType__TCon__Ctor("String")})
	if len(updated.Entries) != 1 {
		t.Fatalf("updated Entries length = %d, want one local entry", len(updated.Entries))
	}
	if got := maxSubstID(updated); got != 3 {
		t.Fatalf("maxSubstID = %d, want 3", got)
	}
}
