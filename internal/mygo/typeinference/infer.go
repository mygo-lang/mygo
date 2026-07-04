package typeinference

import (
	"fmt"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

// ---------------------------------------------------------------------------
// Package-level inference context
// ---------------------------------------------------------------------------

// PkgInfo bundles the AST declaration maps needed for type inference.
// It mirrors the compiler's Package type without creating a circular import.
type PkgInfo struct {
	Name       string
	Decls      []Decl
	Enums      map[string]*EnumDecl
	Structs    map[string]*StructDecl
	Interfaces map[string]*InterfaceDecl
	Funcs      map[string]*FuncDecl
	Impls      []*ImplDecl
}

// TypedInfo holds the results of type inference for an entire package.
// The code generator can consult this info to avoid re-inferring types.
type TypedInfo struct {
	// ExprTypes maps each expression node to its inferred monotype.
	ExprTypes map[Expr]MonoType
	// BindingSchemes maps variable/function names to their generalized schemes.
	BindingSchemes map[string]*Scheme
	// Predicates collects the typeclass predicates generated during inference.
	Predicates map[Expr][]Predicate
}

// InferPackage runs HM type inference on an entire package AST.
// It processes declarations in order and returns a TypedInfo with inferred types.
func InferPackage(pkg *PkgInfo, state *InferState) (*TypedInfo, error) {
	info := &TypedInfo{
		ExprTypes:      make(map[Expr]MonoType),
		BindingSchemes: make(map[string]*Scheme),
		Predicates:     make(map[Expr][]Predicate),
	}

	// Store package info in state for enum variant lookups in pattern matching
	state.PkgInfo = pkg
	state.TypedInfo = info

	// Build initial type environment with built-in types and prelude
	env := initialTypeEnv(pkg)

	// Process declarations in order
	for _, decl := range pkg.Decls {
		var err error
		env, err = inferDecl(decl, env, state, info, pkg)
		if err != nil {
			return nil, err
		}
	}
	return info, nil
}

// initialTypeEnv creates the initial type environment with built-in types and
// prelude declarations.
func initialTypeEnv(pkg *PkgInfo) TypeEnv {
	env := make(TypeEnv)

	// Built-in named types (primitive type constructors)
	builtins := []string{"Int", "Int64", "Float64", "String", "Bool", "Unit"}
	for _, name := range builtins {
		t := TCon{Name: name}
		env[name] = &Scheme{Body: QualifiedType{Body: t}}
	}

	// Container type constructors (recognized as compiler intrinsics)
	containerTypes := []string{"Ref", "Option", "Result", "Slice", "Map", "Set", "List"}
	for _, name := range containerTypes {
		t := TCon{Name: name}
		env[name] = &Scheme{Body: QualifiedType{Body: t}}
	}

	// Boolean literals
	env["true"] = &Scheme{Body: QualifiedType{Body: TCon{Name: "Bool"}}}
	env["false"] = &Scheme{Body: QualifiedType{Body: TCon{Name: "Bool"}}}

	// Load prelude typeclass instances from the package's interfaces
	for name, iface := range pkg.Interfaces {
		_ = iface
		// Register interface names as type constructors in the environment
		env[name] = &Scheme{Body: QualifiedType{Body: TCon{Name: name}}}
	}

	// Prelude enum constructors: None, Some, Ok, Err
	// These will be handled explicitly during inference
	env["None"] = &Scheme{Body: QualifiedType{Body: TCon{Name: "None"}}}
	env["Some"] = &Scheme{Body: QualifiedType{Body: TCon{Name: "Some"}}}

	return env
}

// ---------------------------------------------------------------------------
// Declaration inference
// ---------------------------------------------------------------------------

func inferDecl(decl Decl, env TypeEnv, state *InferState, info *TypedInfo, pkg *PkgInfo) (TypeEnv, error) {
	switch d := decl.(type) {
	case *ImportDecl:
		if !strings.HasPrefix(d.Path, "go:") {
			return env, nil
		}
		path := strings.TrimPrefix(d.Path, "go:")
		alias := importAlias(path, d.Alias)
		info, err := loadGoPackageInfo(alias, path)
		if err != nil {
			return nil, fmt.Errorf("import %q: %w", d.Path, err)
		}
		if state.GoPackages == nil {
			state.GoPackages = map[string]*GoPackageInfo{}
		}
		state.GoPackages[alias] = info
		env = env.Clone()
		env[alias] = &Scheme{Body: QualifiedType{Body: TGoPackage{Alias: alias}}}
		return env, nil

	case *EnumDecl:
		// Register enum type constructors and variants
		return inferEnumDecl(d, env, state, pkg)

	case *StructDecl:
		// Register struct type constructors
		return inferStructDecl(d, env, state)

	case *InterfaceDecl:
		// Register interface as a type in the environment
		env = env.Clone()
		env[d.Name] = &Scheme{Body: QualifiedType{Body: TCon{Name: d.Name}}}
		return env, nil

	case *FuncDecl:
		return inferFuncDecl(d, env, state, info, pkg)

	case *ImplDecl:
		// Impl declarations register instance methods but don't directly
		// affect type inference of new bindings (they are used for
		// typeclass resolution).
		return env, nil

	case *LetStmt:
		return inferLetDecl(d, env, state, info)
	}
	return env, nil
}

func inferEnumDecl(d *EnumDecl, env TypeEnv, state *InferState, pkg *PkgInfo) (TypeEnv, error) {
	env = env.Clone()

	// Build the enum type: e.g., Option[A] -> TCon{Name: "Option", Args: [TVar]}
	typeArgs := make([]MonoType, len(d.TypeParams))
	typeParamVars := make([]int, len(d.TypeParams))
	for i, tp := range d.TypeParams {
		id := state.Fresh()
		typeArgs[i] = TVar{ID: id}
		typeParamVars[i] = id
		env[tp] = &Scheme{Body: QualifiedType{Body: TVar{ID: id}}}
	}
	enumType := TCon{Name: d.Name, Args: typeArgs}

	// Register a polymorphic scheme for the enum type itself
	env[d.Name] = Generalize(env, enumType, nil)

	// Register each variant as a constructor function
	for _, v := range d.Variants {
		// Build the type of each variant constructor
		fieldTypes := make([]MonoType, len(v.Fields))
		for i, f := range v.Fields {
			fieldTypes[i] = substituteTypeParams(typeFromAST(f.Type), d.TypeParams, typeArgs)
		}

		var variantType MonoType
		if len(fieldTypes) == 0 {
			// Nullary constructor: just returns the enum type
			variantType = enumType
		} else {
			// N-ary constructor: field types -> enum type
			variantType = TFunc{Args: fieldTypes, Ret: enumType}
		}

		// Register the variant constructor with the environment
		// For nullary constructors, we still treat them as functions of zero args
		env[v.Name] = Generalize(env, variantType, nil)
	}

	_ = pkg
	return env, nil
}

func inferStructDecl(d *StructDecl, env TypeEnv, state *InferState) (TypeEnv, error) {
	env = env.Clone()

	// Build the struct type with type parameters
	typeArgs := make([]MonoType, len(d.TypeParams))
	for i, tp := range d.TypeParams {
		id := state.Fresh()
		typeArgs[i] = TVar{ID: id}
		env[tp] = &Scheme{Body: QualifiedType{Body: TVar{ID: id}}}
	}
	structType := TCon{Name: d.Name, Args: typeArgs}

	// Register the struct type constructor
	env[d.Name] = Generalize(env, structType, nil)
	return env, nil
}

func inferFuncDecl(d *FuncDecl, env TypeEnv, state *InferState, info *TypedInfo, pkg *PkgInfo) (TypeEnv, error) {
	env = env.Clone()

	// Register type parameters in the environment
	typeParamVars := make(map[string]TVar)
	for _, tp := range d.TypeParams {
		id := state.Fresh()
		tv := TVar{ID: id}
		typeParamVars[tp] = tv
		env[tp] = &Scheme{Body: QualifiedType{Body: tv}}
	}

	// Build the function's declared type
	paramTypes := make([]MonoType, len(d.Params))
	for i, p := range d.Params {
		paramTypes[i] = typeFromAST(p.Type)
	}
	var retType MonoType
	if d.Ret != nil {
		retType = typeFromAST(d.Ret)
	} else {
		retType = TUnit{}
	}

	funcType := TFunc{Args: paramTypes, Ret: retType}

	// Register the function itself (for recursion)
	env[d.Name] = Generalize(env, funcType, nil)

	// Now infer the body
	bodyEnv := env.Clone()

	// Register parameters in the body environment
	for i, p := range d.Params {
		bodyEnv[p.Name] = &Scheme{Body: QualifiedType{Body: paramTypes[i]}}
	}

	// Register using constraints as typeclass-aware bindings
	for _, c := range d.Using {
		iface, ok := pkg.Interfaces[c.Name]
		if !ok {
			continue
		}
		for _, m := range iface.Methods {
			// For each constraint method, create a function type from the interface
			mParamTypes := make([]MonoType, len(m.Params))
			for i, mp := range m.Params {
				// Apply constraint type args to the method parameter types
				mParamTypes[i] = typeFromAST(mp.Type)
			}
			var mRetType MonoType
			if m.Ret != nil {
				mRetType = typeFromAST(m.Ret)
			} else {
				mRetType = TUnit{}
			}
			mFuncType := TFunc{Args: mParamTypes, Ret: mRetType}
			bodyEnv[m.Name] = &Scheme{Body: QualifiedType{Body: mFuncType}}
		}
	}

	// Infer body expression type
	if d.Body != nil {
		bodyType, s, preds, err := inferExpr(bodyEnv, d.Body, state)
		if err != nil {
			return nil, fmt.Errorf("function %q: %w", d.Name, err)
		}

		// Apply inferred substitution to the return type
		inferredRetType := s.ApplyMT(bodyType)

		// Unify with declared return type (if any)
		if d.Ret != nil {
			declaredRetType := s.ApplyMT(retType)
			s, err = Unify(inferredRetType, declaredRetType, s)
			if err != nil {
				return nil, fmt.Errorf("function %q return type mismatch: %w", d.Name, err)
			}
			inferredRetType = s.ApplyMT(inferredRetType)
		}

		// Apply substitution to function type
		inferredFuncType := s.ApplyMT(funcType)
		info.ExprTypes[d.Body] = s.ApplyMT(bodyType)

		// Generalize with predicates
		sch := Generalize(env, inferredFuncType, preds)
		env[d.Name] = sch
		info.BindingSchemes[d.Name] = sch
		info.Predicates[d.Body] = preds
	}

	return env, nil
}

func inferLetDecl(d *LetStmt, env TypeEnv, state *InferState, info *TypedInfo) (TypeEnv, error) {
	// Infer the value
	valType, s, preds, err := inferExpr(env, d.Value, state)
	if err != nil {
		return nil, fmt.Errorf("binding %q: %w", d.Name, err)
	}

	valType = s.ApplyMT(valType)

	// If there's an explicit type annotation, unify with it
	if d.Type != nil {
		annotType := typeFromAST(d.Type)
		var err error
		s, err = Unify(valType, annotType, s)
		if err != nil {
			return nil, fmt.Errorf("binding %q: type annotation mismatch: %w", d.Name, err)
		}
		valType = s.ApplyMT(valType)
	}

	info.ExprTypes[d.Value] = valType

	if d.Mutable {
		// var: don't generalize
		env = env.Extend(d.Name, &Scheme{Body: QualifiedType{Body: valType, Predicates: preds}})
		info.BindingSchemes[d.Name] = env[d.Name]
	} else if d.Name != "_" {
		// let: generalize
		sch := Generalize(env, valType, preds)
		env = env.Extend(d.Name, sch)
		info.BindingSchemes[d.Name] = sch
	}

	return env, nil
}

// ---------------------------------------------------------------------------
// Expression inference — Algorithm W
// ---------------------------------------------------------------------------

// inferExpr returns the inferred type, substitution, and predicates for an
// expression, using Algorithm W. It records every successfully inferred
// expression in the package TypedInfo when one is attached to the state.
func inferExpr(env TypeEnv, e Expr, state *InferState) (MonoType, Subst, []Predicate, error) {
	t, s, preds, err := inferExprRaw(env, e, state)
	if err != nil {
		return nil, nil, nil, err
	}
	if state != nil && state.TypedInfo != nil && e != nil && t != nil {
		state.TypedInfo.ExprTypes[e] = s.ApplyMT(t)
		if len(preds) > 0 {
			state.TypedInfo.Predicates[e] = preds
		}
	}
	return t, s, preds, nil
}

func inferExprRaw(env TypeEnv, e Expr, state *InferState) (MonoType, Subst, []Predicate, error) {
	switch n := e.(type) {
	case *IdentExpr:
		return inferIdent(env, n, state)
	case *LiteralExpr:
		return inferLiteral(n)
	case *CallExpr:
		return inferCall(env, n, state)
	case *BinaryExpr:
		return inferBinary(env, n, state)
	case *PrefixExpr:
		return inferPrefix(env, n, state)
	case *FieldExpr:
		return inferField(env, n, state)
	case *StructLitExpr:
		return inferStructLit(env, n, state)
	case *FuncLitExpr:
		return inferFuncLit(env, n, state)
	case *IfExpr:
		return inferIf(env, n, state)
	case *SwitchExpr:
		return inferSwitch(env, n, state)
	case *WhileExpr:
		return inferWhile(env, n)
	case *BlockExpr:
		return inferBlock(env, n, state)
	case *SliceLitExpr:
		return inferSliceLit(env, n, state)
	case *MapLitExpr:
		return inferMapLit(env, n, state)
	case *SetLitExpr:
		return inferSetLit(env, n, state)
	case *GoExpr:
		return inferGoExpr(env, n, state)
	}
	return nil, nil, nil, fmt.Errorf("unsupported expression %T", e)
}

func inferIdent(env TypeEnv, n *IdentExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	// Look up built-in identifiers
	switch n.Name {
	case "true", "false":
		return TCon{Name: "Bool"}, make(Subst), nil, nil
	case "None":
		// None: Option[A] with a fresh type variable
		a := TVar{ID: state.Fresh()}
		return TCon{Name: "Option", Args: []MonoType{a}}, make(Subst), nil, nil
	case "Nil":
		return nil, nil, nil, fmt.Errorf("Nil is not a valid value; use Option[Ref[T]] for nullable references")
	}

	// Look up in type environment
	sch, ok := env[n.Name]
	if !ok {
		return nil, nil, nil, fmt.Errorf("unbound variable %q", n.Name)
	}
	t := Instantiate(sch, state)
	return t, make(Subst), sch.Body.Predicates, nil
}

func inferLiteral(n *LiteralExpr) (MonoType, Subst, []Predicate, error) {
	switch n.Kind {
	case "number":
		if n.Value == "" {
			return nil, nil, nil, fmt.Errorf("empty number literal")
		}
		// Default to Int for integers, Float64 for floats
		hasDot := false
		for _, c := range n.Value {
			if c == '.' {
				hasDot = true
				break
			}
		}
		if hasDot {
			return TCon{Name: "Float64"}, make(Subst), nil, nil
		}
		return TCon{Name: "Int"}, make(Subst), nil, nil
	case "string":
		return TCon{Name: "String"}, make(Subst), nil, nil
	default:
		return nil, nil, nil, fmt.Errorf("unknown literal kind %q", n.Kind)
	}
}

func inferCall(env TypeEnv, n *CallExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	// Special case: Ref.new(expr)
	if field, ok := n.Callee.(*FieldExpr); ok {
		if id, ok := field.Expr.(*IdentExpr); ok && id.Name == "Ref" && field.Field == "new" {
			return inferRefNew(env, n, state)
		}
	}

	// Infer the callee type
	calleeType, s1, preds1, err := inferExpr(env, n.Callee, state)
	if err != nil {
		return nil, nil, nil, err
	}

	// Infer argument types
	argTypes := make([]MonoType, len(n.Args))
	argSubst := s1
	var allPreds []Predicate
	allPreds = append(allPreds, preds1...)

	for i, arg := range n.Args {
		argType, s, preds, err := inferExpr(env, arg, state)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("argument %d of call: %w", i, err)
		}
		argTypes[i] = s.ApplyMT(argType)
		argSubst = Compose(argSubst, s)
		allPreds = append(allPreds, preds...)
	}

	// Apply accumulated substitution to callee
	calleeType = argSubst.ApplyMT(calleeType)

	if fn, ok := calleeType.(TFunc); ok && fn.Variadic {
		return inferVariadicCall(fn, argTypes, argSubst, allPreds)
	}

	// Create a fresh return type variable
	retVar := TVar{ID: state.Fresh()}

	// Build function type from arguments to return type
	funcType := TFunc{Args: argTypes, Ret: retVar}

	// Unify callee type with function type
	argSubst, err = Unify(calleeType, funcType, argSubst)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("call type mismatch: %w", err)
	}

	// Apply substitution to get actual return type
	returnType := argSubst.ApplyMT(retVar)
	return returnType, argSubst, allPreds, nil
}

