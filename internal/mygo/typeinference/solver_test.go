package typeinference

import (
	"testing"
)

func TestNewSolver(t *testing.T) {
	solver := NewSolver()
	if solver == nil {
		t.Fatal("NewSolver returned nil")
	}
	if solver.instances == nil {
		t.Fatal("Solver.instances is nil")
	}
}

func TestRegisterInstance(t *testing.T) {
	solver := NewSolver()
	inst := &Instance{
		ClassName: "Eq",
		Type:      TCon{Name: "Int"},
	}
	solver.RegisterInstance(inst)

	instances := solver.instances["Eq"]
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}
	if instances[0].Type.String() != "Int" {
		t.Fatalf("expected instance type Int, got %s", instances[0].Type.String())
	}
}

func TestResolvePredicate(t *testing.T) {
	solver := NewSolver()
	builtInInstances := RegisterBuiltInInstances()
	for _, inst := range builtInInstances {
		solver.RegisterInstance(inst)
	}

	// Test resolving Eq[Int]
	pred := Predicate{
		ClassName: "Eq",
		Args:      []MonoType{TCon{Name: "Int"}},
	}

	resolved, err := solver.resolvePredicate(pred, make(Subst))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Eq[Int] should be resolved (no remaining predicates)
	if len(resolved) != 0 {
		t.Fatalf("expected 0 resolved predicates for Eq[Int], got %d", len(resolved))
	}
}

func TestResolveUnresolvedPredicate(t *testing.T) {
	solver := NewSolver()
	// Don't register any instances

	// Test resolving Eq[CustomType]
	pred := Predicate{
		ClassName: "Eq",
		Args:      []MonoType{TCon{Name: "CustomType"}},
	}

	resolved, err := solver.resolvePredicate(pred, make(Subst))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Eq[CustomType] should remain unresolved
	if len(resolved) != 1 {
		t.Fatalf("expected 1 unresolved predicate, got %d", len(resolved))
	}
	if resolved[0].ClassName != "Eq" {
		t.Fatalf("expected class name Eq, got %s", resolved[0].ClassName)
	}
}

func TestResolveMultiplePredicates(t *testing.T) {
	solver := NewSolver()
	builtInInstances := RegisterBuiltInInstances()
	for _, inst := range builtInInstances {
		solver.RegisterInstance(inst)
	}

	preds := []Predicate{
		{ClassName: "Eq", Args: []MonoType{TCon{Name: "Int"}}},
		{ClassName: "Eq", Args: []MonoType{TCon{Name: "String"}}},
	}

	resolved, err := solver.Resolve(preds, make(Subst))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All predicates should be resolved (no unresolved predicates)
	if len(resolved) != 0 {
		t.Fatalf("expected 0 unresolved predicates, got %d", len(resolved))
	}
}

func TestSolveConstraints(t *testing.T) {
	solver := NewSolver()
	builtInInstances := RegisterBuiltInInstances()
	for _, inst := range builtInInstances {
		solver.RegisterInstance(inst)
	}

	preds := []Predicate{
		{ClassName: "Eq", Args: []MonoType{TCon{Name: "Int"}}},
		{ClassName: "ToString", Args: []MonoType{TCon{Name: "String"}}},
	}

	remaining, subst, err := SolveConstraints(preds, make(TypeEnv), solver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All constraints should be solved
	if len(remaining) != 0 {
		t.Fatalf("expected 0 remaining constraints, got %d", len(remaining))
	}

	if subst == nil {
		t.Fatal("expected non-nil substitution")
	}
}

func TestDefaultNumericTypes(t *testing.T) {
	// Test defaulting a type variable to Int
	tv := TVar{ID: 1}
	defaulted := DefaultNumericTypes(tv, make(Subst))

	if _, ok := defaulted.(TCon); !ok || defaulted.(TCon).Name != "Int" {
		t.Fatalf("expected Int, got %v", defaulted)
	}
}

func TestHasTypeVariables(t *testing.T) {
	// Test with type variable
	tv := TVar{ID: 1}
	if !hasTypeVariables(tv) {
		t.Fatal("expected hasTypeVariables to return true for TVar")
	}

	// Test with concrete type
	con := TCon{Name: "Int"}
	if hasTypeVariables(con) {
		t.Fatal("expected hasTypeVariables to return false for TCon{Name: Int}")
	}

	// Test with parameterized type containing type variable
	paramType := TCon{Name: "Option", Args: []MonoType{TVar{ID: 2}}}
	if !hasTypeVariables(paramType) {
		t.Fatal("expected hasTypeVariables to return true for parameterized type with TVar")
	}
}

func TestIsFullyResolved(t *testing.T) {
	subst := make(Subst)

	// Test with concrete types
	concreteTypes := []MonoType{TCon{Name: "Int"}, TCon{Name: "String"}}
	if !isFullyResolved(concreteTypes, subst) {
		t.Fatal("expected isFullyResolved to return true for concrete types")
	}

	// Test with type variables
	varTypes := []MonoType{TVar{ID: 1}}
	if isFullyResolved(varTypes, subst) {
		t.Fatal("expected isFullyResolved to return false for type variables")
	}
}

func TestGetUniqueClassNames(t *testing.T) {
	preds := []Predicate{
		{ClassName: "Eq", Args: []MonoType{TCon{Name: "Int"}}},
		{ClassName: "ToString", Args: []MonoType{TCon{Name: "Int"}}},
		{ClassName: "Eq", Args: []MonoType{TCon{Name: "String"}}}, // duplicate
	}

	names := GetUniqueClassNames(preds)
	if len(names) != 2 {
		t.Fatalf("expected 2 unique class names, got %d", len(names))
	}
	if names[0] != "Eq" || names[1] != "ToString" {
		t.Fatalf("expected [Eq, ToString], got %v", names)
	}
}

func TestRegisterBuiltInInstances(t *testing.T) {
	instances := RegisterBuiltInInstances()

	// Check that Eq instances are registered (these are the only built-in instances)
	eqKey := "Eq[Int]"
	if _, ok := instances[eqKey]; !ok {
		t.Fatalf("expected Eq[Int] instance to be registered")
	}

	// ToString and IEnumerable are NOT built-in — they should only be registered from impl declarations
	toStringKey := "ToString[String]"
	if _, ok := instances[toStringKey]; ok {
		t.Fatalf("expected ToString[String] instance NOT to be registered as built-in")
	}
}
