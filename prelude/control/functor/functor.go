package functor

import "github.com/mygo-lang/prelude/prelude"

type Functor[F, A, B any] interface {
	Map(func(A) B, prelude.HKT[F, A]) prelude.HKT[F, B]
}