func inferVariadicCall(fn TFunc, argTypes []MonoType, s Subst, preds []Predicate) (MonoType, Subst, []Predicate, error) {
	if len(fn.Args) == 0 {
		if len(argTypes) != 0 {
			return nil, nil, nil, fmt.Errorf("variadic call type mismatch: expected 0 args, got %d", len(argTypes))
		}
		return s.ApplyMT(fn.Ret), s, preds, nil
	}
	fixed := len(fn.Args) - 1
	if len(argTypes) < fixed {
		return nil, nil, nil, fmt.Errorf("variadic call type mismatch: expected at least %d args, got %d", fixed, len(argTypes))
	}
	var err error
	for i := 0; i < fixed; i++ {
		s, err = Unify(fn.Args[i], s.ApplyMT(argTypes[i]), s)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("argument %d: %w", i, err)
		}
	}
	restType := fn.Args[len(fn.Args)-1]
	for i := fixed; i < len(argTypes); i++ {
		s, err = Unify(restType, s.ApplyMT(argTypes[i]), s)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("argument %d: %w", i, err)
		}
	}
	return s.ApplyMT(fn.Ret), s, preds, nil
}

func inferRefNew(env TypeEnv, n *CallExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	if len(n.Args) != 1 {
		return nil, nil, nil, fmt.Errorf("Ref.new expects exactly 1 arg, got %d", len(n.Args))
	}
	argType, s, preds, err := inferExpr(env, n.Args[0], state)
	if err != nil {
		return nil, nil, nil, err
	}
	argType = s.ApplyMT(argType)

	// If argument is already a Ref or pointer type, return it as-is
	if con, ok := argType.(TCon); ok && con.Name == "Ref" {
		return argType, s, preds, nil
	}

	return TCon{Name: "Ref", Args: []MonoType{argType}}, s, preds, nil
}

