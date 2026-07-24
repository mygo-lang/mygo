package prelude

func MygoIT11IEnumerableFN17OptionIEnumerableGN1AEGN6OptionGN1AEN1AEM4Each[A any](c Option[A], fn func(A)) {
	if __mygo_match___mygo_expr_0, ok := c.(OptionSome[A]); ok {
		fn(__mygo_match___mygo_expr_0.F0)
	} else {
		if _, ok := c.(OptionNone[A]); ok {
		} else {
		}
	}
	return
}
func MygoIT11IEnumerableFN17OptionIEnumerableGN1AEGN6OptionGN1AEN1AEM3Map[A any, B any](c Option[A], fn func(A) B) Option[B] {
	var __mygo_expr_0 Option[B]
	if __mygo_match___mygo_expr_1, ok := c.(OptionSome[A]); ok {
		__mygo_expr_0 = Some[B](fn(__mygo_match___mygo_expr_1.F0))
	} else {
		if _, ok := c.(OptionNone[A]); ok {
			__mygo_expr_0 = None[B]()
		} else {
		}
	}
	return __mygo_expr_0
}
func MygoIT11IEnumerableFN17OptionIEnumerableGN1AEGN6OptionGN1AEN1AEM6Filter[A any](c Option[A], fn func(A) bool) Option[A] {
	var __mygo_expr_0 Option[A]
	if __mygo_match___mygo_expr_1, ok := c.(OptionSome[A]); ok {
		var __mygo_expr_2 Option[A]
		if fn(__mygo_match___mygo_expr_1.F0) {
			__mygo_expr_2 = Some[A](__mygo_match___mygo_expr_1.F0)
		} else {
			__mygo_expr_2 = None[A]()
		}
		__mygo_expr_0 = __mygo_expr_2
	} else {
		if _, ok := c.(OptionNone[A]); ok {
			__mygo_expr_0 = None[A]()
		} else {
		}
	}
	return __mygo_expr_0
}
func MygoIT11IEnumerableFN17OptionIEnumerableGN1AEGN6OptionGN1AEN1AEM4Fold[A any, B any](c Option[A], initial B, fn func(B, A) B) B {
	var __mygo_expr_0 B
	if __mygo_match___mygo_expr_1, ok := c.(OptionSome[A]); ok {
		__mygo_expr_0 = fn(initial, __mygo_match___mygo_expr_1.F0)
	} else {
		if _, ok := c.(OptionNone[A]); ok {
			__mygo_expr_0 = initial
		} else {
		}
	}
	return __mygo_expr_0
}
func MygoIT11IEnumerableFN17OptionIEnumerableGN1AEGN6OptionGN1AEN1AEM4Find[A any](c Option[A], fn func(A) bool) Option[*A] {
	var __mygo_expr_0 Option[*A]
	if __mygo_match___mygo_expr_1, ok := c.(OptionSome[A]); ok {
		var __mygo_expr_2 Option[*A]
		if fn(__mygo_match___mygo_expr_1.F0) {
			__mygo_expr_2 = Some[*A](&__mygo_match___mygo_expr_1.F0)
		} else {
			__mygo_expr_2 = None[*A]()
		}
		__mygo_expr_0 = __mygo_expr_2
	} else {
		if _, ok := c.(OptionNone[A]); ok {
			__mygo_expr_0 = None[*A]()
		} else {
		}
	}
	return __mygo_expr_0
}
func MygoIT11IEnumerableFN17OptionIEnumerableGN1AEGN6OptionGN1AEN1AEM8Contains[A any](c Option[A], item A, EqualsFn func(A, A) bool) bool {
	var __mygo_expr_0 bool
	if __mygo_match___mygo_expr_1, ok := c.(OptionSome[A]); ok {
		__mygo_expr_0 = EqualsFn(__mygo_match___mygo_expr_1.F0, item)
	} else {
		if _, ok := c.(OptionNone[A]); ok {
			__mygo_expr_0 = false
		} else {
		}
	}
	return __mygo_expr_0
}
func MygoIT11IEnumerableFN17OptionIEnumerableGN1AEGN6OptionGN1AEN1AEM3Len[A any](c Option[A]) int {
	var __mygo_expr_0 int
	if _, ok := c.(OptionSome[A]); ok {
		__mygo_expr_0 = 1
	} else {
		if _, ok := c.(OptionNone[A]); ok {
			__mygo_expr_0 = 0
		} else {
		}
	}
	return __mygo_expr_0
}
func MygoIN6OptionM8UnwrapOr[A any](opt Option[A], defaultVal A) A {
	var __mygo_expr_0 A
	if __mygo_match___mygo_expr_1, ok := opt.(OptionSome[A]); ok {
		__mygo_expr_0 = __mygo_match___mygo_expr_1.F0
	} else {
		if _, ok := opt.(OptionNone[A]); ok {
			__mygo_expr_0 = defaultVal
		} else {
		}
	}
	return __mygo_expr_0
}
func MygoIT2EqFN8OptionEqGN1AEGN6OptionGN1AEEM6Equals[A any](left Option[A], right Option[A], EqualsFn func(A, A) bool) bool {
	var __mygo_expr_0 bool
	if __mygo_match___mygo_expr_1, ok := left.(OptionSome[A]); ok {
		var __mygo_expr_2 bool
		if __mygo_match___mygo_expr_3, ok := right.(OptionSome[A]); ok {
			__mygo_expr_2 = EqualsFn(__mygo_match___mygo_expr_1.F0, __mygo_match___mygo_expr_3.F0)
		} else {
			if _, ok := right.(OptionNone[A]); ok {
				__mygo_expr_2 = false
			} else {
			}
		}
		__mygo_expr_0 = __mygo_expr_2
	} else {
		if _, ok := left.(OptionNone[A]); ok {
			var __mygo_expr_1 bool
			if _, ok := right.(OptionSome[A]); ok {
				__mygo_expr_1 = false
			} else {
				if _, ok := right.(OptionNone[A]); ok {
					__mygo_expr_1 = true
				} else {
				}
			}
			__mygo_expr_0 = __mygo_expr_1
		} else {
		}
	}
	return __mygo_expr_0
}
