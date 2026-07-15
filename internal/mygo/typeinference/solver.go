package typeinference

import (
	"fmt"
	"sort"
)

// ---------------------------------------------------------------------------
// Typeclass Instance Resolution
// ---------------------------------------------------------------------------

// Instance represents a typeclass implementation for a specific type.
// Example: instance Eq[Int] { ... }
type Instance struct {
	ClassName  string      // e.g., "Eq", "ToString"
	Type       MonoType    // e.g., TCon{Name: "Int"} or TCon{Name: "Option", Args: [TVar]}
	Predicates []Predicate // super-class constraints
}

// Solver resolves typeclass predicates to instances.
type Solver struct {
	instances map[string][]*Instance // className -> list of instances
}

// NewSolver creates a new typeclass solver.
func NewSolver() *Solver {
	return &Solver{
		instances: make(map[string][]*Instance),
	}
}

// RegisterInstance registers a typeclass instance.
func (s *Solver) RegisterInstance(inst *Instance) {
	s.instances[inst.ClassName] = append(s.instances[inst.ClassName], inst)
}

// Resolve attempts to resolve a list of predicates using registered instances.
// Returns resolved types (with substitutions applied) or an error.
func (s *Solver) Resolve(preds []Predicate, subst Subst) ([]Predicate, error) {
	if len(preds) == 0 {
		return preds, nil
	}

	resolved := make([]Predicate, 0, len(preds))
	for _, pred := range preds {
		resolvedPred, err := s.resolvePredicate(pred, subst)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, resolvedPred...)
	}
	return resolved, nil
}

// resolvePredicate resolves a single predicate using instance resolution.
func (s *Solver) resolvePredicate(pred Predicate, subst Subst) ([]Predicate, error) {
	// Apply substitution to predicate arguments
	resolvedArgs := make([]MonoType, len(pred.Args))
	for i, arg := range pred.Args {
		resolvedArgs[i] = subst.ApplyMT(arg)
	}
	resolvedPred := Predicate{
		ClassName: pred.ClassName,
		Args:      resolvedArgs,
	}

	// Try to find a matching instance
	instances := s.instances[resolvedPred.ClassName]
	if len(instances) == 0 {
		// No instances registered — leave predicate unresolved
		// This allows the compiler to handle it later (e.g., code generation)
		return []Predicate{resolvedPred}, nil
	}

	// Find matching instance
	for _, inst := range instances {
		instType := subst.ApplyMT(inst.Type)
		if len(resolvedPred.Args) > 0 && unifyTypes(resolvedPred.Args[0], instType, subst) {
			// Found matching instance — check super-class constraints
			if len(inst.Predicates) > 0 {
				// Recursively resolve super-class constraints
				superPreds, err := s.Resolve(inst.Predicates, subst)
				if err != nil {
					return nil, fmt.Errorf("super-class constraint resolution failed for %s: %w", resolvedPred, err)
				}
				// Instance satisfied — no need to keep the predicate
				// (it's resolved by this instance)
				return superPreds, nil
			}
			// Instance satisfied
			return nil, nil
		}
		// Special case: match Ref[T] with Eq[Ref[A]] (any Ref type has Eq)
		if len(resolvedPred.Args) > 0 {
			if predRef, predIsRef := resolvedPred.Args[0].(TCon); predIsRef && predRef.Name == "Ref" {
				if instRef, instIsRef := instType.(TCon); instIsRef && instRef.Name == "Ref" {
					// Found Ref instance — satisfied (pointer comparison, no super-preds)
					return nil, nil
				}
			}
		}
	}

	// No matching instance found — keep predicate unresolved
	// The compiler will report an error during code generation
	return []Predicate{resolvedPred}, nil
}

// unifyTypes checks if two types can be unified (simplified version).
func unifyTypes(t1, t2 MonoType, subst Subst) bool {
	t1 = subst.ApplyMT(t1)
	t2 = subst.ApplyMT(t2)

	// Structural equality
	return eqType(t1, t2)
}

// ResolveAndGeneralize resolves predicates and generalizes a type with resolved constraints.
func (s *Solver) ResolveAndGeneralize(env TypeEnv, t MonoType, preds []Predicate) *Scheme {
	// Resolve predicates
	resolvedPreds, err := s.Resolve(preds, make(Subst))
	if err != nil {
		// If resolution fails, keep original predicates
		resolvedPreds = preds
	}

	// Generalize the type
	sch := Generalize(env, t, resolvedPreds)
	return sch
}

// ---------------------------------------------------------------------------
// Built-in Instances
// ---------------------------------------------------------------------------