func inferBinary(env TypeEnv, n *BinaryExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	switch n.Op {
	case "|>", "<|":
		return inferPipe(env, n, state)
	case "+", "-", "*", "/":
		return inferArithmetic(env, n, state)
	case "&&", "||":
		return inferLogical(env, n, state)
	case "==", "!=", "<", ">", "<=", ">=":
		return inferComparison(env, n, state)
	default:
		return nil, nil, nil, fmt.Errorf("unknown binary operator %q", n.Op)
	}
}

func inferPipe(env TypeEnv, n *BinaryExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	if n.Op == "|>" {
		// left |> right  — apply right to left
		leftType, s1, preds1, err := inferExpr(env, n.Left, state)
		if err != nil {
			return nil, nil, nil, err
		}
		rightType, s2, preds2, err := inferExpr(env, n.Right, state)
		if err != nil {
			return nil, nil, nil, err
		}
		s := Compose(s1, s2)
		allPreds := append(preds1, preds2...)

		leftType = s.ApplyMT(leftType)
		rightType = s.ApplyMT(rightType)

		// If right is a call, handle as function application
		if call, ok := n.Right.(*CallExpr); ok {
			return inferCall(env, call, state)
		}

		// Otherwise, right is a function: unify with left -> ret
		retVar := TVar{ID: state.Fresh()}
		expectedFunc := TFunc{Args: []MonoType{leftType}, Ret: retVar}
		s, err = Unify(rightType, expectedFunc, s)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("pipe |> type mismatch: %w", err)
		}
		return s.ApplyMT(retVar), s, allPreds, nil
	}

	// <| pipe: apply left to right
	leftType, s1, preds1, err := inferExpr(env, n.Left, state)
	if err != nil {
		return nil, nil, nil, err
	}
	rightType, s2, preds2, err := inferExpr(env, n.Right, state)
	if err != nil {
		return nil, nil, nil, err
	}
	s := Compose(s1, s2)

	if call, ok := n.Left.(*CallExpr); ok {
		return inferCall(env, call, state)
	}

	// Otherwise, left is a function: unify
	retVar := TVar{ID: state.Fresh()}
	expectedFunc := TFunc{Args: []MonoType{rightType}, Ret: retVar}
	var unifyErr error
	s, unifyErr = Unify(leftType, expectedFunc, s)
	if unifyErr != nil {
		return nil, nil, nil, fmt.Errorf("pipe <| type mismatch: %w", unifyErr)
	}
	return s.ApplyMT(retVar), s, append(preds1, preds2...), nil
}

