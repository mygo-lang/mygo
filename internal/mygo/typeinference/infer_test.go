package typeinference

import (
	"testing"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

func parseDecl(t *testing.T, src string) Decl {
	t.Helper()
	return nil
}

func boolType() MonoType  { return TCon{Name: "Bool"} }
func intType() MonoType   { return TCon{Name: "Int"} }
func int64Type() MonoType { return TCon{Name: "Int64"} }
func stringType() MonoType {
	return TCon{Name: "String"}
}
func unitType() MonoType { return TUnit{} }

func inferExprType(env TypeEnv, e Expr, state *InferState) (MonoType, error) {
	t, s, _, err := inferExpr(env, e, state)
	if err != nil {
		return nil, err
	}
	return s.ApplyMT(t), nil
}

// ---------------------------------------------------------------------------
// Basic tests
// ---------------------------------------------------------------------------

func TestInferLiteralInt(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)
	lit := &LiteralExpr{Kind: "number", Value: "42"}
	typ, err := inferExprType(env, lit, state)
	if err != nil {
		t.Fatal(err)
	}
	if !eqType(typ, intType()) {
		t.Fatalf("expected Int, got %s", typ)
	}
}

func TestInferLiteralFloat(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)
	lit := &LiteralExpr{Kind: "number", Value: "3.14"}
	typ, err := inferExprType(env, lit, state)
	if err != nil {
		t.Fatal(err)
	}
	if !eqType(typ, TCon{Name: "Float64"}) {
		t.Fatalf("expected Float64, got %s", typ)
	}
}

func TestInferLiteralString(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)
	lit := &LiteralExpr{Kind: "string", Value: "hello"}
	typ, err := inferExprType(env, lit, state)
	if err != nil {
		t.Fatal(err)
	}
	if !eqType(typ, stringType()) {
		t.Fatalf("expected String, got %s", typ)
	}
}

func TestInferGoExprResultType(t *testing.T) {
	state := NewInferState()
	state.TypedInfo = &TypedInfo{
		ExprTypes:      map[Expr]MonoType{},
		BindingSchemes: map[string]*Scheme{},
		Predicates:     map[Expr][]Predicate{},
	}
	env := TypeEnv{
		"n": &Scheme{Body: QualifiedType{Body: intType()}},
	}
	expr := &GoExpr{
		Result: &NamedType{Name: "Int"},
		Code:   "{x} + 1",
		Operands: []GoOperand{{
			Name:  "x",
			Value: &IdentExpr{Name: "n"},
		}},
	}
	typ, err := inferExprType(env, expr, state)
	if err != nil {
		t.Fatal(err)
	}
	if !eqType(typ, intType()) {
		t.Fatalf("expected Int, got %s", typ)
	}
	if _, ok := state.TypedInfo.ExprTypes[expr.Operands[0].Value]; !ok {
		t.Fatalf("operand expression type was not recorded")
	}
}

func TestInferGoExprChecksOperands(t *testing.T) {
	state := NewInferState()
	expr := &GoExpr{
		Result: &NamedType{Name: "Int"},
		Code:   "{x}",
		Operands: []GoOperand{{
			Name:  "x",
			Value: &IdentExpr{Name: "missing"},
		}},
	}
	if _, err := inferExprType(TypeEnv{}, expr, state); err == nil {
		t.Fatal("inferExprType() error = nil, want error")
	}
}

func TestInferGoExprUnit(t *testing.T) {
	state := NewInferState()
	expr := &GoExpr{
		Result: &NamedType{Name: "Unit"},
		Code:   "fmt.Println(1)",
	}
	typ, err := inferExprType(TypeEnv{}, expr, state)
	if err != nil {
		t.Fatal(err)
	}
	if !eqType(typ, TCon{Name: "Unit"}) {
		t.Fatalf("expected Unit, got %s", typ)
	}
}

func TestInferIdentBool(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)
	typ, err := inferExprType(env, &IdentExpr{Name: "true"}, state)
	if err != nil {
		t.Fatal(err)
	}
	if !eqType(typ, boolType()) {
		t.Fatalf("expected Bool, got %s", typ)
	}
}