// RegisterBuiltInInstances registers only Eq for primitive types and Ref.
// All other typeclass instances (ToString, IEnumerable, etc.) are registered
// dynamically when their impl declarations are discovered during inference.
func RegisterBuiltInInstances() map[string]*Instance {
	instances := []*Instance{
		// Eq instances for primitive types — these are always available
		{ClassName: "Eq", Type: TCon{Name: "Int"}},
		{ClassName: "Eq", Type: TCon{Name: "Int8"}},
		{ClassName: "Eq", Type: TCon{Name: "Int16"}},
		{ClassName: "Eq", Type: TCon{Name: "Int32"}},
		{ClassName: "Eq", Type: TCon{Name: "Int64"}},
		{ClassName: "Eq", Type: TCon{Name: "UInt8"}},
		{ClassName: "Eq", Type: TCon{Name: "UInt16"}},
		{ClassName: "Eq", Type: TCon{Name: "UInt32"}},
		{ClassName: "Eq", Type: TCon{Name: "UInt64"}},
		{ClassName: "Eq", Type: TCon{Name: "Float32"}},
		{ClassName: "Eq", Type: TCon{Name: "Float64"}},
		{ClassName: "Eq", Type: TCon{Name: "String"}},
		{ClassName: "Eq", Type: TCon{Name: "Bool"}},
		{ClassName: "Eq", Type: TCon{Name: "Rune"}},
		{ClassName: "Eq", Type: TCon{Name: "Byte"}},
		// Eq for Ref — pointer comparison (address equality)
		{ClassName: "Eq", Type: TCon{Name: "Ref", Args: []MonoType{TVar{ID: 999}}}},
	}

	result := make(map[string]*Instance)
	for _, inst := range instances {
		key := inst.ClassName + "[" + inst.Type.String() + "]"
		if _, ok := result[key]; !ok {
			result[key] = inst
		}
	}
	return result
}

// RegisterImplInstance registers a typeclass instance discovered from an impl declaration.
// This is called when processing ImplDecl nodes during inference.
func (s *Solver) RegisterImplInstance(className string, typ MonoType, superPreds []Predicate) {
	s.instances[className] = append(s.instances[className], &Instance{
		ClassName:  className,
		Type:       typ,
		Predicates: superPreds,
	})
}

// HasEqInstance checks whether the given MonoType has a registered Eq instance.
// It recursively checks: primitive types always have Eq, Ref[T] always has Eq
// (pointer comparison), and struct types check if an Eq instance was registered.
func (s *Solver) HasEqInstance(t MonoType) bool {
	t = applyNoop(t) // no-op apply since we don't have a subst here
	switch con := t.(type) {
	case TCon:
		// Primitive types always have Eq
		switch con.Name {
		case "Int", "Int8", "Int16", "Int32", "Int64",
			"UInt", "UInt8", "UInt16", "UInt32", "UInt64",
			"Float32", "Float64", "String", "Bool", "Byte", "Rune":
			return true
		}
		// Ref[T] always has Eq — pointer comparison (address equality)
		if con.Name == "Ref" {
			return true
		}
		// Check if an Eq instance is registered for this type
		if instances, ok := s.instances["Eq"]; ok {
			for _, inst := range instances {
				if eqType(inst.Type, con) {
					return true
				}
			}
		}
		return false
	case TVar:
		// Type variables are not resolved yet — assume they may have Eq
		// (they will be resolved later during constraint solving)
		return true
	}
	return false
}

// applyNoop is a no-op helper since MonoType.ApplyMT doesn't need a subst here.
func applyNoop(t MonoType) MonoType { return t }

// ---------------------------------------------------------------------------
// Constraint Solving with Occurs Check
// ---------------------------------------------------------------------------

// SolveConstraints attempts to solve a list of constraints using iterative unification.
// This implements a simple constraint solver for HM + typeclasses.
func SolveConstraints(preds []Predicate, env TypeEnv, solver *Solver) ([]Predicate, Subst, error) {
	subst := make(Subst)
	remaining := preds

	// Iteratively resolve constraints
	maxIterations := 100
	for i := 0; i < maxIterations && len(remaining) > 0; i++ {
		resolved := make([]Predicate, 0)
		unresolved := make([]Predicate, 0)

		for _, pred := range remaining {
			// Check if all type arguments are fully resolved (no type variables)
			if isFullyResolved(pred.Args, subst) {
				// Try to resolve with instances
				resolvedPreds, err := solver.resolvePredicate(pred, subst)
				if err != nil {
					return nil, nil, err
				}
				resolved = append(resolved, resolvedPreds...)
			} else {
				// Still has type variables — keep for next iteration
				unresolved = append(unresolved, pred)
			}
		}

		if len(unresolved) == len(remaining) {
			// No progress made — stop
			break
		}
		remaining = unresolved
	}

	return remaining, subst, nil
}

// isFullyResolved checks if all types in the arguments are fully resolved
// (contain no type variables or only concrete types).
func isFullyResolved(types []MonoType, subst Subst) bool {
	for _, t := range types {
		resolved := subst.ApplyMT(t)
		if hasTypeVariables(resolved) {
			return false
		}
	}
	return true
}