func inferArithmetic(env TypeEnv, n *BinaryExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	leftType, s1, preds1, err := inferExpr(env, n.Left, state)
	if err != nil {
		return nil, nil, nil, err
	}
	rightType, s2, preds2, err := inferExpr(env, n.Right, state)
	if err != nil {
		return nil, nil, nil, err
	}
	s := Compose(s1, s2)
	allPreds := append(preds1, preds2...)

	leftType = s.ApplyMT(leftType)
	rightType = s.ApplyMT(rightType)

	// Unify left and right types
	s, err = Unify(leftType, rightType, s)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("arithmetic operator %q type mismatch: %w", n.Op, err)
	}

	return s.ApplyMT(leftType), s, allPreds, nil
}

func inferLogical(env TypeEnv, n *BinaryExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	leftType, s1, preds1, err := inferExpr(env, n.Left, state)
	if err != nil {
		return nil, nil, nil, err
	}
	rightType, s2, preds2, err := inferExpr(env, n.Right, state)
	if err != nil {
		return nil, nil, nil, err
	}
	s := Compose(s1, s2)
	allPreds := append(preds1, preds2...)

	boolType := TCon{Name: "Bool"}
	s, err = Unify(leftType, boolType, s)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("logical operator %q requires Bool operands: %w", n.Op, err)
	}
	s, err = Unify(rightType, boolType, s)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("logical operator %q requires Bool operands: %w", n.Op, err)
	}
	return boolType, s, allPreds, nil
}