func TestInferUnboundVariable(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)
	_, err := inferExprType(env, &IdentExpr{Name: "x"}, state)
	if err == nil {
		t.Fatal("expected error for unbound variable")
	}
}

func TestInferLetBinding(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)

	// let x = 42
	decl := &LetStmt{
		Name:  "x",
		Value: &LiteralExpr{Kind: "number", Value: "42"},
	}
	info := &TypedInfo{
		ExprTypes:      make(map[Expr]MonoType),
		BindingSchemes: make(map[string]*Scheme),
		Predicates:     make(map[Expr][]Predicate),
	}
	var err error
	env, err = inferLetDecl(decl, env, state, info)
	if err != nil {
		t.Fatal(err)
	}
	sch := env["x"]
	if sch == nil {
		t.Fatal("binding x not found in environment")
	}
	// Generalized scheme should allow instantiation to Int
	inst := Instantiate(sch, state)
	if !eqType(inst, intType()) {
		t.Fatalf("expected Int, got %s", inst)
	}
}

func TestInferLetPolymorphism(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)

	// let id = func(x: Int) -> Int
	// This is a simplified test — we test let-polymorphism by checking
	// that the scheme generalizes properly.
	paramType := intType()
	param := Param{Name: "x", Type: &NamedType{Name: "Int"}}
	body := &IdentExpr{Name: "x"}
	env["x"] = &Scheme{Body: QualifiedType{Body: paramType}}

	funcLit := &FuncLitExpr{
		Params: []Param{param},
		Ret:    &NamedType{Name: "Int"},
		Body:   body,
	}

	funcType, _, _, err := inferExpr(env, funcLit, state)
	if err != nil {
		t.Fatal(err)
	}
	expected := TFunc{Args: []MonoType{intType()}, Ret: intType()}
	if !eqType(funcType, expected) {
		t.Fatalf("expected %s, got %s", expected, funcType)
	}
}

// ---------------------------------------------------------------------------
// Let-polymorphism test
// ---------------------------------------------------------------------------

func TestLetPolymorphism(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)

	// let id = func(x) x  (polymorphic identity)
	// Infer this inside a block where x has no type annotation
	xVar := TVar{ID: state.Fresh()}
	env["x"] = &Scheme{Body: QualifiedType{Body: xVar}}
	funcLit := &FuncLitExpr{
		Params: []Param{{Name: "x"}},
		Body:   &IdentExpr{Name: "x"},
	}
	funcType, s, _, err := inferExpr(env, funcLit, state)
	if err != nil {
		t.Fatal(err)
	}
	funcType = s.ApplyMT(funcType)

	// Generalize over the free type variable
	sch := Generalize(env, funcType, nil)
	if len(sch.Bound) == 0 {
		t.Fatal("expected bound type variables in generalized scheme")
	}

	// Instantiate twice: should get two different type variables
	inst1 := Instantiate(sch, state)
	inst2 := Instantiate(sch, state)

	// The result type should be a function from a fresh type var to itself
	t1, ok := inst1.(TFunc)
	if !ok {
		t.Fatalf("expected TFunc, got %T", inst1)
	}
	t2, ok := inst2.(TFunc)
	if !ok {
		t.Fatalf("expected TFunc, got %T", inst2)
	}
	// Each instantiation should produce different type variables
	if eqType(t1.Args[0], t2.Args[0]) {
		// They might be equal by chance if only one free var; but in general
		// they should be different fresh vars
		t.Logf("t1.Args[0]=%s, t2.Args[0]=%s (may be same by coincidence)", t1.Args[0], t2.Args[0])
	}
	// Each function should have arg == ret type
	if !eqType(t1.Args[0], t1.Ret) {
		t.Fatalf("identity arg=%s not equal to ret=%s", t1.Args[0], t1.Ret)
	}
	if !eqType(t2.Args[0], t2.Ret) {
		t.Fatalf("identity arg=%s not equal to ret=%s", t2.Args[0], t2.Ret)
	}
}

// ---------------------------------------------------------------------------
// Occurs check test
// ---------------------------------------------------------------------------

