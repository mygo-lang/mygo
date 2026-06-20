package option

import (
	"strconv"

	"github.com/mygo-lang/prelude/control/monad"
	"github.com/mygo-lang/prelude/prelude"
)

type Option___Type interface {
	IsSome() bool
	IsNone() bool
}

type Option___Kind[A any] prelude.HKT[Option___Type, A]

type Some[A any] struct{ Value A }

func (Some[A]) HKT1(Option___Type) {}
func (Some[A]) HKT2(A)             {}

func (Some[A]) IsSome() bool {
	return true
}
func (Some[A]) IsNone() bool {
	return false
}

type None[A any] struct{}

func (None[A]) HKT1(Option___Type) {}
func (None[A]) HKT2(A)             {}

func (None[A]) IsSome() bool {
	return false
}
func (None[A]) IsNone() bool {
	return true
}

// instance Monad Option
type Option__Monad[A, B any] struct {
	Return func(A) Option___Kind[A]
	Bind   func(Option___Kind[A]) Option___Kind[B]
}

var optionMonad monad.Monad[Option___Type, int, string] = Option__Monad[int, string]{
	Return: func(i int) Option___Kind[int] {
		Some[int]{i}
	},
	Bind: func(o Option___Kind[int]) Option___Kind[string] {
		switch o.(type) {
		case Some[int]:
			return Some[string]{strconv.Itoa(o)}
		default:
			return None[string](strcut{})
		}
	},
}