func inferComparison(env TypeEnv, n *BinaryExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	leftType, s1, preds1, err := inferExpr(env, n.Left, state)
	if err != nil {
		return nil, nil, nil, err
	}
	rightType, s2, preds2, err := inferExpr(env, n.Right, state)
	if err != nil {
		return nil, nil, nil, err
	}
	s := Compose(s1, s2)
	allPreds := append(preds1, preds2...)

	leftType = s.ApplyMT(leftType)
	rightType = s.ApplyMT(rightType)

	s, err = Unify(leftType, rightType, s)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("comparison operator %q type mismatch: %w", n.Op, err)
	}

	// Generate Eq predicate
	resultType := s.ApplyMT(leftType)
	eqPred := Predicate{ClassName: "Eq", Args: []MonoType{resultType}}
	allPreds = append(allPreds, eqPred)

	return TCon{Name: "Bool"}, s, allPreds, nil
}

func inferPrefix(env TypeEnv, n *PrefixExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	exprType, s, preds, err := inferExpr(env, n.Expr, state)
	if err != nil {
		return nil, nil, nil, err
	}

	switch n.Op {
	case "!":
		boolType := TCon{Name: "Bool"}
		s, err = Unify(exprType, boolType, s)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("! requires Bool operand: %w", err)
		}
		return boolType, s, preds, nil
	case "-":
		return exprType, s, preds, nil
	default:
		return nil, nil, nil, fmt.Errorf("unknown prefix operator %q", n.Op)
	}
}

func inferField(env TypeEnv, n *FieldExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	// If base is an identifier, check for enum constructors
	if id, ok := n.Expr.(*IdentExpr); ok {
		if pkg := state.GoPackages[id.Name]; pkg != nil {
			if fn, ok := pkg.Funcs[n.Field]; ok {
				return fn, make(Subst), nil, nil
			}
			return nil, nil, nil, fmt.Errorf("Go package %q has no function %q", id.Name, n.Field)
		}

		// Check if it's an enum name followed by a variant
		if sch, ok := env[id.Name]; ok {
			instType := Instantiate(sch, state)
			if con, ok := instType.(TCon); ok {
				// Check if this is an enum — variant constructor
				if _, ok := env[n.Field]; ok {
					return inferIdent(env, &IdentExpr{Name: n.Field}, state)
				}
				_ = con
			}
		}

		// Check for Go-style Enum.Variant constructor
		if sch, ok := env[n.Field]; ok {
			// The field identifies an enum variant
			instType := Instantiate(sch, state)
			return instType, make(Subst), nil, nil
		}
	}

	// Regular field access: infer base type
	baseType, s, preds, err := inferExpr(env, n.Expr, state)
	if err != nil {
		return nil, nil, nil, err
	}
	baseType = s.ApplyMT(baseType)

	// Look up field in struct types (we need to search all known structs)
	// For now, return a fresh type variable (field access type inference
	// requires structural type info which we'll refine later)
	fresh := TVar{ID: state.Fresh()}
	return fresh, s, preds, nil
}

func inferStructLit(env TypeEnv, n *StructLitExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	// Look up the struct type
	sch, ok := env[n.TypeName]
	if !ok {
		return nil, nil, nil, fmt.Errorf("unknown struct type %q", n.TypeName)
	}

	structType := Instantiate(sch, state)
	s := make(Subst)
	var allPreds []Predicate

	// If explicit type args are given, unify with them
	if len(n.TypeArgs) > 0 {
		con, ok := structType.(TCon)
		if !ok {
			return nil, nil, nil, fmt.Errorf("struct %q is not a type constructor", n.TypeName)
		}
		if len(con.Args) != len(n.TypeArgs) {
			return nil, nil, nil, fmt.Errorf("struct %q: expected %d type args, got %d",
				n.TypeName, len(con.Args), len(n.TypeArgs))
		}
		for i, arg := range n.TypeArgs {
			argType := typeFromAST(arg)
			var err error
			s, err = Unify(con.Args[i], argType, s)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("struct %q type arg %d: %w", n.TypeName, i, err)
			}
		}
		structType = s.ApplyMT(structType)
	}

	// Infer field values
	for _, f := range n.Fields {
		fieldType, fs, preds, err := inferExpr(env, f.Value, state)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("struct %q field %q: %w", n.TypeName, f.Name, err)
		}
		s = Compose(s, fs)
		allPreds = append(allPreds, preds...)
		_ = fieldType
	}

	return structType, s, allPreds, nil
}