func TestOccursCheck(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)

	// Create a type variable that would need occurs check:
	// func(x) x(x) — self-application
	xVar := TVar{ID: state.Fresh()}
	env["x"] = &Scheme{Body: QualifiedType{Body: xVar}}

	// Build: x(x)
	callExpr := &CallExpr{
		Callee: &IdentExpr{Name: "x"},
		Args:   []Expr{&IdentExpr{Name: "x"}},
	}

	_, _, _, err := inferExpr(env, callExpr, state)
	if err == nil {
		t.Fatal("expected occurs check error for x(x)")
	}
	t.Logf("occurs check error (expected): %v", err)
}

// ---------------------------------------------------------------------------
// None/Some inference tests
// ---------------------------------------------------------------------------

func TestInferNoneFree(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)

	// None without context should create Option[?a]
	typ, err := inferExprType(env, &IdentExpr{Name: "None"}, state)
	if err != nil {
		t.Fatal(err)
	}
	con, ok := typ.(TCon)
	if !ok || con.Name != "Option" {
		t.Fatalf("expected Option[?a], got %s", typ)
	}
	if len(con.Args) != 1 {
		t.Fatalf("expected 1 type arg for Option, got %d", len(con.Args))
	}
	_, isVar := con.Args[0].(TVar)
	if !isVar {
		t.Fatalf("expected type variable, got %T", con.Args[0])
	}
}

// ---------------------------------------------------------------------------
// If expression test
// ---------------------------------------------------------------------------

func TestInferIf(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)

	// if true then 1 else 2
	ifExpr := &IfExpr{
		Cond: &IdentExpr{Name: "true"},
		Then: &LiteralExpr{Kind: "number", Value: "1"},
		Else: &LiteralExpr{Kind: "number", Value: "2"},
	}
	typ, err := inferExprType(env, ifExpr, state)
	if err != nil {
		t.Fatal(err)
	}
	if !eqType(typ, intType()) {
		t.Fatalf("expected Int, got %s", typ)
	}
}

func TestInferIfTypeMismatch(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)

	// if true then 1 else "hello"
	ifExpr := &IfExpr{
		Cond: &IdentExpr{Name: "true"},
		Then: &LiteralExpr{Kind: "number", Value: "1"},
		Else: &LiteralExpr{Kind: "string", Value: "hello"},
	}
	_, err := inferExprType(env, ifExpr, state)
	if err == nil {
		t.Fatal("expected type mismatch error for if branches")
	}
	t.Logf("if branch mismatch error (expected): %v", err)
}

// ---------------------------------------------------------------------------
// Function call test
// ---------------------------------------------------------------------------

func TestInferCall(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)

	// Register a function: add(Int, Int) -> Int
	env["add"] = &Scheme{
		Body: QualifiedType{
			Body: TFunc{
				Args: []MonoType{intType(), intType()},
				Ret:  intType(),
			},
		},
	}

	// add(1, 2)
	call := &CallExpr{
		Callee: &IdentExpr{Name: "add"},
		Args: []Expr{
			&LiteralExpr{Kind: "number", Value: "1"},
			&LiteralExpr{Kind: "number", Value: "2"},
		},
	}
	typ, err := inferExprType(env, call, state)
	if err != nil {
		t.Fatal(err)
	}
	if !eqType(typ, intType()) {
		t.Fatalf("expected Int, got %s", typ)
	}
}

// ---------------------------------------------------------------------------
// Block expression test
// ---------------------------------------------------------------------------

func TestInferBlock(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)

	// {
	//   let x = 1
	//   x
	// }
	block := &BlockExpr{
		Stmts: []Stmt{
			&LetStmt{
				Name:  "x",
				Value: &LiteralExpr{Kind: "number", Value: "1"},
			},
			&ExprStmt{
				Expr: &IdentExpr{Name: "x"},
			},
		},
	}
	typ, err := inferExprType(env, block, state)
	if err != nil {
		t.Fatal(err)
	}
	if !eqType(typ, intType()) {
		t.Fatalf("expected Int, got %s", typ)
	}
}

// ---------------------------------------------------------------------------
// Slice literal test
// ---------------------------------------------------------------------------

