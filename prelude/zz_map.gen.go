package prelude

func MygoIT11IEnumerableFN14MapIEnumerableGN1KN1VEGN3MapGN1KN1VEN1VEM4Each[K comparable, V any](c map[K]V, fn func(V)) {
	func() {
		for _, v := range c {
			fn(v)
		}
	}()
}
func MygoIT11IEnumerableFN14MapIEnumerableGN1KN1VEGN3MapGN1KN1VEN1VEM3Len[K comparable, V any](c map[K]V) int {
	return len(c)
}
func MygoIT11IEnumerableFN14MapIEnumerableGN1KN1VEGN3MapGN1KN1VEN1VEM3Map[K comparable, V any, B any](c map[K]V, fn func(V) B) map[K]B {
	return func() map[K]B {
		out := make(map[K]B, len(c))
		for k, v := range c {
			out[k] = fn(v)
		}
		return out
	}()
}
func MygoIT11IEnumerableFN14MapIEnumerableGN1KN1VEGN3MapGN1KN1VEN1VEM6Filter[K comparable, V any](c map[K]V, fn func(V) bool) map[K]V {
	return func() map[K]V {
		out := make(map[K]V, len(c))
		for k, v := range c {
			if fn(v) {
				out[k] = v
			}
		}
		return out
	}()
}
func MygoIT11IEnumerableFN14MapIEnumerableGN1KN1VEGN3MapGN1KN1VEN1VEM4Fold[K comparable, V any, B any](c map[K]V, initial B, fn func(B, V) B) B {
	return func() B {
		acc := initial
		for _, v := range c {
			acc = fn(acc, v)
		}
		return acc
	}()
}
func MygoIT11IEnumerableFN14MapIEnumerableGN1KN1VEGN3MapGN1KN1VEN1VEM4Find[K comparable, V any](c map[K]V, fn func(V) bool) Option[*V] {
	return func() Option[*V] {
		for _, v := range c {
			if fn(v) {
				val := v
				return Some[*V](&val)
			}
		}
		return None[*V]()
	}()
}
func MygoIT11IEnumerableFN14MapIEnumerableGN1KN1VEGN3MapGN1KN1VEN1VEM8Contains[K comparable, V any](c map[K]V, item V, EqualsFn func(V, V) bool) bool {
	return func() bool {
		for _, v := range c {
			if EqualsFn(v, item) {
				return true
			}
		}
		return false
	}()
}
func MygoIT11IAssignableFN3MapGN1KN1VEGN3MapGN1KN1VEN1KN1VEM3Get[K comparable, V any](m map[K]V, index K) Option[V] {
	return func() Option[V] {
		if v, ok := m[index]; ok {
			return Some[V](v)
		}
		return None[V]()
	}()
}
func MygoIT11IAssignableFN3MapGN1KN1VEGN3MapGN1KN1VEN1KN1VEM3Set[K comparable, V any](m map[K]V, index K, value V) {
	m[index] = value
}
func MygoIT11IAssignableFN3MapGN1KN1VEGN3MapGN1KN1VEN1KN1VEM6Delete[K comparable, V any](m map[K]V, index K) {
	delete(m, index)
}