func inferFuncLit(env TypeEnv, n *FuncLitExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	// Build environment for body
	bodyEnv := env.Clone()

	paramTypes := make([]MonoType, len(n.Params))
	for i, p := range n.Params {
		if p.Type != nil {
			paramTypes[i] = typeFromAST(p.Type)
		} else {
			paramTypes[i] = TVar{ID: state.Fresh()}
		}
		bodyEnv[p.Name] = &Scheme{Body: QualifiedType{Body: paramTypes[i]}}
	}

	// Infer body type
	bodyType, s, preds, err := inferExpr(bodyEnv, n.Body, state)
	if err != nil {
		return nil, nil, nil, err
	}
	bodyType = s.ApplyMT(bodyType)

	// If there's a declared return type, unify with it
	if n.Ret != nil {
		retType := typeFromAST(n.Ret)
		var err error
		s, err = Unify(bodyType, retType, s)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("function literal return type mismatch: %w", err)
		}
		bodyType = s.ApplyMT(bodyType)
	}

	// Build function type
	funcType := TFunc{Args: paramTypes, Ret: bodyType}
	return funcType, s, preds, nil
}

func inferIf(env TypeEnv, n *IfExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	// Infer condition type: must be Bool
	condType, s1, preds1, err := inferExpr(env, n.Cond, state)
	if err != nil {
		return nil, nil, nil, err
	}
	s, err := Unify(condType, TCon{Name: "Bool"}, s1)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("if condition must be Bool: %w", err)
	}

	// Infer then and else branches
	thenType, s2, preds2, err := inferExpr(env, n.Then, state)
	if err != nil {
		return nil, nil, nil, err
	}
	elseType, s3, preds3, err := inferExpr(env, n.Else, state)
	if err != nil {
		return nil, nil, nil, err
	}

	s = Compose(s, s2)
	s = Compose(s, s3)
	allPreds := append(preds1, preds2...)
	allPreds = append(allPreds, preds3...)

	thenType = s.ApplyMT(thenType)
	elseType = s.ApplyMT(elseType)

	// Unify then and else branch types
	s, err = Unify(thenType, elseType, s)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("if branch types mismatch: %w", err)
	}

	return s.ApplyMT(thenType), s, allPreds, nil
}

func inferSwitch(env TypeEnv, n *SwitchExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	// Infer target type
	targetType, s, preds, err := inferExpr(env, n.Target, state)
	if err != nil {
		return nil, nil, nil, err
	}
	targetType = s.ApplyMT(targetType)

	if len(n.Cases) == 0 {
		return TUnit{}, s, preds, nil
	}

	// Resolve target type to enum name and type args for pattern binding
	enumName, enumTypeArgs := resolveEnumType(targetType)
	enumDecl := lookupEnum(state.PkgInfo, enumName)

	// Infer each case body type and unify them
	var caseTypes []MonoType
	var allPreds []Predicate
	allPreds = append(allPreds, preds...)
	seenCaseStmts := map[Stmt]struct{}{}

	for _, cas := range n.Cases {
		caseEnv := env.Clone()

		// Extend environment with pattern bindings
		switch pat := cas.Pattern.(type) {
		case *VariantPattern:
			activeEnum := enumDecl
			variant, ok := findEnumVariant(activeEnum, pat.Name)
			if !ok {
				activeEnum, variant, ok = lookupVariant(state.PkgInfo, pat.Name)
			}
			if ok {
				// For each pattern arg, look up the variant field type and
				// substitute enum type parameters with the target type arguments.
				// e.g. target Option[Int] + variant Some(A) → binding a: Int
				for i, arg := range pat.Args {
					if arg == "_" {
						continue
					}
					bound := false
					if i < len(variant.Fields) {
						fieldType := typeFromAST(variant.Fields[i].Type)
						if activeEnum != nil && len(activeEnum.TypeParams) > 0 && len(enumTypeArgs) > 0 {
							fieldType = substituteTypeParams(fieldType, activeEnum.TypeParams, enumTypeArgs)
						}
						caseEnv[arg] = &Scheme{Body: QualifiedType{Body: fieldType}}
						bound = true
					}
					if !bound {
						caseEnv[arg] = &Scheme{Body: QualifiedType{Body: TVar{ID: state.Fresh()}}}
					}
				}
			} else {
				for _, arg := range pat.Args {
					if arg == "_" {
						continue
					}
					caseEnv[arg] = &Scheme{Body: QualifiedType{Body: TVar{ID: state.Fresh()}}}
				}
			}
		}

		caseBody := switchCaseBodyForInference(cas.Body, seenCaseStmts)
		caseType, cs, cp, err := inferExpr(caseEnv, caseBody, state)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("switch case: %w", err)
		}
		rememberSwitchCaseStmts(cas.Body, seenCaseStmts)
		caseType = cs.ApplyMT(caseType)
		caseTypes = append(caseTypes, caseType)
		s = Compose(s, cs)
		allPreds = append(allPreds, cp...)
	}

	// Unify all case body types
	resultType := caseTypes[0]
	for i := 1; i < len(caseTypes); i++ {
		s, err = Unify(resultType, caseTypes[i], s)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("switch case types mismatch: %w", err)
		}
		resultType = s.ApplyMT(resultType)
	}
	return resultType, s, allPreds, nil
}

func switchCaseBodyForInference(body Expr, seen map[Stmt]struct{}) Expr {
	b, ok := body.(*BlockExpr)
	if !ok || len(b.Stmts) == 0 || len(seen) == 0 {
		return body
	}
	var stmts []Stmt
	for _, st := range b.Stmts {
		if _, ok := seen[st]; ok {
			continue
		}
		stmts = append(stmts, st)
	}
	if len(stmts) == len(b.Stmts) {
		return body
	}
	return &BlockExpr{Line: b.Line, Column: b.Column, Stmts: stmts}
}

func rememberSwitchCaseStmts(body Expr, seen map[Stmt]struct{}) {
	b, ok := body.(*BlockExpr)
	if !ok {
		return
	}
	for _, st := range b.Stmts {
		seen[st] = struct{}{}
	}
}

