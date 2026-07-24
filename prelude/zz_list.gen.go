package prelude

type List[A any] struct {
	Head A
	Tail Option[*List[A]]
}

func MygoIN4ListM4Cons[T any](head T, tail Option[List[T]]) List[T] {
	return List[T]{Head: head, Tail: MygoIT11IEnumerableFN17OptionIEnumerableGN1AEGN6OptionGN1AEN1AEM3Map(tail, func(a List[T]) *List[T] {
		return &a
	})}
}
func MygoIN4ListM4Head[T any](self List[T]) T {
	return self.Head
}
func MygoIN4ListM4Tail[T any](self List[T]) Option[List[T]] {
	return MygoIT11IEnumerableFN17OptionIEnumerableGN1AEGN6OptionGN1AEN1AEM3Map(self.Tail, func(a *List[T]) List[T] {
		return *a
	})
}
func MygoIT11IEnumerableFN15ListIEnumerableGN1TEGN4ListGN1TEN1TEM4Each[T any](c List[T], fn func(T)) {
	current := &c
	done := false
	for !done {
		fn(current.Head)
		if _, ok := current.Tail.(OptionNone[*List[T]]); ok {
			done = true
		} else {
			if __mygo_match___mygo_expr_0, ok := current.Tail.(OptionSome[*List[T]]); ok {
				current = __mygo_match___mygo_expr_0.F0
			} else {
			}
		}
	}
	return
}
func MygoIT11IEnumerableFN15ListIEnumerableGN1TEGN4ListGN1TEN1TEM3Len[T any](c List[T]) int {
	count := 0
	current := &c
	done := false
	for !done {
		count = count + 1
		if _, ok := current.Tail.(OptionNone[*List[T]]); ok {
			done = true
		} else {
			if __mygo_match___mygo_expr_0, ok := current.Tail.(OptionSome[*List[T]]); ok {
				current = __mygo_match___mygo_expr_0.F0
			} else {
			}
		}
	}
	return count
}
func MygoIT11IEnumerableFN15ListIEnumerableGN1TEGN4ListGN1TEN1TEM3Map[T any, B any](c List[T], fn func(T) B) List[B] {
	done := false
	headVal := fn(c.Head)
	result := List[B]{Head: headVal, Tail: None[T]()}
	current := &c
	for !done {
		if _, ok := current.Tail.(OptionNone[*List[T]]); ok {
			done = true
		} else {
			if __mygo_match___mygo_expr_0, ok := current.Tail.(OptionSome[*List[T]]); ok {
				current = __mygo_match___mygo_expr_0.F0
				result = List[B]{Head: fn(current.Head), Tail: Some(&result)}
			} else {
			}
		}
	}
	return result
}
func MygoIT11IEnumerableFN15ListIEnumerableGN1TEGN4ListGN1TEN1TEM6Filter[T any](c List[T], fn func(T) bool) List[T] {
	done := false
	current := &c
	result := List[T]{Head: current.Head, Tail: None[T]()}
	for !done {
		if fn(current.Head) {
			result = List[T]{Head: current.Head, Tail: Some(&result)}
		} else {
		}
		if _, ok := current.Tail.(OptionNone[*List[T]]); ok {
			done = true
		} else {
			if __mygo_match___mygo_expr_0, ok := current.Tail.(OptionSome[*List[T]]); ok {
				current = __mygo_match___mygo_expr_0.F0
			} else {
			}
		}
	}
	return result
}
func MygoIT11IEnumerableFN15ListIEnumerableGN1TEGN4ListGN1TEN1TEM4Fold[T any, B any](c List[T], initial B, fn func(B, T) B) B {
	acc := initial
	done := false
	current := &c
	for !done {
		acc = fn(acc, current.Head)
		if _, ok := current.Tail.(OptionNone[*List[T]]); ok {
			done = true
		} else {
			if __mygo_match___mygo_expr_0, ok := current.Tail.(OptionSome[*List[T]]); ok {
				current = __mygo_match___mygo_expr_0.F0
			} else {
			}
		}
	}
	return acc
}
func MygoIT11IEnumerableFN15ListIEnumerableGN1TEGN4ListGN1TEN1TEM4Find[T any](c List[T], fn func(T) bool) Option[*T] {
	done := false
	result := None[*T]()
	current := &c
	for !done {
		if fn(current.Head) {
			result = Some[*T](&current.Head)
			done = true
		} else {
			if _, ok := current.Tail.(OptionNone[*List[T]]); ok {
				done = true
			} else {
				if __mygo_match___mygo_expr_0, ok := current.Tail.(OptionSome[*List[T]]); ok {
					current = __mygo_match___mygo_expr_0.F0
				} else {
				}
			}
		}
	}
	return result
}
func MygoIT11IEnumerableFN15ListIEnumerableGN1TEGN4ListGN1TEN1TEM8Contains[T any](c List[T], item T, EqualsFn func(T, T) bool) bool {
	done := false
	result := false
	current := &c
	for !done {
		if EqualsFn(current.Head, item) {
			result = true
			done = true
		} else {
			if _, ok := current.Tail.(OptionNone[*List[T]]); ok {
				done = true
			} else {
				if __mygo_match___mygo_expr_0, ok := current.Tail.(OptionSome[*List[T]]); ok {
					current = __mygo_match___mygo_expr_0.F0
				} else {
				}
			}
		}
	}
	return result
}
