package functor

import "github.com/mygo-lang/prelude/prelude"

type Function__FunctorImpl[F, A, B any] func(func(A) B, prelude.HKT[F, A]) prelude.HKT[F, B]

func (impl Function__FunctorImpl[F, A, B]) Map(f func(A) B) func(fa prelude.HKT[F, A]) prelude.HKT[F, B] {
	return func(fa prelude.HKT[F, A]) prelude.HKT[F, B] {
		return impl(f, fa)
	}
}