func inferWhile(env TypeEnv, n *WhileExpr) (MonoType, Subst, []Predicate, error) {
	// While loops return Unit — the condition must be Bool, but we skip
	// inference of the body for type checking since while is a control flow:
	// it repeats until false.
	return TUnit{}, make(Subst), nil, nil
}

func inferBlock(env TypeEnv, n *BlockExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	currentEnv := env
	s := make(Subst)
	var allPreds []Predicate
	stmts := effectiveBlockStmts(n)

	for i, stmt := range stmts {
		isLast := i == len(stmts)-1
		switch st := stmt.(type) {
		case *ExprStmt:
			stmtType, ss, preds, err := inferExpr(currentEnv, st.Expr, state)
			if err != nil {
				return nil, nil, nil, err
			}
			s = Compose(s, ss)
			allPreds = append(allPreds, preds...)

			if isLast {
				return s.ApplyMT(stmtType), s, allPreds, nil
			}
		case *LetStmt:
			var err error
			currentEnv, err = inferLetDecl(st, currentEnv, state, &TypedInfo{
				ExprTypes:      make(map[Expr]MonoType),
				BindingSchemes: make(map[string]*Scheme),
				Predicates:     make(map[Expr][]Predicate),
			})
			if err != nil {
				return nil, nil, nil, err
			}
		case *AssignStmt:
			// Mutable assignment: infer the value and check it's assignable
			targetType, ok := currentEnv[st.Name]
			if !ok {
				return nil, nil, nil, fmt.Errorf("unknown binding %q in assignment", st.Name)
			}
			valType, ss, preds, err := inferExpr(currentEnv, st.Value, state)
			if err != nil {
				return nil, nil, nil, err
			}
			s = Compose(s, ss)
			allPreds = append(allPreds, preds...)

			targetInst := Instantiate(targetType, state)
			valType = s.ApplyMT(valType)
			s, err = Unify(targetInst, valType, s)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("assignment to %q: type mismatch: %w", st.Name, err)
			}
		case *ReturnStmt:
			if st.Value != nil {
				valType, ss, preds, err := inferExpr(currentEnv, st.Value, state)
				if err != nil {
					return nil, nil, nil, err
				}
				s = Compose(s, ss)
				allPreds = append(allPreds, preds...)
				_ = valType
			}
			if isLast {
				return TUnit{}, s, allPreds, nil
			}
		}
	}

	return TUnit{}, s, allPreds, nil
}

func effectiveBlockStmts(n *BlockExpr) []Stmt {
	if n == nil || len(n.Stmts) < 2 {
		if n == nil {
			return nil
		}
		return n.Stmts
	}
	lastExpr, ok := n.Stmts[len(n.Stmts)-1].(*ExprStmt)
	if !ok {
		return n.Stmts
	}
	sw, ok := lastExpr.Expr.(*SwitchExpr)
	if !ok {
		return n.Stmts
	}
	hoisted := map[Stmt]struct{}{}
	for _, c := range sw.Cases {
		if b, ok := c.Body.(*BlockExpr); ok {
			for _, st := range b.Stmts {
				hoisted[st] = struct{}{}
			}
		}
	}
	if len(hoisted) == 0 {
		return n.Stmts
	}
	out := make([]Stmt, 0, len(n.Stmts))
	for _, st := range n.Stmts[:len(n.Stmts)-1] {
		if _, ok := hoisted[st]; ok {
			continue
		}
		out = append(out, st)
	}
	out = append(out, lastExpr)
	return out
}

func inferSliceLit(env TypeEnv, n *SliceLitExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	if len(n.Elems) == 0 {
		// Empty slice: requires a type from annotation or context
		if n.Elem != nil {
			elemType := typeFromAST(n.Elem)
			return TCon{Name: "Slice", Args: []MonoType{elemType}}, make(Subst), nil, nil
		}
		// Create a fresh element type variable
		elemVar := TVar{ID: state.Fresh()}
		return TCon{Name: "Slice", Args: []MonoType{elemVar}}, make(Subst), nil, nil
	}

	// Infer element types from elements
	s := make(Subst)
	var allPreds []Predicate
	var elemTypes []MonoType
	for _, elem := range n.Elems {
		elemType, es, preds, err := inferExpr(env, elem, state)
		if err != nil {
			return nil, nil, nil, err
		}
		s = Compose(s, es)
		allPreds = append(allPreds, preds...)
		elemTypes = append(elemTypes, s.ApplyMT(elemType))
	}

	// Unify all element types
	elemType := elemTypes[0]
	for i := 1; i < len(elemTypes); i++ {
		var err error
		s, err = Unify(elemType, elemTypes[i], s)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("slice element type mismatch: %w", err)
		}
		elemType = s.ApplyMT(elemType)
	}

	return TCon{Name: "Slice", Args: []MonoType{elemType}}, s, allPreds, nil
}

