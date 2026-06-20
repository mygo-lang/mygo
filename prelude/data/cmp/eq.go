package cmp

type eq__Interface[A any] interface {
	Eq(A, A) bool
}

type Eq__Impl[A any] struct {
	eq__Interface[A]
}

func (impl Eq__Impl[A]) Neq(a, b A) bool {
	return !impl.Eq(a, b)
}

type Eq[A any] interface {
	eq__Interface[A]
	Neq(A, A) bool
}

type Eq__comparable[A comparable] struct {
	Eq__Impl[A]
}

func (Eq__comparable[A]) Eq(a, b A) bool {
	return a == b
}