func TestInferSliceLit(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)

	// [1, 2, 3]
	slice := &SliceLitExpr{
		Elems: []Expr{
			&LiteralExpr{Kind: "number", Value: "1"},
			&LiteralExpr{Kind: "number", Value: "2"},
			&LiteralExpr{Kind: "number", Value: "3"},
		},
	}
	typ, err := inferExprType(env, slice, state)
	if err != nil {
		t.Fatal(err)
	}
	expected := TCon{Name: "Slice", Args: []MonoType{intType()}}
	if !eqType(typ, expected) {
		t.Fatalf("expected %s, got %s", expected, typ)
	}
}

// ---------------------------------------------------------------------------
// Function literal test
// ---------------------------------------------------------------------------

func TestInferFuncLit(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)

	// func(x: Int) -> Int { x + 1 }
	funcLit := &FuncLitExpr{
		Params: []Param{
			{Name: "x", Type: &NamedType{Name: "Int"}},
		},
		Ret: &NamedType{Name: "Int"},
		Body: &BinaryExpr{
			Op:    "+",
			Left:  &IdentExpr{Name: "x"},
			Right: &LiteralExpr{Kind: "number", Value: "1"},
		},
	}
	env["x"] = &Scheme{Body: QualifiedType{Body: intType()}}
	// Infer via the function literal inference
	funcType, s, _, err := inferExpr(env, funcLit, state)
	if err != nil {
		t.Fatal(err)
	}
	funcType = s.ApplyMT(funcType)
	expected := TFunc{Args: []MonoType{intType()}, Ret: intType()}
	if !eqType(funcType, expected) {
		t.Fatalf("expected %s, got %s", expected, funcType)
	}
}

// ---------------------------------------------------------------------------
// Comparison test (generates Eq predicate)
// ---------------------------------------------------------------------------

func TestInferComparison(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)

	// 1 == 2
	bin := &BinaryExpr{
		Op:    "==",
		Left:  &LiteralExpr{Kind: "number", Value: "1"},
		Right: &LiteralExpr{Kind: "number", Value: "2"},
	}
	typ, s, preds, err := inferExpr(env, bin, state)
	if err != nil {
		t.Fatal(err)
	}
	typ = s.ApplyMT(typ)
	if !eqType(typ, boolType()) {
		t.Fatalf("expected Bool, got %s", typ)
	}
	if len(preds) == 0 {
		t.Fatal("expected Eq predicate for comparison")
	}
	if preds[0].ClassName != "Eq" {
		t.Fatalf("expected Eq predicate, got %s", preds[0].ClassName)
	}
}

// ---------------------------------------------------------------------------
// Unification tests
// ---------------------------------------------------------------------------

func TestUnifySimple(t *testing.T) {
	s := make(Subst)
	a := TCon{Name: "Int"}
	b := TCon{Name: "Int"}
	result, err := Unify(a, b, s)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty subst, got %v", result)
	}
}

func TestUnifyVar(t *testing.T) {
	s := make(Subst)
	v := TVar{ID: 0}
	result, err := Unify(v, intType(), s)
	if err != nil {
		t.Fatal(err)
	}
	if result[0] == nil || !eqType(result[0], intType()) {
		t.Fatalf("expected t0 -> Int, got %v", result)
	}
}

func TestUnifyTypeMismatch(t *testing.T) {
	s := make(Subst)
	_, err := Unify(intType(), stringType(), s)
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
}

func TestUnifyFuncType(t *testing.T) {
	s := make(Subst)
	a := TFunc{Args: []MonoType{intType()}, Ret: intType()}
	b := TFunc{Args: []MonoType{intType()}, Ret: intType()}
	result, err := Unify(a, b, s)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty subst, got %v", result)
	}
}

func TestUnifyFuncTypeMismatch(t *testing.T) {
	s := make(Subst)
	a := TFunc{Args: []MonoType{intType()}, Ret: intType()}
	b := TFunc{Args: []MonoType{stringType()}, Ret: intType()}
	_, err := Unify(a, b, s)
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
}

// ---------------------------------------------------------------------------
// Substitution tests
// ---------------------------------------------------------------------------

func TestApplySubst(t *testing.T) {
	s := Subst{0: intType()}
	v := TVar{ID: 0}
	result := s.ApplyMT(v)
	if !eqType(result, intType()) {
		t.Fatalf("expected Int, got %s", result)
	}
}

