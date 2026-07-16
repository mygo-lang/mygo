package typeinference

import (
	"fmt"
	"os"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
	"github.com/mygo-lang/mygo/internal/mygo/common"
)

// ---------------------------------------------------------------------------
// Package-level inference context
// ---------------------------------------------------------------------------

// PkgInfo bundles the AST declaration maps needed for type inference.
// It mirrors the compiler's Package type without creating a circular import.
type PkgInfo struct {
	Dir            string
	WorkspaceRoot  string
	Name           string
	Decls          []Decl
	Enums          map[string]*EnumDecl
	Structs        map[string]*StructDecl
	Interfaces     map[string]*InterfaceDecl
	Funcs          map[string]*FuncDecl
	Impls          []*ImplDecl
	SourceFiles    map[any]string
	DotImportEnums map[string]*EnumDecl // enums from dot-imported packages
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

func inherentReceiverName(t TypeExpr) string {
	if nt, ok := t.(*NamedType); ok {
		return nt.Name
	}
	return ""
}

// typeExprToString serializes a TypeExpr to a string representation.
// For HKT types like C[A], it returns "C[A]" instead of just "C".
func typeExprToString(t TypeExpr) string {
	if nt, ok := t.(*NamedType); ok {
		if len(nt.Args) > 0 {
			args := make([]string, len(nt.Args))
			for i, a := range nt.Args {
				args[i] = typeExprToString(a)
			}
			return nt.Name + "[" + strings.Join(args, ", ") + "]"
		}
		return nt.Name
	}
	if ft, ok := t.(*FuncType); ok {
		params := make([]string, len(ft.Params))
		for i, p := range ft.Params {
			params[i] = typeExprToString(p)
		}
		ret := typeExprToString(ft.Ret)
		return "func(" + strings.Join(params, ", ") + ") -> " + ret
	}
	if tt, ok := t.(*TupleType); ok {
		if len(tt.Elems) == 0 {
			return "()"
		}
		elems := make([]string, len(tt.Elems))
		for i, e := range tt.Elems {
			elems[i] = typeExprToString(e)
		}
		return "(" + strings.Join(elems, ", ") + ")"
	}
	return "unknown"
}

func inherentMethodName(receiverName, methodName string) string {
	return receiverName + "_" + methodName
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

	// For test packages (name ends with "_test"), auto-import the main package
	// symbols via dot-import effect. This allows test files to reference
	// exported symbols without qualification. We register main package
	// functions and types so they're available as unqualified names.
	if strings.HasSuffix(pkg.Name, "_test") {
		mainPkgName := strings.TrimSuffix(pkg.Name, "_test")
		if mainPkgInfo, err := loadMyGoPackageInfo(pkg.WorkspaceRoot, pkg.Dir, pkg.Dir, mainPkgName, nil); err == nil {
			// Register all exported functions from main package with their
			// polymorphic signatures so test packages infer real result types.
			for name, sch := range mainPkgInfo.Funcs {
				env[name] = sch
			}
			// Register all exported types (structs, enums) from main package
			for name := range mainPkgInfo.Types {
				env[name] = &Scheme{Body: QualifiedType{Body: TCon{Name: name}}}
			}
			if state.PkgInfo != nil && mainPkgInfo.Structs != nil {
				if state.PkgInfo.Structs == nil {
					state.PkgInfo.Structs = map[string]*StructDecl{}
				}
				for name, st := range mainPkgInfo.Structs {
					if _, exists := state.PkgInfo.Structs[name]; !exists {
						state.PkgInfo.Structs[name] = st
					}
				}
			}
		}
	}

	// Pre-register top-level function signatures so forward references work.
	for _, decl := range pkg.Decls {
		fn, ok := decl.(*FuncDecl)
		if !ok {
			continue
		}
		env[fn.Name] = funcDeclSignatureScheme(fn, env, state)
	}
	for _, impl := range pkg.Impls {
		if impl.InterfaceName != "" || impl.Name != "" {
			continue
		}
		receiverName := inherentReceiverName(impl.Type)
		if receiverName == "" {
			continue
		}
		for _, fn := range impl.Methods {
			env[inherentMethodName(receiverName, fn.Name)] = funcDeclSignatureScheme(fn, env, state)
		}
	}

	// Process declarations in order
	for _, decl := range pkg.Decls {
		var err error
		env, err = inferDecl(decl, env, state, info, pkg)
		if err != nil {
			return nil, errorAtNode(pkg, decl, err)
		}
	}
	return info, nil
}

func funcDeclSignatureScheme(fn *FuncDecl, env TypeEnv, state *InferState) *Scheme {
	typeParamVars := make(map[string]MonoType, len(fn.TypeParams))
	for _, tp := range fn.TypeParams {
		typeParamVars[tp] = TVar{ID: state.Fresh()}
	}
	paramTypes := make([]MonoType, len(fn.Params))
	for i, p := range fn.Params {
		paramTypes[i] = typeFromASTWithParams(p.Type, typeParamVars)
	}
	var retType MonoType = TUnit{}
	if fn.Ret != nil {
		retType = typeFromASTWithParams(fn.Ret, typeParamVars)
	}
	return Generalize(env, TFunc{Args: paramTypes, Ret: retType}, nil)
}

func sourceFileFor(pkg *PkgInfo, node any) string {
	if file := common.NodeSourceFile(node); file != "" {
		return file
	}
	if pkg != nil && pkg.SourceFiles != nil {
		return pkg.SourceFiles[node]
	}
	return ""
}

func errorAtNode(pkg *PkgInfo, node any, err error) error {
	if err == nil || common.IsPositionedError(err) {
		return err
	}
	return common.ErrorAtNode(sourceFileFor(pkg, node), node, "%v", err)
}

func wrapInferenceError(format string, err error, args ...any) error {
	if err == nil || common.IsPositionedError(err) {
		return err
	}
	args = append(args, err)
	return fmt.Errorf(format, args...)
}

// initialTypeEnv creates the initial type environment with built-in types and
// prelude declarations.
func initialTypeEnv(pkg *PkgInfo) TypeEnv {
	env := make(TypeEnv)

	// Built-in named types (primitive type constructors)
	builtins := []string{"Int", "Int8", "UInt8", "Int16", "UInt16", "Int32", "UInt32", "Int64", "UInt", "UInt64", "Float32", "Float64", "Byte", "Rune", "String", "Bool", "Unit"}
	for _, name := range builtins {
		t := TCon{Name: name}
		env[name] = &Scheme{Body: QualifiedType{Body: t}}
	}

	// Container and prelude type constructors (recognized as compiler intrinsics)
	containerTypes := []string{"Ref", "Option", "Result", "Slice", "Map", "Set", "List", "ToString", "Eq", "IEnumerable", "IOption"}
	for _, name := range containerTypes {
		t := TCon{Name: name}
		env[name] = &Scheme{Body: QualifiedType{Body: t}}
	}

	// Boolean literals
	env["true"] = &Scheme{Body: QualifiedType{Body: TCon{Name: "Bool"}}}
	env["false"] = &Scheme{Body: QualifiedType{Body: TCon{Name: "Bool"}}}

	// Prelude constructors and helper functions that remain globally visible.
	for _, name := range []string{"Some", "Ok", "Err", "Zero", "OptionFlatMap", "ResultIsOk", "ResultIsErr", "ResultUnwrap", "ResultMap", "ResultMapErr", "ResultAndThen", "ResultOrElse", "TypeKeyFromType"} {
		env[name] = &Scheme{Body: QualifiedType{Body: TCon{Name: name}}}
	}

	// Package-local interfaces remain available as types.
	for name, iface := range pkg.Interfaces {
		_ = iface
		env[name] = &Scheme{Body: QualifiedType{Body: TCon{Name: name}}}
	}

	return env
}

// ---------------------------------------------------------------------------
// Declaration inference
// ---------------------------------------------------------------------------

func inferDecl(decl Decl, env TypeEnv, state *InferState, info *TypedInfo, pkg *PkgInfo) (TypeEnv, error) {
	switch d := decl.(type) {
	case *ImportDecl:
		alias := d.Alias
		if alias == "" {
			alias = importAlias(strings.TrimPrefix(d.Path, "go:"), d.Alias)
		}
		if strings.HasPrefix(d.Path, "go:") {
			path := strings.TrimPrefix(d.Path, "go:")
			goInfo, err := loadGoPackageInfo(alias, path)
			if err != nil {
				return nil, wrapInferenceError("import %q: %w", err, d.Path)
			}
			if state.GoPackages == nil {
				state.GoPackages = map[string]*GoPackageInfo{}
			}
			state.GoPackages[alias] = goInfo
			env = env.Clone()
			// Dot-import: register all exported Go symbols directly in env.
			if alias == "." {
				for name, fn := range goInfo.Funcs {
					env[name] = &Scheme{Body: QualifiedType{Body: fn}}
				}
			} else {
				env[alias] = &Scheme{Body: QualifiedType{Body: TGoPackage{Alias: alias}}}
			}
			return env, nil
		}
		if state.PkgInfo == nil || state.PkgInfo.Dir == "" {
			return env, nil
		}
		workspaceRoot := ""
		if pkg != nil {
			workspaceRoot = pkg.WorkspaceRoot
		}
		mygoInfo, err := loadMyGoPackageInfo(workspaceRoot, state.PkgInfo.Dir, d.Path, alias, state.MyGoPackageCache)
		if err != nil {
			return nil, wrapInferenceError("import %q: %w", err, d.Path)
		}
		if state.MyGoPackages == nil {
			state.MyGoPackages = map[string]*MyGoPackageInfo{}
		}
		state.MyGoPackages[alias] = mygoInfo
		env = env.Clone()
		// Dot-import: register all exported MyGO symbols directly in env.
		if alias == "." {
			for name, sch := range mygoInfo.Funcs {
				env[name] = sch
			}
			for name := range mygoInfo.Types {
				env[name] = &Scheme{Body: QualifiedType{Body: TCon{Name: name}}}
			}
			// Merge struct info so field access works for imported structs.
			if state.PkgInfo != nil && mygoInfo.Structs != nil {
				for name, st := range mygoInfo.Structs {
					if _, exists := state.PkgInfo.Structs[name]; !exists {
						state.PkgInfo.Structs[name] = st
					}
				}
			}
		} else {
			env[alias] = &Scheme{Body: QualifiedType{Body: TCon{Name: alias}}}
		}
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

	// Register a polymorphic scheme for the enum type itself.
	// Temporarily remove type parameters so Generalize properly quantifies them.
	for _, tp := range d.TypeParams {
		delete(env, tp)
	}
	env[d.Name] = Generalize(env, enumType, nil)

	// Only prelude Option/Result payload constructors remain globally visible.
	// Other enum variants must be called through Enum.Variant(...).
	if d.Name == "Option" || d.Name == "Result" {
		for _, v := range d.Variants {
			if !isGlobalPreludeVariant(v.Name) {
				continue
			}
			variantType := enumVariantConstructorType(d, &v, typeArgs)
			for _, tp := range d.TypeParams {
				delete(env, tp)
			}
			env[v.Name] = Generalize(env, variantType, nil)
		}
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

	// Register the struct type constructor.
	// Temporarily remove type parameters so Generalize properly quantifies them.
	for _, tp := range d.TypeParams {
		delete(env, tp)
	}
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

	// Build the function's declared type, replacing type parameter names
	// with their corresponding TVar so Generalize can quantify them.
	tpMapping := make(map[string]MonoType)
	for name, tv := range typeParamVars {
		tpMapping[name] = tv
	}

	paramTypes := make([]MonoType, len(d.Params))
	for i, p := range d.Params {
		paramTypes[i] = typeFromASTWithParams(p.Type, tpMapping)
	}
	var retType MonoType
	if d.Ret != nil {
		retType = typeFromASTWithParams(d.Ret, tpMapping)
	} else {
		retType = TUnit{}
	}

	funcType := TFunc{Args: paramTypes, Ret: retType}

	// Register the function itself (for recursion).
	// Temporarily remove type parameters from env so Generalize properly
	// quantifies them — otherwise they appear "free in env" and are never
	// bound, causing Instantiate to return the same shared TVar across
	// calls and breaking monomorphisation / type-argument inference.
	for _, tp := range d.TypeParams {
		delete(env, tp)
	}
	env[d.Name] = Generalize(env, funcType, nil)

	// Now infer the body
	bodyEnv := env.Clone()
	for name, tv := range typeParamVars {
		bodyEnv[name] = &Scheme{Body: QualifiedType{Body: tv}}
	}

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
		// Convert constraint type args to MonoTypes for substitution.
		constraintArgTypes := make([]MonoType, len(c.Args))
		for i, arg := range c.Args {
			constraintArgTypes[i] = typeFromAST(arg)
		}
		for _, m := range iface.Methods {
			// For each constraint method, create a function type from the interface,
			// substituting interface type params with concrete constraint arg types.
			mParamTypes := make([]MonoType, len(m.Params))
			for i, mp := range m.Params {
				mt := typeFromAST(mp.Type)
				mParamTypes[i] = substituteTypeParams(mt, iface.TypeParams, constraintArgTypes)
			}
			var mRetType MonoType
			if m.Ret != nil {
				mRetType = typeFromAST(m.Ret)
				mRetType = substituteTypeParams(mRetType, iface.TypeParams, constraintArgTypes)
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
			return nil, wrapInferenceError("function %q: %w", err, d.Name)
		}

		// Apply inferred substitution to the return type
		inferredRetType := s.ApplyMT(bodyType)

		// If the declared return type is Unit, skip unification — the last
		// expression's value is discarded (void context).  Otherwise unify.
		if d.Ret != nil {
			_, isUnit := retType.(TUnit)
			if !isUnit {
				if tc, ok := retType.(TCon); ok && tc.Name == "Unit" {
					isUnit = true
				}
			}
			if isUnit {
				// Force function return type to TUnit{} (void).
				funcType = TFunc{Args: paramTypes, Ret: TUnit{}}
			} else {
				declaredRetType := s.ApplyMT(retType)
				fmt.Fprintf(os.Stderr, "DEBUG inferFuncDecl %q: bodyType=%s inferredRetType=%s retType=%s declaredRetType=%s\n",
					d.Name, s.ApplyMT(bodyType), inferredRetType, retType, declaredRetType)
				if dc, ok := declaredRetType.(TCon); ok {
					fmt.Fprintf(os.Stderr, "DEBUG declaredRetType TCon Name=%s Args=%v\n", dc.Name, dc.Args)
				}
				if ic, ok := inferredRetType.(TCon); ok {
					fmt.Fprintf(os.Stderr, "DEBUG inferredRetType TCon Name=%s Args=%v\n", ic.Name, ic.Args)
				}
				if goFFIRefAutoWrapsToOption(d.Body, inferredRetType, declaredRetType) {
					inferredRetType = declaredRetType
				} else {
					s, err = Unify(inferredRetType, declaredRetType, s)
					if err != nil {
						return nil, wrapInferenceError("function %q return type mismatch: %w", err, d.Name)
					}
					inferredRetType = s.ApplyMT(inferredRetType)
				}
			}
		}

		// Apply substitution to function type
		// inferred func type preserved as original funcType
		info.ExprTypes[d.Body] = s.ApplyMT(bodyType)

		// Generalize with predicates
		sch := Generalize(env, funcType, preds)
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
		return nil, wrapInferenceError("binding %q: %w", err, d.Name)
	}

	valType = s.ApplyMT(valType)

	// If there's an explicit type annotation, unify with it
	if d.Type != nil {
		annotType := typeFromAST(d.Type)
		var err error
		s, err = Unify(valType, annotType, s)
		if err != nil {
			return nil, wrapInferenceError("binding %q: type annotation mismatch: %w", err, d.Name)
		}
		valType = s.ApplyMT(valType)
	}

	info.ExprTypes[d.Value] = valType

	if d.Bind != nil {
		tuple, ok := valType.(TCon)
		if !ok || tuple.Name != "Tuple" {
			return nil, fmt.Errorf("binding %q: tuple destructuring requires a tuple value", d.Name)
		}
		newEnv, err := inferBindPattern(d.Bind, tuple.Args, s, env, info)
		if err != nil {
			return nil, wrapInferenceError("binding %q: %w", err, d.Name)
		}
		env = newEnv
		return env, nil
	}

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

func inferBindPattern(p BindPattern, args []MonoType, s Subst, env TypeEnv, info *TypedInfo) (TypeEnv, error) {
	switch pat := p.(type) {
	case *BindNamePattern:
		if len(args) != 1 {
			return nil, fmt.Errorf("pattern %q arity mismatch", pat.Name)
		}
		if pat.Name == "_" {
			return env, nil
		}
		elemType := s.ApplyMT(args[0])
		env = env.Extend(pat.Name, &Scheme{Body: QualifiedType{Body: elemType}})
		info.BindingSchemes[pat.Name] = env[pat.Name]
		return env, nil
	case *BindTuplePattern:
		if len(args) != len(pat.Elems) {
			return nil, fmt.Errorf("tuple destructuring arity mismatch")
		}
		for i, elem := range pat.Elems {
			nextArgs := []MonoType{args[i]}
			if nested, ok := s.ApplyMT(args[i]).(TCon); ok && nested.Name == "Tuple" {
				nextArgs = nested.Args
			}
			newEnv, err := inferBindPattern(elem, nextArgs, s, env, info)
			if err != nil {
				return nil, err
			}
			env = newEnv
		}
		return env, nil
	default:
		return nil, fmt.Errorf("unsupported binding pattern %T", p)
	}
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
		if state != nil {
			err = errorAtNode(state.PkgInfo, e, err)
		}
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
	case *CastExpr:
		return inferCast(env, n, state)
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
	case *TupleLitExpr:
		elems := make([]MonoType, 0, len(n.Elems))
		var subst Subst = make(Subst)
		var preds []Predicate
		for _, e := range n.Elems {
			t, s, ps, err := inferExpr(env, e, state)
			if err != nil {
				return nil, nil, nil, err
			}
			elems = append(elems, s.ApplyMT(t))
			subst = Compose(subst, s)
			preds = append(preds, ps...)
		}
		return TCon{Name: "Tuple", Args: elems}, subst, preds, nil
	case *UnitLitExpr:
		return TUnit{}, make(Subst), nil, nil
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
	case "Some":
		a := TVar{ID: state.Fresh()}
		return TFunc{Args: []MonoType{a}, Ret: TCon{Name: "Option", Args: []MonoType{a}}}, make(Subst), nil, nil
	case "Zero":
		a := TVar{ID: state.Fresh()}
		return TFunc{Ret: a}, make(Subst), nil, nil
	case "Ok":
		a := TVar{ID: state.Fresh()}
		e := TVar{ID: state.Fresh()}
		return TFunc{Args: []MonoType{a}, Ret: TCon{Name: "Result", Args: []MonoType{a, e}}}, make(Subst), nil, nil
	case "Err":
		a := TVar{ID: state.Fresh()}
		e := TVar{ID: state.Fresh()}
		return TFunc{Args: []MonoType{e}, Ret: TCon{Name: "Result", Args: []MonoType{a, e}}}, make(Subst), nil, nil
	case "Nil":
		return nil, nil, nil, fmt.Errorf("Nil is not a valid value; use Option[Ref[T]] for nullable references")
	}

	// Look up in type environment
	sch, ok := env[n.Name]
	if !ok {
		if state != nil && state.PkgInfo != nil {
			for _, iface := range state.PkgInfo.Interfaces {
				for _, m := range iface.Methods {
					if m.Name != n.Name {
						continue
					}
					typeArgs := make([]MonoType, len(iface.TypeParams))
					for i := range iface.TypeParams {
						typeArgs[i] = TVar{ID: state.Fresh()}
					}
					args := make([]MonoType, len(m.Params))
					for i, p := range m.Params {
						args[i] = substituteTypeParams(typeFromAST(p.Type), iface.TypeParams, typeArgs)
					}
					ret := MonoType(TUnit{})
					if m.Ret != nil {
						ret = substituteTypeParams(typeFromAST(m.Ret), iface.TypeParams, typeArgs)
					}
					return TFunc{Args: args, Ret: ret}, make(Subst), nil, nil
				}
			}
		}
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
		info := ParseNumericLiteral(n.Value)
		return TCon{Name: info.Type}, make(Subst), nil, nil
	case "string":
		return TCon{Name: "String"}, make(Subst), nil, nil
	case "rune":
		return TCon{Name: "Rune"}, make(Subst), nil, nil
	default:
		return nil, nil, nil, fmt.Errorf("unknown literal kind %q", n.Kind)
	}
}

func inferCall(env TypeEnv, n *CallExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	if id, ok := n.Callee.(*IdentExpr); ok && id.Name == "None" {
		return nil, nil, nil, fmt.Errorf("None is a value constructor; use None, not None()")
	}
	// Special case: Ref.new(expr)
	if field, ok := n.Callee.(*FieldExpr); ok {
		if enumName, fn, _, ok := qualifiedEnumVariantConstructor(field, state); ok {
			argTypes := make([]MonoType, len(n.Args))
			argSubst := make(Subst)
			var allPreds []Predicate
			for i, arg := range n.Args {
				argType, s, preds, err := inferExpr(env, arg, state)
				if err != nil {
					return nil, nil, nil, wrapInferenceError("argument %d of %s.%s: %w", err, i, enumName, field.Field)
				}
				argSubst = Compose(argSubst, s)
				argTypes[i] = argSubst.ApplyMT(argType)
				allPreds = append(allPreds, preds...)
			}
			retVar := TVar{ID: state.Fresh()}
			argSubst, err := Unify(fn, TFunc{Args: argTypes, Ret: retVar}, argSubst)
			if err != nil {
				return nil, nil, nil, wrapInferenceError("call type mismatch: %w", err)
			}
			return argSubst.ApplyMT(retVar), argSubst, allPreds, nil
		}
		if id, ok := field.Expr.(*IdentExpr); ok && id.Name == "Ref" && field.Field == "new" {
			return inferRefNew(env, n, state)
		}
		if field.Field == "value" && len(n.Args) == 0 {
			baseType, s, preds, err := inferExpr(env, field.Expr, state)
			if err != nil {
				return nil, nil, nil, err
			}
			baseType = s.ApplyMT(baseType)
			if con, ok := baseType.(TCon); ok && con.Name == "Ref" && len(con.Args) == 1 {
				return s.ApplyMT(con.Args[0]), s, preds, nil
			}
		}
		if id, ok := field.Expr.(*IdentExpr); !ok || (state.GoPackages[id.Name] == nil && state.MyGoPackages[id.Name] == nil && !isInherentStaticMethodSelector(id.Name, field.Field, state)) {
			receiverType, s1, preds1, err := inferExpr(env, field.Expr, state)
			if err != nil {
				return nil, nil, nil, err
			}

			argTypes := make([]MonoType, len(n.Args))
			argSubst := s1
			var allPreds []Predicate
			allPreds = append(allPreds, preds1...)

			for i, arg := range n.Args {
				argType, s, preds, err := inferExpr(env, arg, state)
				if err != nil {
					return nil, nil, nil, wrapInferenceError("argument %d of call: %w", err, i)
				}
				argSubst = Compose(argSubst, s)
				argTypes[i] = argSubst.ApplyMT(argType)
				allPreds = append(allPreds, preds...)
			}

			receiverType = argSubst.ApplyMT(receiverType)
			if fn, ok := receiverType.(TFunc); ok && fn.Variadic {
				return inferVariadicCall(fn, argTypes, argSubst, allPreds)
			}
			if fieldFn, ok := structFunctionFieldType(receiverType, field.Field, state); ok {
				if fieldFn.Variadic {
					return inferVariadicCall(fieldFn, argTypes, argSubst, allPreds)
				}
				retVar := TVar{ID: state.Fresh()}
				funcType := TFunc{Args: argTypes, Ret: retVar}
				argSubst, err = Unify(fieldFn, funcType, argSubst)
				if err != nil {
					return nil, nil, nil, wrapInferenceError("field function call type mismatch: %w", err)
				}
				return argSubst.ApplyMT(retVar), argSubst, allPreds, nil
			}

			retVar := TVar{ID: state.Fresh()}
			funcType := TFunc{Args: append([]MonoType{receiverType}, argTypes...), Ret: retVar}
			argSubst, err = Unify(receiverType, receiverType, argSubst)
			if err != nil {
				return nil, nil, nil, err
			}
			calleeType, s2, preds2, err := inferExpr(env, n.Callee, state)
			if err != nil {
				return nil, nil, nil, err
			}
			argSubst = Compose(argSubst, s2)
			allPreds = append(allPreds, preds2...)
			calleeType = argSubst.ApplyMT(calleeType)
			argSubst, err = Unify(calleeType, funcType, argSubst)
			if err != nil {
				return nil, nil, nil, wrapInferenceError("call type mismatch: %w", err)
			}
			return argSubst.ApplyMT(retVar), argSubst, allPreds, nil
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
			return nil, nil, nil, wrapInferenceError("argument %d of call: %w", err, i)
		}
		argSubst = Compose(argSubst, s)
		argTypes[i] = argSubst.ApplyMT(argType)
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

	fmt.Fprintf(os.Stderr, "DEBUG inferCall: calleeType=%s funcType=%s argCount=%d\n", calleeType.String(), funcType.String(), len(argTypes))
	// Unify callee type with function type
	argSubst, err = Unify(calleeType, funcType, argSubst)
	if err != nil {
		return nil, nil, nil, wrapInferenceError("call type mismatch: %w", err)
	}

	// Apply substitution to get actual return type
	returnType := argSubst.ApplyMT(retVar)
	return returnType, argSubst, allPreds, nil
}

func structFunctionFieldType(receiverType MonoType, fieldName string, state *InferState) (TFunc, bool) {
	if ref, ok := receiverType.(TCon); ok && ref.Name == "Ref" && len(ref.Args) == 1 {
		receiverType = ref.Args[0]
	}
	con, ok := receiverType.(TCon)
	if !ok || state == nil || state.PkgInfo == nil {
		return TFunc{}, false
	}
	st := state.PkgInfo.Structs[con.Name]
	if st == nil {
		return TFunc{}, false
	}
	for _, f := range st.Fields {
		if f.Name != fieldName {
			continue
		}
		fieldType := typeFromAST(f.Type)
		if len(con.Args) == len(st.TypeParams) {
			fieldType = substituteTypeParams(fieldType, st.TypeParams, con.Args)
		}
		fn, ok := fieldType.(TFunc)
		return fn, ok
	}
	return TFunc{}, false
}

func isInherentStaticMethodSelector(receiverName, methodName string, state *InferState) bool {
	_, ok := inherentStaticMethodType(receiverName, methodName, state)
	return ok
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
			return nil, nil, nil, wrapInferenceError("argument %d: %w", err, i)
		}
	}
	restType := fn.Args[len(fn.Args)-1]
	for i := fixed; i < len(argTypes); i++ {
		s, err = Unify(restType, s.ApplyMT(argTypes[i]), s)
		if err != nil {
			return nil, nil, nil, wrapInferenceError("argument %d: %w", err, i)
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
			return nil, nil, nil, wrapInferenceError("pipe |> type mismatch: %w", err)
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
		return nil, nil, nil, wrapInferenceError("pipe <| type mismatch: %w", unifyErr)
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
		return nil, nil, nil, wrapInferenceError("arithmetic operator %q type mismatch: %w", err, n.Op)
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
		return nil, nil, nil, wrapInferenceError("logical operator %q requires Bool operands: %w", err, n.Op)
	}
	s, err = Unify(rightType, boolType, s)
	if err != nil {
		return nil, nil, nil, wrapInferenceError("logical operator %q requires Bool operands: %w", err, n.Op)
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
		return nil, nil, nil, wrapInferenceError("comparison operator %q type mismatch: %w", err, n.Op)
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
			return nil, nil, nil, wrapInferenceError("! requires Bool operand: %w", err)
		}
		return boolType, s, preds, nil
	case "-":
		return exprType, s, preds, nil
	default:
		return nil, nil, nil, fmt.Errorf("unknown prefix operator %q", n.Op)
	}
}

func inferCast(env TypeEnv, n *CastExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	_, s, preds, err := inferExpr(env, n.Expr, state)
	if err != nil {
		return nil, nil, nil, err
	}
	return typeFromAST(n.Type), s, preds, nil
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
		if pkg := state.MyGoPackages[id.Name]; pkg != nil {
			if sch, ok := pkg.Funcs[n.Field]; ok {
				inst := Instantiate(sch, state)
				qualified := qualifyMyGoType(id.Name, pkg.Types, inst)
				return qualified, make(Subst), sch.Body.Predicates, nil
			}
			if _, ok := pkg.Types[n.Field]; ok {
				return TCon{Name: id.Name + "." + n.Field}, make(Subst), nil, nil
			}
			return nil, nil, nil, fmt.Errorf("MyGO package %q has no exported symbol %q", id.Name, n.Field)
		}
		if fn, ok := inherentStaticMethodType(id.Name, n.Field, state); ok {
			return fn, make(Subst), nil, nil
		}

		if _, fn, arity, ok := qualifiedEnumVariantConstructor(n, state); ok {
			if arity == 0 {
				return fn.Ret, make(Subst), nil, nil
			}
			return fn, make(Subst), nil, nil
		}
	}

	// Regular field access: infer base type
	baseType, s, preds, err := inferExpr(env, n.Expr, state)
	if err != nil {
		return nil, nil, nil, err
	}
	baseType = s.ApplyMT(baseType)
	if ref, ok := baseType.(TCon); ok && ref.Name == "Ref" && len(ref.Args) == 1 {
		baseType = s.ApplyMT(ref.Args[0])
	}

	// No debug logging — field resolution is internal.
	_ = baseType

	// If the base is a concrete struct, resolve the field type from package metadata.
	if con, ok := baseType.(TCon); ok && state != nil && state.PkgInfo != nil {
		if st := state.PkgInfo.Structs[con.Name]; st != nil {
			for _, f := range st.Fields {
				if f.Name != n.Field {
					continue
				}
				if len(con.Args) == len(st.TypeParams) {
					fieldTypeParams := make(map[string]MonoType, len(st.TypeParams))
					for i, name := range st.TypeParams {
						fieldTypeParams[name] = con.Args[i]
					}
					return typeFromASTWithParams(f.Type, fieldTypeParams), s, preds, nil
				}
				return typeFromAST(f.Type), s, preds, nil
			}
			for _, impl := range state.PkgInfo.Impls {
				if impl.InterfaceName != "" || impl.Name != "" || inherentReceiverName(impl.Type) != con.Name {
					continue
				}
				for _, m := range impl.Methods {
					if m.Name != n.Field {
						continue
					}
					methodTypeParams := make(map[string]MonoType, len(impl.TypeParams)+len(m.TypeParams))
					if receiver, ok := impl.Type.(*NamedType); ok && len(con.Args) == len(receiver.Args) {
						for i, name := range impl.TypeParams {
							methodTypeParams[name] = con.Args[i]
						}
					}
					for _, name := range m.TypeParams {
						methodTypeParams[name] = TVar{ID: state.Fresh()}
					}
					paramTypes := make([]MonoType, 0, len(m.Params))
					for _, p := range m.Params {
						paramTypes = append(paramTypes, typeFromASTWithParams(p.Type, methodTypeParams))
					}
					ret := MonoType(TUnit{})
					if m.Ret != nil {
						ret = typeFromASTWithParams(m.Ret, methodTypeParams)
					}
					return TFunc{Args: paramTypes, Ret: ret}, s, preds, nil
				}
			}
			return nil, nil, nil, fmt.Errorf("struct %q has no field or method %q", con.Name, n.Field)
		}
		for _, iface := range state.PkgInfo.Interfaces {
			for _, m := range iface.Methods {
				if m.Name != n.Field {
					continue
				}
				typeArgs := make([]MonoType, len(iface.TypeParams))
				for i := range iface.TypeParams {
					typeArgs[i] = TVar{ID: state.Fresh()}
				}
				methodTypeParams := make(map[string]MonoType, len(iface.TypeParams)+len(m.TypeParams))
				for i, name := range iface.TypeParams {
					methodTypeParams[name] = typeArgs[i]
				}
				for _, name := range m.TypeParams {
					methodTypeParams[name] = TVar{ID: state.Fresh()}
				}
				paramTypes := make([]MonoType, 0, len(m.Params))
				for _, p := range m.Params {
					paramTypes = append(paramTypes, typeFromASTWithParams(p.Type, methodTypeParams))
				}
				ret := MonoType(TUnit{})
				if m.Ret != nil {
					ret = typeFromASTWithParams(m.Ret, methodTypeParams)
				}
				return TFunc{Args: paramTypes, Ret: ret}, s, preds, nil
			}
		}
	}

	// Fall back to a fresh variable when the base type is not concrete yet.
	fresh := TVar{ID: state.Fresh()}
	return fresh, s, preds, nil
}

func inherentStaticMethodType(receiverName, methodName string, state *InferState) (TFunc, bool) {
	if state == nil || state.PkgInfo == nil {
		return TFunc{}, false
	}
	for _, impl := range state.PkgInfo.Impls {
		if impl.InterfaceName != "" || impl.Name != "" || inherentReceiverName(impl.Type) != receiverName {
			continue
		}
		for _, m := range impl.Methods {
			if m.Name != methodName || inherentMethodHasReceiver(impl, m) {
				continue
			}
			methodTypeParams := make(map[string]MonoType, len(impl.TypeParams)+len(m.TypeParams))
			for _, name := range impl.TypeParams {
				methodTypeParams[name] = TVar{ID: state.Fresh()}
			}
			for _, name := range m.TypeParams {
				methodTypeParams[name] = TVar{ID: state.Fresh()}
			}
			paramTypes := make([]MonoType, 0, len(m.Params))
			for _, p := range m.Params {
				paramTypes = append(paramTypes, typeFromASTWithParams(p.Type, methodTypeParams))
			}
			ret := MonoType(TUnit{})
			if m.Ret != nil {
				ret = typeFromASTWithParams(m.Ret, methodTypeParams)
			}
			return TFunc{Args: paramTypes, Ret: ret}, true
		}
	}
	return TFunc{}, false
}

func inherentMethodHasReceiver(impl *ImplDecl, m *FuncDecl) bool {
	if impl == nil || m == nil || len(m.Params) == 0 {
		return false
	}
	return sameTypeExpr(m.Params[0].Type, impl.Type)
}

func sameTypeExpr(a, b TypeExpr) bool {
	switch a := a.(type) {
	case *NamedType:
		b, ok := b.(*NamedType)
		if !ok || a.Name != b.Name || len(a.Args) != len(b.Args) {
			return false
		}
		for i := range a.Args {
			if !sameTypeExpr(a.Args[i], b.Args[i]) {
				return false
			}
		}
		return true
	case *FuncType:
		b, ok := b.(*FuncType)
		if !ok || len(a.Params) != len(b.Params) {
			return false
		}
		for i := range a.Params {
			if !sameTypeExpr(a.Params[i], b.Params[i]) {
				return false
			}
		}
		return sameTypeExpr(a.Ret, b.Ret)
	case *TupleType:
		b, ok := b.(*TupleType)
		if !ok || len(a.Elems) != len(b.Elems) {
			return false
		}
		for i := range a.Elems {
			if !sameTypeExpr(a.Elems[i], b.Elems[i]) {
				return false
			}
		}
		return true
	default:
		return a == nil && b == nil
	}
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
			argType := typeFromASTInEnv(arg, env, state)
			var err error
			s, err = Unify(con.Args[i], argType, s)
			if err != nil {
				return nil, nil, nil, wrapInferenceError("struct %q type arg %d: %w", err, n.TypeName, i)
			}
		}
		structType = s.ApplyMT(structType)
	}

	// Infer field values
	for _, f := range n.Fields {
		var fieldDecl *Field
		if state != nil && state.PkgInfo != nil {
			if st := state.PkgInfo.Structs[n.TypeName]; st != nil {
				for i := range st.Fields {
					if st.Fields[i].Name == f.Name {
						fieldDecl = &st.Fields[i]
						break
					}
				}
			}
		}
		if fieldDecl != nil {
			if mapLit, ok := f.Value.(*MapLitExpr); ok && len(mapLit.Pairs) == 0 && mapLit.Key == nil && mapLit.Val == nil {
				if named, ok := fieldDecl.Type.(*NamedType); ok && len(named.Args) == 2 && named.Name == "Map" {
					mapLit.Key = named.Args[0]
					mapLit.Val = named.Args[1]
				}
			}
		}
		fieldType, fs, preds, err := inferExpr(env, f.Value, state)
		if err != nil {
			return nil, nil, nil, wrapInferenceError("struct %q field %q: %w", err, n.TypeName, f.Name)
		}
		s = Compose(s, fs)
		allPreds = append(allPreds, preds...)
		if fieldDecl != nil {
			expectedFieldType := typeFromAST(fieldDecl.Type)
			if state != nil && state.PkgInfo != nil {
				if st := state.PkgInfo.Structs[n.TypeName]; st != nil {
					if con, ok := s.ApplyMT(structType).(TCon); ok && len(con.Args) == len(st.TypeParams) {
						fieldTypeParams := make(map[string]MonoType, len(st.TypeParams))
						for i, name := range st.TypeParams {
							fieldTypeParams[name] = con.Args[i]
						}
						expectedFieldType = typeFromASTWithParams(fieldDecl.Type, fieldTypeParams)
					}
				}
			}
			var unifyErr error
			s, unifyErr = Unify(s.ApplyMT(fieldType), expectedFieldType, s)
			if unifyErr != nil {
				return nil, nil, nil, wrapInferenceError("struct %q field %q type mismatch: %w", unifyErr, n.TypeName, f.Name)
			}
		}
	}

	return structType, s, allPreds, nil
}

