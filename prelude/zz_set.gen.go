package prelude

func MygoIT11IEnumerableFN14SetIEnumerableGN1AEGN3SetGN1AEN1AEM3Len[A comparable](c map[A]struct{}) int {
	return len(c)
}
func MygoIT11IEnumerableFN14SetIEnumerableGN1AEGN3SetGN1AEN1AEM4Each[A comparable](c map[A]struct{}, fn func(A)) {
	func() {
		for v := range c {
			fn(v)
		}
	}()
}
func MygoIT11IEnumerableFN14SetIEnumerableGN1AEGN3SetGN1AEN1AEM3Map[A comparable, B comparable](c map[A]struct{}, fn func(A) B) map[B]struct{} {
	return func() map[B]struct{} {
		out := make(map[B]struct{}, len(c))
		for v := range c {
			out[fn(v)] = struct{}{}
		}
		return out
	}()
}
func MygoIT11IEnumerableFN14SetIEnumerableGN1AEGN3SetGN1AEN1AEM6Filter[A comparable](c map[A]struct{}, fn func(A) bool) map[A]struct{} {
	return func() map[A]struct{} {
		out := make(map[A]struct{}, len(c))
		for v := range c {
			if fn(v) {
				out[v] = struct{}{}
			}
		}
		return out
	}()
}
func MygoIT11IEnumerableFN14SetIEnumerableGN1AEGN3SetGN1AEN1AEM4Fold[A comparable, B any](c map[A]struct{}, initial B, fn func(B, A) B) B {
	return func() B {
		acc := initial
		for v := range c {
			acc = fn(acc, v)
		}
		return acc
	}()
}
func MygoIT11IEnumerableFN14SetIEnumerableGN1AEGN3SetGN1AEN1AEM4Find[A comparable](c map[A]struct{}, fn func(A) bool) Option[*A] {
	return func() Option[*A] {
		for v := range c {
			if fn(v) {
				val := v
				return Some[*A](&val)
			}
		}
		return None[*A]()
	}()
}
func MygoIT11IEnumerableFN14SetIEnumerableGN1AEGN3SetGN1AEN1AEM8Contains[A comparable](c map[A]struct{}, item A, EqualsFn func(A, A) bool) bool {
	return func() bool {
		_, ok := c[item]
		return ok
	}()
}
func MygoIN3SetM3New[A comparable]() map[A]struct{} {
	return func() map[A]struct{} {
		return make(map[A]struct{})
	}()
}
func MygoIN3SetM3Add[A comparable](items map[A]struct{}, item A) map[A]struct{} {
	return func() map[A]struct{} {
		items[item] = struct{}{}
		return items
	}()
}
func MygoIN3SetM6Delete[A comparable](items map[A]struct{}, item A) map[A]struct{} {
	return func() map[A]struct{} {
		delete(items, item)
		return items
	}()
}