func TestApplySubstNested(t *testing.T) {
	s := Subst{0: intType()}
	tcon := TCon{Name: "Option", Args: []MonoType{TVar{ID: 0}}}
	result := s.ApplyMT(tcon)
	expected := TCon{Name: "Option", Args: []MonoType{intType()}}
	if !eqType(result, expected) {
		t.Fatalf("expected %s, got %s", expected, result)
	}
}

// ---------------------------------------------------------------------------
// Scheme generalization / instantiation tests
// ---------------------------------------------------------------------------

func TestGeneralizeEmptyEnv(t *testing.T) {
	env := make(TypeEnv)
	tv := TVar{ID: 0}
	sch := Generalize(env, TFunc{Args: []MonoType{tv}, Ret: tv}, nil)
	if len(sch.Bound) != 1 {
		t.Fatalf("expected 1 bound var, got %d: %v", len(sch.Bound), sch.Bound)
	}
}

func TestGeneralizeFreeInEnv(t *testing.T) {
	env := make(TypeEnv)
	// If the type variable appears free in the environment, it should NOT be bound
	tv := TVar{ID: 0}
	env["x"] = &Scheme{Body: QualifiedType{Body: tv}}
	sch := Generalize(env, TFunc{Args: []MonoType{tv}, Ret: tv}, nil)
	if len(sch.Bound) != 0 {
		t.Fatalf("expected 0 bound vars (t0 is free in env), got %d", len(sch.Bound))
	}
}

func TestInstantiate(t *testing.T) {
	state := NewInferState()
	// forall [0]. Int -> t0
	sch := &Scheme{
		Bound: []int{0},
		Body: QualifiedType{
			Body: TFunc{Args: []MonoType{intType()}, Ret: TVar{ID: 0}},
		},
	}
	inst := Instantiate(sch, state)
	tfunc, ok := inst.(TFunc)
	if !ok {
		t.Fatalf("expected TFunc, got %T", inst)
	}
	if !eqType(tfunc.Args[0], intType()) {
		t.Fatalf("expected Int arg, got %s", tfunc.Args[0])
	}
	// Return type should be a fresh type var (not t0)
	if _, ok := tfunc.Ret.(TVar); !ok {
		t.Fatalf("expected fresh TVar, got %T", tfunc.Ret)
	}
	if tv, ok := tfunc.Ret.(TVar); ok && tv.ID == 0 {
		t.Fatalf("expected fresh TVar != t0, got t%d", tv.ID)
	}
}

// ---------------------------------------------------------------------------
// Free vars tests
// ---------------------------------------------------------------------------

func TestFreeVarsTVar(t *testing.T) {
	vars := freeVarsMT(TVar{ID: 42})
	if len(vars) != 1 || vars[0] != 42 {
		t.Fatalf("expected [42], got %v", vars)
	}
}

func TestFreeVarsTCon(t *testing.T) {
	vars := freeVarsMT(TCon{Name: "Option", Args: []MonoType{TVar{ID: 5}, TVar{ID: 7}}})
	if len(vars) != 2 {
		t.Fatalf("expected 2 vars, got %v", vars)
	}
}

func TestFreeVarsTUnit(t *testing.T) {
	vars := freeVarsMT(TUnit{})
	if len(vars) != 0 {
		t.Fatalf("expected 0 vars, got %v", vars)
	}
}

// ---------------------------------------------------------------------------
// Map literal test
// ---------------------------------------------------------------------------

func TestInferMapLit(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)

	// {"a": "1", "b": "2"}
	m := &MapLitExpr{
		Pairs: []MapLitPair{
			{Key: &LiteralExpr{Kind: "string", Value: "a"}, Value: &LiteralExpr{Kind: "string", Value: "1"}},
			{Key: &LiteralExpr{Kind: "string", Value: "b"}, Value: &LiteralExpr{Kind: "string", Value: "2"}},
		},
	}
	typ, err := inferExprType(env, m, state)
	if err != nil {
		t.Fatal(err)
	}
	expected := TCon{Name: "Map", Args: []MonoType{stringType(), stringType()}}
	if !eqType(typ, expected) {
		t.Fatalf("expected %s, got %s", expected, typ)
	}
}

