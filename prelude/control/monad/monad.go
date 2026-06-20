package monad

import (
	"github.com/mygo-lang/prelude/control/applicative"
	"github.com/mygo-lang/prelude/prelude"
)

type Monad[M, A, B any] interface {
	applicative.Applicative[M, A, B]
	Return(A) prelude.HKT[M, A]
	Bind(prelude.HKT[M, A], func(A) prelude.HKT[M, B]) prelude.HKT[M, B]
}
