package typeinference

import "fmt"

// Unify performs unification of two monotypes under the given substitution,
// returning a new (possibly extended) substitution. This is the core of
// Algorithm W's equation-solving step.
func Unify(t1, t2 MonoType, s Subst) (Subst, error) {
	t1 = s.ApplyMT(t1)
	t2 = s.ApplyMT(t2)

	if eqType(t1, t2) {
		return s, nil
	}
	if isRuneAliasPair(t1, t2) {
		return s, nil
	}
	if s2, ok, err := unifyStringRuneSequence(t1, t2, s); ok || err != nil {
		return s2, err
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

func isRuneAliasPair(t1, t2 MonoType) bool {
	c1, ok1 := t1.(TCon)
	c2, ok2 := t2.(TCon)
	if !ok1 || !ok2 || len(c1.Args) != 0 || len(c2.Args) != 0 {
		return false
	}
	return (c1.Name == "rune" && c2.Name == "Int32") || (c1.Name == "Int32" && c2.Name == "rune")
}

// unifyStringRuneSequence teaches higher-kinded collection constraints that
// String is a sequence of rune values. Interface methods such as
// IEnumerable[C[A], A].Len accept C[A]; a receiver of type String should
// therefore solve A as rune instead of failing structural unification against
// the abstract C[A] shape.
func unifyStringRuneSequence(t1, t2 MonoType, s Subst) (Subst, bool, error) {
	if elem, ok := stringSequenceElem(t1, t2); ok {
		s2, err := Unify(elem, TCon{Name: "rune"}, s)
		return s2, true, err
	}
	if elem, ok := stringSequenceElem(t2, t1); ok {
		s2, err := Unify(elem, TCon{Name: "rune"}, s)
		return s2, true, err
	}
	return s, false, nil
}

func stringSequenceElem(pattern, actual MonoType) (MonoType, bool) {
	p, ok := pattern.(TCon)
	if !ok || p.Name != "C" || len(p.Args) != 1 {
		return nil, false
	}
	a, ok := actual.(TCon)
	if !ok || a.Name != "String" || len(a.Args) != 0 {
		return nil, false
	}
	return p.Args[0], true
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