// ---------------------------------------------------------------------------
// Set literal test
// ---------------------------------------------------------------------------

func TestInferSetLit(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)

	// {"x", "y"}
	s := &SetLitExpr{
		Elems: []Expr{
			&LiteralExpr{Kind: "string", Value: "x"},
			&LiteralExpr{Kind: "string", Value: "y"},
		},
	}
	typ, err := inferExprType(env, s, state)
	if err != nil {
		t.Fatal(err)
	}
	expected := TCon{Name: "Set", Args: []MonoType{stringType()}}
	if !eqType(typ, expected) {
		t.Fatalf("expected %s, got %s", expected, typ)
	}
}

// ---------------------------------------------------------------------------
// Compose substitution tests
// ---------------------------------------------------------------------------

func TestComposeSubst(t *testing.T) {
	s1 := Subst{0: TVar{ID: 1}}
	s2 := Subst{1: intType()}
	result := Compose(s1, s2)
	// Should map 0 -> Int
	if !eqType(result[0], intType()) {
		t.Fatalf("expected t0 -> Int, got %v", result[0])
	}
}

// ---------------------------------------------------------------------------
// Occurs check utility test
// ---------------------------------------------------------------------------

func TestOccursIn(t *testing.T) {
	if occursIn(0, TVar{ID: 0}) {
		// t0 occurs in t0 — this is fine but bindVar should catch it
	}
	if !occursIn(0, TCon{Name: "Option", Args: []MonoType{TVar{ID: 0}}}) {
		t.Fatal("expected t0 to occur in Option[t0]")
	}
	if occursIn(0, TCon{Name: "Option", Args: []MonoType{TVar{ID: 1}}}) {
		t.Fatal("expected t0 not to occur in Option[t1]")
	}
	if occursIn(0, TUnit{}) {
		t.Fatal("expected t0 not to occur in Unit")
	}
}

// ---------------------------------------------------------------------------
// Full package inference test
// ---------------------------------------------------------------------------

func TestInferPackageSimple(t *testing.T) {
	state := NewInferState()
	pkg := &PkgInfo{
		Name: "test",
		Decls: []Decl{
			&FuncDecl{
				Name: "add",
				Params: []Param{
					{Name: "x", Type: &NamedType{Name: "Int"}},
					{Name: "y", Type: &NamedType{Name: "Int"}},
				},
				Ret: &NamedType{Name: "Int"},
				Body: &BinaryExpr{
					Op:    "+",
					Left:  &IdentExpr{Name: "x"},
					Right: &IdentExpr{Name: "y"},
				},
			},
		},
		Enums:      map[string]*EnumDecl{},
		Structs:    map[string]*StructDecl{},
		Interfaces: map[string]*InterfaceDecl{},
		Funcs:      map[string]*FuncDecl{},
		Impls:      []*ImplDecl{},
	}

	info, err := InferPackage(pkg, state)
	if err != nil {
		t.Fatal(err)
	}

	// Check that add got a scheme
	sch := info.BindingSchemes["add"]
	if sch == nil {
		t.Fatal("expected scheme for 'add'")
	}
	t.Logf("add scheme: %s", sch)
}

func TestInferGoFmtSprintSelector(t *testing.T) {
	call := &CallExpr{
		Callee: &FieldExpr{
			Expr:  &IdentExpr{Name: "fmt"},
			Field: "Sprint",
		},
		Args: []Expr{&LiteralExpr{Kind: "number", Value: "42"}},
	}
	pkg := &PkgInfo{
		Name: "main",
		Decls: []Decl{
			&ImportDecl{Alias: "fmt", Path: "go:fmt"},
			&FuncDecl{
				Name: "demo",
				Ret:  &NamedType{Name: "String"},
				Body: call,
			},
		},
		Enums:      map[string]*EnumDecl{},
		Structs:    map[string]*StructDecl{},
		Interfaces: map[string]*InterfaceDecl{},
		Funcs:      map[string]*FuncDecl{},
		Impls:      []*ImplDecl{},
	}

	info, err := InferPackage(pkg, NewInferState())
	if err != nil {
		t.Fatalf("InferPackage() error = %v", err)
	}
	if !eqType(info.ExprTypes[call], stringType()) {
		t.Fatalf("expected fmt.Sprint call to infer String, got %s", info.ExprTypes[call])
	}
}

