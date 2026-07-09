package typeinference

// Package typeinference implements Hindley-Milner type inference (Algorithm W)
// with let-polymorphism, occurs check, and qualified types for typeclass constraints,
// following the Haskell 98 core HM design.

import (
	"fmt"
	"strings"

	. "github.com/mygo-lang/mygo/internal/mygo/ast"
)

// ---------------------------------------------------------------------------
// Internal type representation
// ---------------------------------------------------------------------------

// MonoType is the internal representation of a monomorphic type.
type MonoType interface {
	monoType()
	String() string
}

type (
	// TVar is a type variable identified by a unique integer.
	TVar struct{ ID int }
	// TCon is a named type constructor applied to zero or more type arguments.
	TCon struct {
		Name string
		Args []MonoType
	}
	// TFunc is a function type from argument types to a return type.
	TFunc struct {
		Args     []MonoType
		Ret      MonoType
		Variadic bool
	}
	// TGoPackage is an imported Go package alias visible to selector inference.
	TGoPackage struct{ Alias string }
	// TUnit is the unit type (empty tuple), used for void-returning functions.
	TUnit struct{}
)

func (TVar) monoType()       {}
func (TCon) monoType()       {}
func (TFunc) monoType()      {}
func (TGoPackage) monoType() {}
func (TUnit) monoType()      {}

func (t TVar) String() string { return fmt.Sprintf("t%d", t.ID) }
func (t TCon) String() string {
	if len(t.Args) == 0 {
		return t.Name
	}
	args := make([]string, len(t.Args))
	for i, a := range t.Args {
		args[i] = a.String()
	}
	return t.Name + "[" + strings.Join(args, ", ") + "]"
}
func (t TFunc) String() string {
	args := make([]string, len(t.Args))
	for i, a := range t.Args {
		if t.Variadic && i == len(t.Args)-1 {
			args[i] = "..." + a.String()
			continue
		}
		args[i] = a.String()
	}
	ret := t.Ret.String()
	if len(args) == 1 {
		return args[0] + " -> " + ret
	}
	return "(" + strings.Join(args, ", ") + ") -> " + ret
}
func (t TGoPackage) String() string { return "go package " + t.Alias }
func (TUnit) String() string        { return "Unit" }

// Predicate represents a typeclass constraint, e.g. Show[Int].
type Predicate struct {
	ClassName string
	Args      []MonoType
}

func (p Predicate) String() string {
	args := make([]string, len(p.Args))
	for i, a := range p.Args {
		args[i] = a.String()
	}
	return p.ClassName + "[" + strings.Join(args, ", ") + "]"
}

// QualifiedType is a type qualified by a list of predicates.
type QualifiedType struct {
	Predicates []Predicate
	Body       MonoType
}

func (q QualifiedType) String() string {
	if len(q.Predicates) == 0 {
		return q.Body.String()
	}
	preds := make([]string, len(q.Predicates))
	for i, p := range q.Predicates {
		preds[i] = p.String()
	}
	return "(" + strings.Join(preds, ", ") + ") => " + q.Body.String()
}

// Scheme represents a polymorphic type: Forall[bound]. qualifiedType
type Scheme struct {
	Bound []int // free var IDs that are bound by this scheme
	Body  QualifiedType
}

func (s Scheme) String() string {
	if len(s.Bound) == 0 {
		return s.Body.String()
	}
	vars := make([]string, len(s.Bound))
	for i, id := range s.Bound {
		vars[i] = fmt.Sprintf("t%d", id)
	}
	return "forall " + strings.Join(vars, " ") + ". " + s.Body.String()
}

// Subst is a substitution mapping from type variable IDs to MonoType.
type Subst map[int]MonoType

func (s Subst) Clone() Subst {
	dup := make(Subst, len(s))
	for k, v := range s {
		dup[k] = v
	}
	return dup
}

// TypeEnv maps variable names to type schemes.
type TypeEnv map[string]*Scheme

func (env TypeEnv) Clone() TypeEnv {
	dup := make(TypeEnv, len(env))
	for k, v := range env {
		dup[k] = v
	}
	return dup
}

