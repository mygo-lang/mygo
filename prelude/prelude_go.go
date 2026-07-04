package prelude

// Type definitions matching the compiler-generated output from prelude.mygo.
// These are duplicated here so prelude_go.go can compile independently
// (before zz_mygo.gen.go is generated).

// Option[A] is a generic optional value enum.
type Option[A any] interface {
	isOption()
}

// OptionSome[A] is the Some variant of Option.
type OptionSome[A any] struct {
	F0 A
}

func (OptionSome[A]) isOption() {}

// Some constructs an Option with a value.
func Some[A any](a0 A) Option[A] { return OptionSome[A]{F0: a0} }

// OptionNone[A] is the None variant of Option.
type OptionNone[A any] struct{}

func (OptionNone[A]) isOption() {}

// None constructs an Option with no value.
func None[A any]() Option[A] { return OptionNone[A]{} }

// Eq[A] is the equality typeclass interface.
type Eq[A any] interface {
	equals(left, right A) bool
}

// === Slice ([]) Enumerable Go helpers ===

// eachSlice calls fn for each element of s.
func eachSlice[T any](s []T, fn func(T)) {
	for _, v := range s {
		fn(v)
	}
}

// mapSlice creates a new slice by applying fn to each element of s.
func mapSlice[T, B any](s []T, fn func(T) B) []B {
	out := make([]B, len(s))
	for i, v := range s {
		out[i] = fn(v)
	}
	return out
}

// filterSlice returns a new slice with elements for which fn returns true.
func filterSlice[T any](s []T, fn func(T) bool) []T {
	out := make([]T, 0, len(s))
	for _, v := range s {
		if fn(v) {
			out = append(out, v)
		}
	}
	return out
}

// foldSlice reduces s to a single value using fn.
func foldSlice[T, B any](s []T, initial B, fn func(B, T) B) B {
	acc := initial
	for _, v := range s {
		acc = fn(acc, v)
	}
	return acc
}

// findSlice returns Some(&s[i]) for the first element matching fn, or None.
func findSlice[T any](s []T, fn func(T) bool) Option[*T] {
	for i := range s {
		if fn(s[i]) {
			return Some[*T](&s[i])
		}
	}
	return None[*T]()
}

// containsSlice checks if any element in s satisfies eq.equals against item.
func containsSlice[T any](s []T, item T, eq Eq[T]) bool {
	for _, v := range s {
		if eq.equals(v, item) {
			return true
		}
	}
	return false
}

// === Map[K,V] generic Enumerable Go helpers (K comparable) ===

// eachMap calls fn for each value in m.
func eachMap[K comparable, V any](m map[K]V, fn func(V)) {
	for _, v := range m {
		fn(v)
	}
}

// mapMap creates a new map[K]B by applying fn to each value of m.
func mapMap[K comparable, V, B any](m map[K]V, fn func(V) B) map[K]B {
	out := make(map[K]B, len(m))
	for k, v := range m {
		out[k] = fn(v)
	}
	return out
}

// filterMap returns a new map[K]V with values for which fn returns true.
func filterMap[K comparable, V any](m map[K]V, fn func(V) bool) map[K]V {
	out := make(map[K]V, len(m))
	for k, v := range m {
		if fn(v) {
			out[k] = v
		}
	}
	return out
}

// foldMap reduces a map to a single value using fn.
func foldMap[K comparable, V, B any](m map[K]V, initial B, fn func(B, V) B) B {
	acc := initial
	for _, v := range m {
		acc = fn(acc, v)
	}
	return acc
}

// findMap returns Some(&v) for the first value matching fn, or None.
func findMap[K comparable, V any](m map[K]V, fn func(V) bool) Option[*V] {
	for _, v := range m {
		if fn(v) {
			value := v
			return Some[*V](&value)
		}
	}
	return None[*V]()
}

// containsMap checks if any value in m satisfies eq.equals against item.
func containsMap[K comparable, V any](m map[K]V, item V, eq Eq[V]) bool {
	for _, v := range m {
		if eq.equals(v, item) {
			return true
		}
	}
	return false
}

// === Set[A] Enumerable Go helpers ===

// eachSet calls fn for each element in the set.
func eachSet[T comparable](s map[T]struct{}, fn func(T)) {
	for v := range s {
		fn(v)
	}
}

// mapSet creates a new set[B] by applying fn to each element of s.
func mapSet[T, B comparable](s map[T]struct{}, fn func(T) B) map[B]struct{} {
	out := make(map[B]struct{}, len(s))
	for v := range s {
		out[fn(v)] = struct{}{}
	}
	return out
}

// filterSet returns a new set with elements for which fn returns true.
func filterSet[T comparable](s map[T]struct{}, fn func(T) bool) map[T]struct{} {
	out := make(map[T]struct{}, len(s))
	for v := range s {
		if fn(v) {
			out[v] = struct{}{}
		}
	}
	return out
}

// foldSet reduces a set to a single value using fn.
func foldSet[T comparable, B any](s map[T]struct{}, initial B, fn func(B, T) B) B {
	acc := initial
	for v := range s {
		acc = fn(acc, v)
	}
	return acc
}

// findSet returns Some(&v) for the first element matching fn, or None.
func findSet[T comparable](s map[T]struct{}, fn func(T) bool) Option[*T] {
	for v := range s {
		if fn(v) {
			value := v
			return Some[*T](&value)
		}
	}
	return None[*T]()
}

// containsSet checks if item satisfies eq.equals against any element in s.
func containsSet[T comparable](s map[T]struct{}, item T, eq Eq[T]) bool {
	for v := range s {
		if eq.equals(v, item) {
			return true
		}
	}
	return false
}