func inferFuncLit(env TypeEnv, n *FuncLitExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	// Build environment for body
	bodyEnv := env.Clone()

	paramTypes := make([]MonoType, len(n.Params))
	for i, p := range n.Params {
		if p.Type != nil {
			paramTypes[i] = typeFromASTInEnv(p.Type, env, state)
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
		retType := typeFromASTInEnv(n.Ret, env, state)
		var err error
		s, err = Unify(bodyType, retType, s)
		if err != nil {
			return nil, nil, nil, wrapInferenceError("function literal return type mismatch: %w", err)
		}
		bodyType = s.ApplyMT(bodyType)
	}

	// Build function type
	funcType := TFunc{Args: paramTypes, Ret: bodyType}
	return funcType, s, preds, nil
}

func typeFromASTInEnv(t TypeExpr, env TypeEnv, state *InferState) MonoType {
	switch t := t.(type) {
	case *NamedType:
		if len(t.Args) == 0 {
			if sch, ok := env[t.Name]; ok {
				return Instantiate(sch, state)
			}
		}
		args := make([]MonoType, len(t.Args))
		for i, arg := range t.Args {
			args[i] = typeFromASTInEnv(arg, env, state)
		}
		return TCon{Name: t.Name, Args: args}
	case *FuncType:
		params := make([]MonoType, len(t.Params))
		for i, p := range t.Params {
			params[i] = typeFromASTInEnv(p, env, state)
		}
		return TFunc{Args: params, Ret: typeFromASTInEnv(t.Ret, env, state)}
	case *TupleType:
		if len(t.Elems) == 0 {
			return TUnit{}
		}
		args := make([]MonoType, len(t.Elems))
		for i, elem := range t.Elems {
			args[i] = typeFromASTInEnv(elem, env, state)
		}
		return TCon{Name: "Tuple", Args: args}
	default:
		return typeFromAST(t)
	}
}

func inferIf(env TypeEnv, n *IfExpr, state *InferState) (MonoType, Subst, []Predicate, error) {
	// Infer condition type: must be Bool
	condType, s1, preds1, err := inferExpr(env, n.Cond, state)
	if err != nil {
		return nil, nil, nil, err
	}
	s, err := Unify(condType, TCon{Name: "Bool"}, s1)
	if err != nil {
		return nil, nil, nil, wrapInferenceError("if condition must be Bool: %w", err)
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
		return nil, nil, nil, wrapInferenceError("if branch types mismatch: %w", err)
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
		caseEnv, err := inferPatternBindings(caseEnv, cas.Pattern, targetType, s, state, enumDecl, enumTypeArgs)
		if err != nil {
			return nil, nil, nil, wrapInferenceError("switch pattern: %w", err)
		}

		caseBody := switchCaseBodyForInference(cas.Body, seenCaseStmts)
		caseType, cs, cp, err := inferExpr(caseEnv, caseBody, state)
		if err != nil {
			return nil, nil, nil, wrapInferenceError("switch case: %w", err)
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
			return nil, nil, nil, wrapInferenceError("switch case types mismatch: %w", err)
		}
		resultType = s.ApplyMT(resultType)
	}
	return resultType, s, allPreds, nil
}

func inferPatternBindings(env TypeEnv, pat Pattern, targetType MonoType, s Subst, state *InferState, enumDecl *EnumDecl, enumTypeArgs []MonoType) (TypeEnv, error) {
	switch p := pat.(type) {
	case *WildcardPattern:
		return env, nil
	case *TuplePattern:
		tupleType, ok := s.ApplyMT(targetType).(TCon)
		if !ok || tupleType.Name != "Tuple" {
			return nil, fmt.Errorf("tuple pattern requires a tuple value")
		}
		if len(tupleType.Args) != len(p.Elems) {
			return nil, fmt.Errorf("tuple pattern arity mismatch")
		}
		for i, elem := range p.Elems {
			newEnv, err := inferPatternBindings(env, elem, tupleType.Args[i], s, state, enumDecl, enumTypeArgs)
			if err != nil {
				return nil, err
			}
			env = newEnv
		}
		return env, nil
	case *VariantPattern:
		if enumName, enumArgs := resolveEnumType(targetType); enumName == "Option" {
			if p.Name == "Some" {
				if len(p.Args) > 0 && p.Args[0] != "_" && len(enumArgs) > 0 {
					env[p.Args[0]] = &Scheme{Body: QualifiedType{Body: enumArgs[0]}}
				}
				return env, nil
			}
			if p.Name == "None" {
				return env, nil
			}
		}
		if enumName, enumArgs := resolveEnumType(targetType); enumName == "Result" {
			if p.Name == "Ok" {
				if len(p.Args) > 0 && p.Args[0] != "_" && len(enumArgs) > 0 {
					env[p.Args[0]] = &Scheme{Body: QualifiedType{Body: enumArgs[0]}}
				}
				return env, nil
			}
			if p.Name == "Err" {
				if len(p.Args) > 0 && p.Args[0] != "_" && len(enumArgs) > 1 {
					env[p.Args[0]] = &Scheme{Body: QualifiedType{Body: enumArgs[1]}}
				}
				return env, nil
			}
		}
		activeEnum := enumDecl
		variant, ok := findEnumVariant(activeEnum, p.Name)
		if !ok {
			activeEnum, variant, ok = lookupVariant(state.PkgInfo, p.Name)
		}
		if !ok {
			switch enumNameFromType(targetType) {
			case "Option":
				if p.Name == "Some" {
					if len(p.Args) > 0 && p.Args[0] != "_" && len(enumTypeArgs) > 0 {
						env[p.Args[0]] = &Scheme{Body: QualifiedType{Body: enumTypeArgs[0]}}
					}
					return env, nil
				}
				if p.Name == "None" {
					return env, nil
				}
			case "Result":
				if p.Name == "Ok" && len(enumTypeArgs) > 0 {
					if len(p.Args) > 0 && p.Args[0] != "_" {
						env[p.Args[0]] = &Scheme{Body: QualifiedType{Body: enumTypeArgs[0]}}
					}
					return env, nil
				}
				if p.Name == "Err" && len(enumTypeArgs) > 1 {
					if len(p.Args) > 0 && p.Args[0] != "_" {
						env[p.Args[0]] = &Scheme{Body: QualifiedType{Body: enumTypeArgs[1]}}
					}
					return env, nil
				}
			}
			return nil, fmt.Errorf("variant pattern %q requires an enum target", p.Name)
		}
		if ok {
			for i, arg := range p.Args {
				if arg == "_" {
					continue
				}
				bound := false
				if i < len(variant.Fields) {
					fieldType := typeFromAST(variant.Fields[i].Type)
					if activeEnum != nil && len(activeEnum.TypeParams) > 0 && len(enumTypeArgs) > 0 {
						fieldType = substituteTypeParams(fieldType, activeEnum.TypeParams, enumTypeArgs)
					}
					env[arg] = &Scheme{Body: QualifiedType{Body: fieldType}}
					bound = true
				}
				if !bound {
					env[arg] = &Scheme{Body: QualifiedType{Body: TVar{ID: state.Fresh()}}}
				}
			}
			return env, nil
		}
		for _, arg := range p.Args {
			if arg == "_" {
				continue
			}
			env[arg] = &Scheme{Body: QualifiedType{Body: TVar{ID: state.Fresh()}}}
		}
		return env, nil
	case *LiteralPattern:
		switch p.Kind {
		case "string":
			if err := unifyPatternLiteral(targetType, TCon{Name: "String"}, s); err != nil {
				return nil, wrapInferenceError("string literal pattern: %w", err)
			}
		case "number":
			lt, _, _, err := inferLiteral(&LiteralExpr{Kind: "number", Value: p.Value})
			if err != nil {
				return nil, wrapInferenceError("number literal pattern: %w", err)
			}
			if err := unifyPatternLiteral(targetType, lt, s); err != nil {
				return nil, wrapInferenceError("number literal pattern: %w", err)
			}
		}
		return env, nil
	default:
		return nil, fmt.Errorf("unsupported pattern %T", pat)
	}
}

func unifyPatternLiteral(target MonoType, concrete MonoType, s Subst) error {
	t := s.ApplyMT(target)
	next, err := Unify(t, concrete, s)
	if err != nil {
		return err
	}
	for k, v := range next {
		s[k] = v
	}
	return nil
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
				return nil, nil, nil, wrapInferenceError("assignment to %q: type mismatch: %w", err, st.Name)
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
			return nil, nil, nil, wrapInferenceError("slice element type mismatch: %w", err)
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
			return nil, nil, nil, wrapInferenceError("map key types mismatch: %w", err)
		}

		vt, vs, preds, err := inferExpr(env, n.Pairs[i].Value, state)
		if err != nil {
			return nil, nil, nil, err
		}
		s = Compose(s, vs)
		allPreds = append(allPreds, preds...)
		s, err = Unify(valType, s.ApplyMT(vt), s)
		if err != nil {
			return nil, nil, nil, wrapInferenceError("map value types mismatch: %w", err)
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
			return nil, nil, nil, wrapInferenceError("set element type mismatch: %w", err)
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
			return nil, nil, nil, wrapInferenceError("go operand %q: %w", err, op.Name)
		}
		s = Compose(s, os)
		allPreds = append(allPreds, preds...)
	}
	resultType := typeFromAST(n.Result)
	// Go FFI auto-conversion: (A, B) tuple → Result[A, B]
	// This enables go[((), error)] to be inferred as Result[Unit, error].
	if tup, ok := resultType.(TCon); ok && tup.Name == "Tuple" && len(tup.Args) == 2 {
		resultType = TCon{Name: "Result", Args: tup.Args}
	}
	return resultType, s, allPreds, nil
}