// hasTypeVariables checks if a type contains any type variables.
func hasTypeVariables(t MonoType) bool {
	switch t := t.(type) {
	case TVar:
		return true
	case TCon:
		for _, arg := range t.Args {
			if hasTypeVariables(arg) {
				return true
			}
		}
		return false
	case TFunc:
		for _, arg := range t.Args {
			if hasTypeVariables(arg) {
				return true
			}
		}
		return hasTypeVariables(t.Ret)
	case TGoPackage, TUnit:
		return false
	}
	return false
}

// ---------------------------------------------------------------------------
// Type Defaulting
// ---------------------------------------------------------------------------

// DefaultNumericTypes applies default numeric types based on context.
// In Haskell, ambiguous numeric types default to Int or Float.
func DefaultNumericTypes(t MonoType, subst Subst) MonoType {
	// Check if type is an ambiguous numeric type variable
	if varType, ok := t.(TVar); ok {
		if isNumericTypeVariable(varType, subst) {
			// Default to Int for integer contexts
			return TCon{Name: "Int"}
		}
	}

	// Recursively default in type constructor arguments
	if con, ok := t.(TCon); ok {
		defaultedArgs := make([]MonoType, len(con.Args))
		for i, arg := range con.Args {
			defaultedArgs[i] = DefaultNumericTypes(arg, subst)
		}
		return TCon{Name: con.Name, Args: defaultedArgs}
	}

	if fn, ok := t.(TFunc); ok {
		defaultedArgs := make([]MonoType, len(fn.Args))
		for i, arg := range fn.Args {
			defaultedArgs[i] = DefaultNumericTypes(arg, subst)
		}
		return TFunc{
			Args:     defaultedArgs,
			Ret:      DefaultNumericTypes(fn.Ret, subst),
			Variadic: fn.Variadic,
		}
	}

	return t
}

// isNumericTypeVariable checks if a type variable should default to a numeric type.
func isNumericTypeVariable(tv TVar, subst Subst) bool {
	// Check if the type variable is bound to a numeric type
	if replacement, ok := subst[tv.ID]; ok {
		if con, ok := replacement.(TCon); ok {
			return isNumericType(con)
		}
	}
	// If not substituted yet, check if it's used in a numeric context
	// For now, we assume ambiguous numeric types should default to Int
	return true
}

// isNumericType checks if a type is a numeric type.
func isNumericType(t TCon) bool {
	numericTypes := []string{
		"Int", "Int8", "Int16", "Int32", "Int64",
		"UInt8", "UInt16", "UInt32", "UInt64",
		"Float32", "Float64",
	}
	for _, name := range numericTypes {
		if t.Name == name {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Polymorphic Instance Resolution
// ---------------------------------------------------------------------------

// ResolvePolymorphic attempts to resolve a predicate with polymorphic types.
// This handles cases like Eq[Option[Int]] where Option is a parameterized type.
func (s *Solver) ResolvePolymorphic(pred Predicate, subst Subst) ([]Predicate, error) {
	// Apply substitution
	resolvedArgs := make([]MonoType, len(pred.Args))
	for i, arg := range pred.Args {
		resolvedArgs[i] = subst.ApplyMT(arg)
	}

	// Check if this is a parameterized type
	if len(resolvedArgs) > 0 {
		if con, ok := resolvedArgs[0].(TCon); ok {
			// For parameterized types, check if we have instances for the base type
			// Example: Eq[Option[A]] should match instance Eq[Option[A]]
			instances := s.instances[pred.ClassName]
			for _, inst := range instances {
				instType := subst.ApplyMT(inst.Type)
				if conType, ok := instType.(TCon); ok && conType.Name == con.Name {
					// Check if type arguments match
					if len(con.Args) == len(conType.Args) {
						match := true
						for i := range con.Args {
							if !eqType(con.Args[i], conType.Args[i]) {
								match = false
								break
							}
						}
						if match {
							// Instance matches — return any super-class constraints
							if len(inst.Predicates) > 0 {
								return s.Resolve(inst.Predicates, subst)
							}
							return nil, nil
						}
					}
				}
			}
		}
	}

	// No matching instance — return unresolved
	return []Predicate{pred}, nil
}

// ---------------------------------------------------------------------------
// Utility Functions
// ---------------------------------------------------------------------------

// GetResolvedTypes extracts the resolved types from a list of predicates.
func GetResolvedTypes(preds []Predicate) []MonoType {
	types := make([]MonoType, 0, len(preds))
	for _, pred := range preds {
		if len(pred.Args) > 0 {
			types = append(types, pred.Args[0])
		}
	}
	return types
}

// GetUniqueClassNames returns unique typeclass names from a list of predicates.
func GetUniqueClassNames(preds []Predicate) []string {
	names := make(map[string]bool)
	result := make([]string, 0)
	for _, pred := range preds {
		if !names[pred.ClassName] {
			names[pred.ClassName] = true
			result = append(result, pred.ClassName)
		}
	}
	sort.Strings(result)
	return result
}