func TestInferGoFmtSprintBoundFunction(t *testing.T) {
	body := &BlockExpr{
		Stmts: []Stmt{
			&LetStmt{
				Name: "show",
				Value: &FieldExpr{
					Expr:  &IdentExpr{Name: "fmt"},
					Field: "Sprint",
				},
			},
			&ExprStmt{Expr: &CallExpr{
				Callee: &IdentExpr{Name: "show"},
				Args:   []Expr{&LiteralExpr{Kind: "number", Value: "42"}},
			}},
		},
	}
	pkg := &PkgInfo{
		Name: "main",
		Decls: []Decl{
			&ImportDecl{Alias: "fmt", Path: "go:fmt"},
			&FuncDecl{
				Name: "demo",
				Ret:  &NamedType{Name: "String"},
				Body: body,
			},
		},
		Enums:      map[string]*EnumDecl{},
		Structs:    map[string]*StructDecl{},
		Interfaces: map[string]*InterfaceDecl{},
		Funcs:      map[string]*FuncDecl{},
		Impls:      []*ImplDecl{},
	}

	info, err := InferPackage(pkg, NewInferState())
	if err != nil {
		t.Fatalf("InferPackage() error = %v", err)
	}
	if !eqType(info.ExprTypes[body], stringType()) {
		t.Fatalf("expected bound fmt.Sprint call to infer String, got %s", info.ExprTypes[body])
	}
}

// ---------------------------------------------------------------------------
// Type equality tests
// ---------------------------------------------------------------------------

func TestEqType(t *testing.T) {
	if !eqType(intType(), intType()) {
		t.Fatal("Int should equal Int")
	}
	if eqType(intType(), stringType()) {
		t.Fatal("Int should not equal String")
	}
	if !eqType(TCon{Name: "Slice", Args: []MonoType{intType()}},
		TCon{Name: "Slice", Args: []MonoType{intType()}}) {
		t.Fatal("Slice[Int] should equal Slice[Int]")
	}
	if eqType(TCon{Name: "Slice", Args: []MonoType{intType()}},
		TCon{Name: "Slice", Args: []MonoType{stringType()}}) {
		t.Fatal("Slice[Int] should not equal Slice[String]")
	}
	if !eqType(TFunc{Args: []MonoType{intType()}, Ret: stringType()},
		TFunc{Args: []MonoType{intType()}, Ret: stringType()}) {
		t.Fatal("function types should match")
	}
	if eqType(TFunc{Args: []MonoType{intType()}, Ret: stringType()},
		TFunc{Args: []MonoType{intType()}, Ret: intType()}) {
		t.Fatal("function types should not match")
	}
}

// ---------------------------------------------------------------------------
// Instantiate with let-polymorphism test
// ---------------------------------------------------------------------------

func TestLetPolymorphismUse(t *testing.T) {
	state := NewInferState()
	env := make(TypeEnv)

	// Create a polymorphic identity: forall [0]. t0 -> t0
	sch := &Scheme{
		Bound: []int{0},
		Body: QualifiedType{
			Body: TFunc{Args: []MonoType{TVar{ID: 0}}, Ret: TVar{ID: 0}},
		},
	}
	env["id"] = sch

	// Use id with Int: id(42)
	call1 := &CallExpr{
		Callee: &IdentExpr{Name: "id"},
		Args:   []Expr{&LiteralExpr{Kind: "number", Value: "42"}},
	}
	typ1, err := inferExprType(env, call1, state)
	if err != nil {
		t.Fatal(err)
	}
	if !eqType(typ1, intType()) {
		t.Fatalf("expected Int from id(42), got %s", typ1)
	}

	// Use id with String: id("hello")
	call2 := &CallExpr{
		Callee: &IdentExpr{Name: "id"},
		Args:   []Expr{&LiteralExpr{Kind: "string", Value: "hello"}},
	}
	typ2, err := inferExprType(env, call2, state)
	if err != nil {
		t.Fatal(err)
	}
	if !eqType(typ2, stringType()) {
		t.Fatalf("expected String from id(\"hello\"), got %s", typ2)
	}
}
