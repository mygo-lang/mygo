package prelude

func MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM4Each[T any](c []T, fn func(T)) {
	func() {
		for _, v := range c {
			fn(v)
		}
	}()
}
func MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM3Len[T any](c []T) int {
	return len(c)
}
func MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM3Map[T any, B any](c []T, fn func(T) B) []B {
	return func() []B {
		out := make([]B, len(c))
		for i, v := range c {
			out[i] = fn(v)
		}
		return out
	}()
}
func MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM6Filter[T any](c []T, fn func(T) bool) []T {
	return func() []T {
		out := make([]T, 0, len(c))
		for _, v := range c {
			if fn(v) {
				out = append(out, v)
			}
		}
		return out
	}()
}
func MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM4Fold[T any, B any](c []T, initial B, fn func(B, T) B) B {
	return func() B {
		acc := initial
		for _, v := range c {
			acc = fn(acc, v)
		}
		return acc
	}()
}
func MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM4Find[T any](c []T, fn func(T) bool) Option[*T] {
	return func() Option[*T] {
		for i := range c {
			if fn(c[i]) {
				return Some[*T](&c[i])
			}
		}
		return None[*T]()
	}()
}
func MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM8Contains[T any](c []T, item T, EqualsFn func(T, T) bool) bool {
	return func() bool {
		for _, v := range c {
			if EqualsFn(v, item) {
				return true
			}
		}
		return false
	}()
}
func MygoIN5SliceM6Append[T any](items []T, item T) []T {
	return func() []T {
		return append(items, item)
	}()
}
func MygoIN5SliceM7Prepend[T any](items []T, item T) []T {
	return func() []T {
		out := make([]T, 0, len(items)+1)
		out = append(out, item)
		out = append(out, items...)
		return out
	}()
}
func MygoIT10IIndexableFN14SliceIndexableGN1TEGN5SliceGN1TEN3IntN1TEM3Get[T any](s []T, index int) Option[T] {
	if index < 0 || index >= MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM3Len(s) {
		return None[T]()
	} else {
		return Some[T](s[index])
	}
}
func MygoIT10IIndexableFN14SliceIndexableGN1TEGN5SliceGN1TEN3IntN1TEM5Slice[T any](s []T, startPos int, endPos int) Option[[]T] {
	if startPos < 0 || endPos < startPos || endPos > MygoIT11IEnumerableFN16SliceIEnumerableGN1TEGN5SliceGN1TEN1TEM3Len(s) {
		return None[[]T]()
	} else {
		return Some[[]T](s[startPos:endPos])
	}
}
func MygoIT11IAssignableFN5SliceGN1TEGN5SliceGN1TEN3IntN1TEM3Get[T any](s []T, index int) Option[T] {
	return func() Option[T] {
		if index >= 0 && index < len(s) {
			return Some[T](s[index])
		}
		return None[T]()
	}()
}
func MygoIT11IAssignableFN5SliceGN1TEGN5SliceGN1TEN3IntN1TEM3Set[T any](s []T, index int, value T) {
	s[index] = value
}
func MygoIT11IAssignableFN5SliceGN1TEGN5SliceGN1TEN3IntN1TEM6Delete[T any](s []T, index int) {
	s = append(s[:index], s[index+1:]...)
}
