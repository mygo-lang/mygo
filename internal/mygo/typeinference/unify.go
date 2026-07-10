package typeinference

import (
	"fmt"
	"strings"
)

// Unify performs unification of two monotypes under the given substitution,
// returning a new (possibly extended) substitution. This is the core of
// Algorithm W's equation-solving step.
func Unify(t1, t2 MonoType, s Subst) (Subst, error) {
	t1 = s.ApplyMT(t1)
	t2 = s.ApplyMT(t2)

	if eqType(t1, t2) {
		return s, nil
	}

	switch t1 := t1.(type) {
	case TVar:
		return bindVar(t1.ID, t2, s)
	case TCon:
		if t1.Name == "Any" || t1.Name == "any" {
			return s, nil
		}
		switch t2 := t2.(type) {
		case TVar:
			return bindVar(t2.ID, t1, s)
		case TCon:
			if t2.Name == "Any" || t2.Name == "any" {
				return s, nil
			}
			if isNumericConName(t1.Name) && isNumericConName(t2.Name) {
				return s, nil
			}
			if t1.Name != t2.Name || len(t1.Args) != len(t2.Args) {
				return nil, fmt.Errorf("cannot unify %s with %s", t1, t2)
			}
			for i := range t1.Args {
				var err error
				s, err = Unify(t1.Args[i], t2.Args[i], s)
				if err != nil {
					return nil, err
				}
			}
			return s, nil
		case TFunc:
			return nil, fmt.Errorf("cannot unify type constructor %s with function type", t1)
		case TGoPackage:
			return nil, fmt.Errorf("cannot unify type constructor %s with Go package %s", t1, t2)
		case TUnit:
			return nil, fmt.Errorf("cannot unify type constructor %s with Unit", t1)
		}
	case TFunc:
		switch t2 := t2.(type) {
		case TVar:
			return bindVar(t2.ID, t1, s)
		case TFunc:
			if t1.Variadic != t2.Variadic {
				return nil, fmt.Errorf("cannot unify variadic and non-variadic function types: %s vs %s", t1, t2)
			}
			if len(t1.Args) != len(t2.Args) {
				return nil, fmt.Errorf("cannot unify function types with different arities: %s vs %s", t1, t2)
			}
			for i := range t1.Args {
				var err error
				s, err = Unify(t1.Args[i], t2.Args[i], s)
				if err != nil {
					return nil, err
				}
			}
			return Unify(t1.Ret, t2.Ret, s)
		case TCon:
			return nil, fmt.Errorf("cannot unify function type with type constructor %s", t2)
		case TGoPackage:
			return nil, fmt.Errorf("cannot unify function type with Go package %s", t2)
		case TUnit:
			return nil, fmt.Errorf("cannot unify function type with Unit")
		}
	case TGoPackage:
		switch t2 := t2.(type) {
		case TVar:
			return bindVar(t2.ID, t1, s)
		case TGoPackage:
			if t1.Alias == t2.Alias {
				return s, nil
			}
			return nil, fmt.Errorf("cannot unify Go package %s with %s", t1, t2)
		default:
			return nil, fmt.Errorf("cannot unify Go package %s with %s", t1, t2)
		}
	case TUnit:
		switch t2.(type) {
		case TVar:
			return bindVar(t2.(TVar).ID, t1, s)
		default:
			return nil, fmt.Errorf("cannot unify Unit with %s", t2)
		}
	}

	return nil, fmt.Errorf("unexpected unification: %s vs %s", t1, t2)
}

func isNumericConName(name string) bool {
	switch strings.TrimSpace(name) {
	case "Int", "Int8", "Int16", "Int32", "Int64", "UInt", "UInt8", "UInt16", "UInt32", "UInt64", "Float32", "Float64",
		"int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64":
		return true
	default:
		return false
	}
}

// bindVar binds a type variable to a type, performing an occurs check first.
func bindVar(id int, t MonoType, s Subst) (Subst, error) {
	if _, ok := t.(TVar); ok && t.(TVar).ID == id {
		return s, nil
	}
	if occursIn(id, t) {
		return nil, fmt.Errorf("occurs check failed: cannot bind t%d in %s", id, t)
	}
	s = s.Clone()
	s[id] = t
	return s, nil
}

// occursIn checks whether a type variable with the given ID occurs in t.
func occursIn(id int, t MonoType) bool {
	switch t := t.(type) {
	case TVar:
		return t.ID == id
	case TCon:
		for _, a := range t.Args {
			if occursIn(id, a) {
				return true
			}
		}
		return false
	case TFunc:
		for _, a := range t.Args {
			if occursIn(id, a) {
				return true
			}
		}
		return occursIn(id, t.Ret)
	case TGoPackage:
		return false
	case TUnit:
		return false
	}
	return false
}

// eqType checks structural equality of two monotypes (ignoring substitution).
func eqType(a, b MonoType) bool {
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
			if !eqType(a.Args[i], b.Args[i]) {
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
			if !eqType(a.Args[i], b.Args[i]) {
				return false
			}
		}
		return eqType(a.Ret, b.Ret)
	case TGoPackage:
		b, ok := b.(TGoPackage)
		return ok && a.Alias == b.Alias
	case TUnit:
		_, ok := b.(TUnit)
		return ok
	}
	return false
}