func inferMapLit(env TypeEnv, n *MapLitExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	if len(n.Pairs) == 0 {
		if n.Key != nil && n.Val != nil {
			keyType := typeFromAST(n.Key)
			valType := typeFromAST(n.Val)
			return TCon{Name: "Map", Args: []MonoType{keyType, valType}}, make(Subst), nil, nil
		}
		kVar := TVar{ID: state.Fresh()}
		vVar := TVar{ID: state.Fresh()}
		return TCon{Name: "Map", Args: []MonoType{kVar, vVar}}, make(Subst), nil, nil
	}

	s := make(Subst)
	var allPreds []Predicate

	// Infer first pair to establish expected key/value types
	firstKeyType, ks1, preds1, err := inferExpr(env, n.Pairs[0].Key, state)
	if err != nil {
		return nil, nil, nil, err
	}
	firstValType, vs1, preds2, err := inferExpr(env, n.Pairs[0].Value, state)
	if err != nil {
		return nil, nil, nil, err
	}
	s = Compose(s, ks1)
	s = Compose(s, vs1)
	allPreds = append(allPreds, preds1...)
	allPreds = append(allPreds, preds2...)

	keyType := s.ApplyMT(firstKeyType)
	valType := s.ApplyMT(firstValType)

	// Unify remaining pairs
	for i := 1; i < len(n.Pairs); i++ {
		kt, ks, preds, err := inferExpr(env, n.Pairs[i].Key, state)
		if err != nil {
			return nil, nil, nil, err
		}
		s = Compose(s, ks)
		allPreds = append(allPreds, preds...)
		s, err = Unify(keyType, s.ApplyMT(kt), s)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("map key types mismatch: %w", err)
		}

		vt, vs, preds, err := inferExpr(env, n.Pairs[i].Value, state)
		if err != nil {
			return nil, nil, nil, err
		}
		s = Compose(s, vs)
		allPreds = append(allPreds, preds...)
		s, err = Unify(valType, s.ApplyMT(vt), s)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("map value types mismatch: %w", err)
		}
	}

	return TCon{Name: "Map", Args: []MonoType{keyType, valType}}, s, allPreds, nil
}

func inferSetLit(env TypeEnv, n *SetLitExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	if len(n.Elems) == 0 {
		if n.Elem != nil {
			elemType := typeFromAST(n.Elem)
			return TCon{Name: "Set", Args: []MonoType{elemType}}, make(Subst), nil, nil
		}
		elemVar := TVar{ID: state.Fresh()}
		return TCon{Name: "Set", Args: []MonoType{elemVar}}, make(Subst), nil, nil
	}

	s := make(Subst)
	var allPreds []Predicate
	var elemTypes []MonoType
	for _, elem := range n.Elems {
		elemType, es, preds, err := inferExpr(env, elem, state)
		if err != nil {
			return nil, nil, nil, err
		}
		s = Compose(s, es)
		allPreds = append(allPreds, preds...)
		elemTypes = append(elemTypes, s.ApplyMT(elemType))
	}

	elemType := elemTypes[0]
	for i := 1; i < len(elemTypes); i++ {
		var err error
		s, err = Unify(elemType, elemTypes[i], s)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("set element type mismatch: %w", err)
		}
		elemType = s.ApplyMT(elemType)
	}

	return TCon{Name: "Set", Args: []MonoType{elemType}}, s, allPreds, nil
}

func inferGoExpr(env TypeEnv, n *GoExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	s := make(Subst)
	var allPreds []Predicate
	for _, op := range n.Operands {
		_, os, preds, err := inferExpr(env, op.Value, state)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("go operand %q: %w", op.Name, err)
		}
		s = Compose(s, os)
		allPreds = append(allPreds, preds...)
	}
	return typeFromAST(n.Result), s, allPreds, nil
}

// ---------------------------------------------------------------------------
// Pattern-matching helpers
// ---------------------------------------------------------------------------

// resolveEnumType extracts the enum type constructor name and its type arguments
// from a MonoType. For example, from Option[Int] it returns ("Option", [Int]).
func resolveEnumType(t MonoType) (string, []MonoType) {
	switch t := t.(type) {
	case TCon:
		return t.Name, t.Args
	}
	return "", nil
}

// lookupEnum looks up an enum declaration by name from the package info.
func lookupEnum(pkg *PkgInfo, name string) *EnumDecl {
	if pkg == nil {
		return nil
	}
	return pkg.Enums[name]
}

// findEnumVariant finds a variant by name within an enum declaration.
func findEnumVariant(enum *EnumDecl, name string) (*EnumVariant, bool) {
	if enum == nil {
		return nil, false
	}
	for i := range enum.Variants {
		if enum.Variants[i].Name == name {
			return &enum.Variants[i], true
		}
	}
	return nil, false
}

func lookupVariant(pkg *PkgInfo, name string) (*EnumDecl, *EnumVariant, bool) {
	if pkg == nil {
		return nil, nil, false
	}
	for _, enum := range pkg.Enums {
		if variant, ok := findEnumVariant(enum, name); ok {
			return enum, variant, true
		}
	}
	return nil, nil, false
}

// substituteTypeParams substitutes type parameter names in a MonoType with the
// corresponding concrete types from typeArgs, using the enum's TypeParams list
// as the mapping key.
func substituteTypeParams(t MonoType, typeParams []string, typeArgs []MonoType) MonoType {
	if len(typeParams) == 0 || len(typeArgs) == 0 {
		return t
	}
	switch t := t.(type) {
	case TVar:
		return t
	case TCon:
		// Check if this TCon is a type parameter
		for i, tp := range typeParams {
			if t.Name == tp && len(t.Args) == 0 {
				if i < len(typeArgs) {
					return typeArgs[i]
				}
			}
		}
		// Recursively substitute in args
		args := make([]MonoType, len(t.Args))
		for i, a := range t.Args {
			args[i] = substituteTypeParams(a, typeParams, typeArgs)
		}
		return TCon{Name: t.Name, Args: args}
	case TFunc:
		args := make([]MonoType, len(t.Args))
		for i, a := range t.Args {
			args[i] = substituteTypeParams(a, typeParams, typeArgs)
		}
		return TFunc{Args: args, Ret: substituteTypeParams(t.Ret, typeParams, typeArgs), Variadic: t.Variadic}
	case TGoPackage:
		return t
	case TUnit:
		return t
	}
	return t
}