func goFFIRefAutoWrapsToOption(body Expr, actual, expected MonoType) bool {
	if _, ok := body.(*GoExpr); !ok {
		return false
	}
	actualRef, ok := actual.(TCon)
	if !ok || actualRef.Name != "Ref" || len(actualRef.Args) != 1 {
		return false
	}
	expectedOption, ok := expected.(TCon)
	if !ok || expectedOption.Name != "Option" || len(expectedOption.Args) != 1 {
		return false
	}
	expectedRef, ok := expectedOption.Args[0].(TCon)
	if !ok || expectedRef.Name != "Ref" || len(expectedRef.Args) != 1 {
		return false
	}
	return monoTypesEqual(actualRef.Args[0], expectedRef.Args[0])
}

func monoTypesEqual(a, b MonoType) bool {
	switch a := a.(type) {
	case TVar:
		b, ok := b.(TVar)
		return ok && a.ID == b.ID
	case TCon:
		b, ok := b.(TCon)
		if !ok || a.Name != b.Name || len(a.Args) != len(b.Args) {
			return false
		}
		for i := range a.Args {
			if !monoTypesEqual(a.Args[i], b.Args[i]) {
				return false
			}
		}
		return true
	case TFunc:
		b, ok := b.(TFunc)
		if !ok || a.Variadic != b.Variadic || len(a.Args) != len(b.Args) {
			return false
		}
		for i := range a.Args {
			if !monoTypesEqual(a.Args[i], b.Args[i]) {
				return false
			}
		}
		return monoTypesEqual(a.Ret, b.Ret)
	case TGoPackage:
		b, ok := b.(TGoPackage)
		return ok && a.Alias == b.Alias
	case TUnit:
		_, ok := b.(TUnit)
		return ok
	}
	return false
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

func enumNameFromType(t MonoType) string {
	name, _ := resolveEnumType(t)
	return name
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

func isGlobalPreludeVariant(name string) bool {
	switch name {
	case "Some", "Ok", "Err":
		return true
	default:
		return false
	}
}

func enumVariantConstructorType(enum *EnumDecl, variant *EnumVariant, enumTypeArgs []MonoType) TFunc {
	fieldTypes := make([]MonoType, len(variant.Fields))
	for i, f := range variant.Fields {
		fieldTypes[i] = substituteTypeParams(typeFromAST(f.Type), enum.TypeParams, enumTypeArgs)
	}
	return TFunc{Args: fieldTypes, Ret: TCon{Name: enum.Name, Args: enumTypeArgs}}
}

func qualifiedEnumVariantConstructor(field *FieldExpr, state *InferState) (string, TFunc, int, bool) {
	if state == nil || state.PkgInfo == nil {
		return "", TFunc{}, 0, false
	}
	id, ok := field.Expr.(*IdentExpr)
	if !ok {
		return "", TFunc{}, 0, false
	}
	enum := lookupEnum(state.PkgInfo, id.Name)
	if enum == nil {
		return "", TFunc{}, 0, false
	}
	variant, ok := findEnumVariant(enum, field.Field)
	if !ok {
		return "", TFunc{}, 0, false
	}
	typeArgs := make([]MonoType, len(enum.TypeParams))
	for i := range enum.TypeParams {
		typeArgs[i] = TVar{ID: state.Fresh()}
	}
	return enum.Name, enumVariantConstructorType(enum, variant, typeArgs), len(variant.Fields), true
}

func lookupVariant(pkg *PkgInfo, name string) (*EnumDecl, *EnumVariant, bool) {
	if pkg == nil {
		return nil, nil, false
	}
	// First check local enums
	for _, enum := range pkg.Enums {
		if variant, ok := findEnumVariant(enum, name); ok {
			return enum, variant, true
		}
	}
	// Then check dot-import enums
	if pkg.DotImportEnums != nil {
		for _, enum := range pkg.DotImportEnums {
			if variant, ok := findEnumVariant(enum, name); ok {
				return enum, variant, true
			}
		}
	}
	return nil, nil, false
}

// substituteTypeParams substitutes type parameter names in a MonoType with the
// corresponding concrete types from typeArgs, using typeParams as the mapping key.
//
// Type parameters can be simple (e.g. "A", "K") or higher-kinded (e.g. "C[A]").
//   - Simple param "A" matches TCon{Name: "A"} with zero type arguments.
//   - HKT param "C[A]" matches TCon{Name: "C"} with one or more type arguments
//     (the constructor name must match and the TCon must have args).
//
// HKT matches take priority: if "C[A]" is in typeParams, TCon{Name: "C", Args: _}
// is replaced by the corresponding typeArg even though "C" could also be a simple
// param name.
func substituteTypeParams(t MonoType, typeParams []string, typeArgs []MonoType) MonoType {
	if len(typeParams) == 0 {
		return t
	}

	// Pre-compute lookup maps.
	// hktMap: constructor name → index (last one wins if duplicates).
	hktMap := make(map[string]int)
	// simpleSet: set of simple param names (only those NOT also HKT).
	simpleSet := make(map[string]struct{})

	for i, tp := range typeParams {
		if hktName, _ := parseHKTTypeParam(tp); hktName != "" {
			hktMap[hktName] = i
		} else {
			simpleSet[tp] = struct{}{}
		}
	}

	// Early exit when there's nothing to substitute and t has no nested params.
	if len(hktMap) == 0 && len(simpleSet) == 0 {
		return t
	}

	switch t := t.(type) {
	case TCon:
		// 1) Check HKT match first (constructor name + has args).
		if idx, ok := hktMap[t.Name]; ok && len(t.Args) > 0 {
			if idx < len(typeArgs) {
				return typeArgs[idx]
			}
		}
		// 2) Check simple param match (name matches, no args).
		if _, ok := simpleSet[t.Name]; ok && len(t.Args) == 0 {
			for i, tp := range typeParams {
				if _, inner := parseHKTTypeParam(tp); inner == "" && tp == t.Name {
					if i < len(typeArgs) {
						return typeArgs[i]
					}
					break
				}
			}
		}
		// 3) Recursively substitute in type arguments.
		if len(t.Args) > 0 {
			args := make([]MonoType, len(t.Args))
			for i, a := range t.Args {
				args[i] = substituteTypeParams(a, typeParams, typeArgs)
			}
			return TCon{Name: t.Name, Args: args}
		}
		return t

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
	case TVar, TKVar:
		// Type variables produced by inference (fresh TVars) cannot be mapped
		// back to type-parameter names here.  Callers that need TVar
		// substitution should use typeFromASTWithParams instead.
		return t
	}

	return t
}

// parseHKTTypeParam parses a higher-kinded type parameter like "C[A]" or "C[K, V]".
// Returns the constructor name ("C") and a comma-separated list of inner params ("A" or "K, V").
// Returns ("", "") if the string is not a HKT type parameter.
func parseHKTTypeParam(tp string) (constructorName string, innerParams string) {
	open := strings.Index(tp, "[")
	close := strings.LastIndex(tp, "]")
	if open <= 0 || close <= open+1 || close != len(tp)-1 {
		return "", ""
	}
	return tp[:open], tp[open+1 : close]
}

// qualifyMyGoType rewrites bare type names in the given MonoType to include a
// package prefix when the type is defined in the imported MyGo package.
// This is needed because function signatures loaded from MyGo packages use
// unqualified type names (e.g. "Parser"), but the caller refers to them with
// a package prefix (e.g. "ps.Parser"). Without this rewrite, Unify would fail
// to see them as the same type.
func qualifyMyGoType(alias string, pkgTypes map[string]struct{}, t MonoType) MonoType {
	switch t := t.(type) {
	case TVar:
		return t
	case TKVar:
		return t
	case TCon:
		// Recursively qualify inner type arguments first.
		args := make([]MonoType, len(t.Args))
		for i, a := range t.Args {
			args[i] = qualifyMyGoType(alias, pkgTypes, a)
		}
		// Qualify the type constructor name if it's a bare name defined in this package.
		name := t.Name
		if name != "" && !containsDot(name) {
			if _, ok := pkgTypes[name]; ok {
				name = alias + "." + name
			}
		}
		return TCon{Name: name, Args: args}
	case TFunc:
		args := make([]MonoType, len(t.Args))
		for i, a := range t.Args {
			args[i] = qualifyMyGoType(alias, pkgTypes, a)
		}
		return TFunc{
			Args:     args,
			Ret:      qualifyMyGoType(alias, pkgTypes, t.Ret),
			Variadic: t.Variadic,
		}
	case TGoPackage:
		return t
	case TUnit:
		return t
	}
	return t
}

func containsDot(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return true
		}
	}
	return false
}
