package prelude

type Option[A any] interface {
	isOption()
}
type OptionSome[A any] struct {
	F0 A
}

func (OptionSome[A]) isOption() {
}
func Some[A any](v0 A) Option[A] {
	return OptionSome[A]{F0: v0}
}

type OptionNone[A any] struct {
}

func (OptionNone[A]) isOption() {
}
func None[A any]() Option[A] {
	return OptionNone[A]{}
}

type Result[A any, E any] interface {
	isResult()
}
type ResultOk[A any, E any] struct {
	F0 A
}

func (ResultOk[A, E]) isResult() {
}
func Ok[A any, E any](v0 A) Result[A, E] {
	return ResultOk[A, E]{F0: v0}
}

type ResultErr[A any, E any] struct {
	F0 E
}

func (ResultErr[A, E]) isResult() {
}
func Err[A any, E any](v0 E) Result[A, E] {
	return ResultErr[A, E]{F0: v0}
}
func OptionToResult[A any, E any](opt Option[A], errVal E) Result[A, E] {
	var __mygo_expr_0 Result[A, E]
	if __mygo_match___mygo_expr_1, ok := opt.(OptionSome[A]); ok {
		__mygo_expr_0 = Ok[A, E](__mygo_match___mygo_expr_1.F0)
	} else {
		if _, ok := opt.(OptionNone[A]); ok {
			__mygo_expr_0 = Err[A, E](errVal)
		} else {
		}
	}
	return __mygo_expr_0
}
func MygoIN6ResultM8ToOption[A any, E any](res Result[A, E]) Option[A] {
	var __mygo_expr_0 Option[A]
	if __mygo_match___mygo_expr_1, ok := res.(ResultOk[A, E]); ok {
		__mygo_expr_0 = Some[A](__mygo_match___mygo_expr_1.F0)
	} else {
		if _, ok := res.(ResultErr[A, E]); ok {
			__mygo_expr_0 = None[A]()
		} else {
		}
	}
	return __mygo_expr_0
}
func MygoIN6ResultM7Flatten[A any, E any](res Result[Result[A, E], E]) Result[A, E] {
	var __mygo_expr_0 Result[A, E]
	if __mygo_match___mygo_expr_2, ok := res.(ResultOk[Result[A, E], E]); ok {
		__mygo_expr_0 = __mygo_match___mygo_expr_2.F0
	} else {
		if __mygo_match___mygo_expr_1, ok := res.(ResultErr[Result[A, E], E]); ok {
			__mygo_expr_0 = Err[A, E](__mygo_match___mygo_expr_1.F0)
		} else {
		}
	}
	return __mygo_expr_0
}
func OptionFilter[A any](opt Option[A], fn func(A) bool) Option[A] {
	var __mygo_expr_0 Option[A]
	if __mygo_match___mygo_expr_1, ok := opt.(OptionSome[A]); ok {
		var __mygo_expr_2 Option[A]
		if fn(__mygo_match___mygo_expr_1.F0) {
			__mygo_expr_2 = opt
		} else {
			__mygo_expr_2 = None[A]()
		}
		__mygo_expr_0 = __mygo_expr_2
	} else {
		if _, ok := opt.(OptionNone[A]); ok {
			__mygo_expr_0 = None[A]()
		} else {
		}
	}
	return __mygo_expr_0
}
func MygoIT2EqFN8ResultEqGN1AN1EEGN6ResultGN1AN1EEEM6Equals[A any, E any](left Result[A, E], right Result[A, E], EqualsFn func(A, A) bool, EqualsFn_1 func(E, E) bool) bool {
	var __mygo_expr_0 bool
	if __mygo_match___mygo_expr_2, ok := left.(ResultOk[A, E]); ok {
		var __mygo_expr_3 bool
		if __mygo_match___mygo_expr_4, ok := right.(ResultOk[A, E]); ok {
			__mygo_expr_3 = EqualsFn(__mygo_match___mygo_expr_2.F0, __mygo_match___mygo_expr_4.F0)
		} else {
			if _, ok := right.(ResultErr[A, E]); ok {
				__mygo_expr_3 = false
			} else {
			}
		}
		__mygo_expr_0 = __mygo_expr_3
	} else {
		if __mygo_match___mygo_expr_1, ok := left.(ResultErr[A, E]); ok {
			var __mygo_expr_2 bool
			if _, ok := right.(ResultOk[A, E]); ok {
				__mygo_expr_2 = false
			} else {
				if __mygo_match___mygo_expr_3, ok := right.(ResultErr[A, E]); ok {
					__mygo_expr_2 = EqualsFn_1(__mygo_match___mygo_expr_1.F0, __mygo_match___mygo_expr_3.F0)
				} else {
				}
			}
			__mygo_expr_0 = __mygo_expr_2
		} else {
		}
	}
	return __mygo_expr_0
}
func Panic(msg string) {
	panic(msg)
}
func Zero[A any]() A {
	return func() A {
		var zero A
		return zero
	}()
}
