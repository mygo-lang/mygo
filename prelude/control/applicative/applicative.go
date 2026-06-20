package applicative

import (
	"github.com/mygo-lang/prelude/control/functor"
	"github.com/mygo-lang/prelude/prelude"
)

type Applicative__Interface[F, A, B any] interface {
	Pure(A) prelude.HKT[F, A]
	Apply(prelude.HKT[F, func(A) B], prelude.HKT[F, A]) prelude.HKT[F, B]
}

type Applicative[F, A, B any] interface {
	Applicative__Interface[F, A, B]
	functor.Functor[F, A, B]
}

// LiftA2 :: (a -> b -> c) -> F a -> F b -> F c
func LiftA2[F, A, B, C any](
	functor functor.Functor[F, A, func(B) C], // 用于 fmap f over F A producing F (B->C)
	apply Applicative[F, func(B) C, C], // 用于 ap F (B->C) <*> F B -> F C
	f func(A) func(B) C,
	fa prelude.HKT[F, A],
	fb prelude.HKT[F, B],
) prelude.HKT[F, C] {
	fab := functor.Map(f, fa) // F (B -> C)
	return apply.Apply(fab, fb)  // F C
}

type Functor[F any] interface {
    Map[A, B any](fn func(A) B, fa HKT[F, A]) HKT[F, B]
}