// InferState holds inference state: fresh variable counter and accumulated constraints.
type InferState struct {
	FreshVarID       int
	PkgInfo          *PkgInfo // package info for enum/struct variant lookups
	GoPackages       map[string]*GoPackageInfo
	MyGoPackages     map[string]*MyGoPackageInfo
	MyGoPackageCache map[string]*MyGoPackageInfo
	TypedInfo        *TypedInfo
}

type MyGoPackageInfo struct {
	Alias string
	Path  string
	Name  string
	Funcs map[string]TFunc
	Types map[string]struct{}
}

func NewInferState() *InferState {
	return &InferState{
		FreshVarID:       1,
		GoPackages:       map[string]*GoPackageInfo{},
		MyGoPackages:     map[string]*MyGoPackageInfo{},
		MyGoPackageCache: map[string]*MyGoPackageInfo{},
	}
}

func (s *InferState) Fresh() int {
	id := s.FreshVarID
	s.FreshVarID++
	return id
}

// ---------------------------------------------------------------------------
// Free type variables
// ---------------------------------------------------------------------------

func freeVarsMT(t MonoType) []int {
	seen := map[int]struct{}{}
	var walk func(MonoType)
	walk = func(t MonoType) {
		switch t := t.(type) {
		case TVar:
			if _, ok := seen[t.ID]; !ok {
				seen[t.ID] = struct{}{}
			}
		case TCon:
			for _, a := range t.Args {
				walk(a)
			}
		case TFunc:
			for _, a := range t.Args {
				walk(a)
			}
			walk(t.Ret)
		case TGoPackage:
		case TUnit:
		}
	}
	walk(t)
	out := make([]int, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	return out
}

func freeVarsQual(q QualifiedType) []int {
	seen := map[int]struct{}{}
	for _, p := range q.Predicates {
		for _, a := range p.Args {
			for _, id := range freeVarsMT(a) {
				seen[id] = struct{}{}
			}
		}
	}
	for _, id := range freeVarsMT(q.Body) {
		seen[id] = struct{}{}
	}
	out := make([]int, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	return out
}

func freeVarsScheme(s *Scheme) []int {
	bound := map[int]struct{}{}
	for _, id := range s.Bound {
		bound[id] = struct{}{}
	}
	all := freeVarsQual(s.Body)
	out := make([]int, 0, len(all))
	for _, id := range all {
		if _, ok := bound[id]; !ok {
			out = append(out, id)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Substitution application
// ---------------------------------------------------------------------------

func (s Subst) ApplyMT(t MonoType) MonoType {
	switch t := t.(type) {
	case TVar:
		if replacement, ok := s[t.ID]; ok {
			// Avoid infinite recursion on self-referencing substitutions.
			if r2, ok2 := replacement.(TVar); ok2 && r2.ID == t.ID {
				return t
			}
			return s.ApplyMT(replacement)
		}
		return t
	case TCon:
		args := make([]MonoType, len(t.Args))
		for i, a := range t.Args {
			args[i] = s.ApplyMT(a)
		}
		return TCon{Name: t.Name, Args: args}
	case TFunc:
		args := make([]MonoType, len(t.Args))
		for i, a := range t.Args {
			args[i] = s.ApplyMT(a)
		}
		return TFunc{Args: args, Ret: s.ApplyMT(t.Ret), Variadic: t.Variadic}
	case TGoPackage:
		return t
	case TUnit:
		return t
	}
	return t
}

func (s Subst) ApplyPred(p Predicate) Predicate {
	args := make([]MonoType, len(p.Args))
	for i, a := range p.Args {
		args[i] = s.ApplyMT(a)
	}
	return Predicate{ClassName: p.ClassName, Args: args}
}

func (s Subst) ApplyQual(q QualifiedType) QualifiedType {
	preds := make([]Predicate, len(q.Predicates))
	for i, p := range q.Predicates {
		preds[i] = s.ApplyPred(p)
	}
	return QualifiedType{Predicates: preds, Body: s.ApplyMT(q.Body)}
}

func (s Subst) ApplyScheme(sch *Scheme) *Scheme {
	// Remove bound vars from substitution before applying
	filtered := make(Subst)
	for k, v := range s {
		isBound := false
		for _, b := range sch.Bound {
			if k == b {
				isBound = true
				break
			}
		}
		if !isBound {
			filtered[k] = v
		}
	}
	return &Scheme{
		Bound: sch.Bound,
		Body:  filtered.ApplyQual(sch.Body),
	}
}

// Compose combines two substitutions: s2 ∘ s1, applying s2 after s1.
func Compose(s1, s2 Subst) Subst {
	result := make(Subst)
	for k, v := range s1 {
		result[k] = s2.ApplyMT(v)
	}
	for k, v := range s2 {
		if _, ok := result[k]; !ok {
			result[k] = v
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Lookup and environment helpers
// ---------------------------------------------------------------------------

// LookupVar looks up a variable in the type environment and instantiates it.
func (env TypeEnv) LookupVar(name string, state *InferState) (MonoType, error) {
	sch, ok := env[name]
	if !ok {
		return nil, fmt.Errorf("unbound variable %q", name)
	}
	return Instantiate(sch, state), nil
}

// Extend adds a binding to the type environment.
func (env TypeEnv) Extend(name string, sch *Scheme) TypeEnv {
	dup := env.Clone()
	dup[name] = sch
	return dup
}

// ---------------------------------------------------------------------------
// Instantiate and Generalize
// ---------------------------------------------------------------------------

// Instantiate replaces all bound type variables of a scheme with fresh ones.
func Instantiate(sch *Scheme, state *InferState) MonoType {
	subst := make(Subst)
	for _, id := range sch.Bound {
		subst[id] = TVar{ID: state.Fresh()}
	}
	return subst.ApplyMT(sch.Body.Body)
}

// Generalize generalizes a monotype into a scheme by quantifying over free
// type variables that are not free in the environment.
func Generalize(env TypeEnv, t MonoType, preds []Predicate) *Scheme {
	envFree := map[int]struct{}{}
	for _, sch := range env {
		for _, id := range freeVarsScheme(sch) {
			envFree[id] = struct{}{}
		}
	}
	bodyFree := map[int]struct{}{}
	for _, id := range freeVarsMT(t) {
		bodyFree[id] = struct{}{}
	}
	for _, p := range preds {
		for _, a := range p.Args {
			for _, id := range freeVarsMT(a) {
				bodyFree[id] = struct{}{}
			}
		}
	}

	bound := make([]int, 0)
	for id := range bodyFree {
		if _, ok := envFree[id]; !ok {
			bound = append(bound, id)
		}
	}

	return &Scheme{
		Bound: bound,
		Body: QualifiedType{
			Predicates: preds,
			Body:       t,
		},
	}
}

// ---------------------------------------------------------------------------
// Conversion to/from AST types
// ---------------------------------------------------------------------------

// typeFromAST converts an AST TypeExpr into a MonoType.
func typeFromAST(t TypeExpr) MonoType {
	switch t := t.(type) {
	case *NamedType:
		args := make([]MonoType, len(t.Args))
		for i, a := range t.Args {
			args[i] = typeFromAST(a)
		}
		return TCon{Name: t.Name, Args: args}
	case *FuncType:
		params := make([]MonoType, len(t.Params))
		for i, p := range t.Params {
			params[i] = typeFromAST(p)
		}
		ret := typeFromAST(t.Ret)
		return TFunc{Args: params, Ret: ret}
	case *TupleType:
		if len(t.Elems) == 0 {
			return TUnit{}
		}
		args := make([]MonoType, len(t.Elems))
		for i, e := range t.Elems {
			args[i] = typeFromAST(e)
		}
		return TCon{Name: "Tuple", Args: args}
	}
	return TCon{Name: "unknown"}
}

// returnType extracts the return type from a Monotype (for function types).
func returnType(t MonoType) MonoType {
	switch t := t.(type) {
	case TFunc:
		return t.Ret
	case TUnit:
		return TUnit{}
	}
	return t
}
