package cmp

import (
	stdcmp "cmp"
)

type ord__Interface[A any] interface {
	Eq[A]
	Lt(A, A) bool
}

type Ord[A any] interface {
	ord__Interface[A]
	Le(A, A) bool
	Gt(A, A) bool
	Ge(A, A) bool
	Min(A, A) A
	Max(A, A) A
}

type Ord__Impl[A any] struct {
	ord__Interface[A]
}

func (impl Ord__Impl[A]) Le(a, b A) bool {
	return impl.Lt(a, a) || impl.Eq(a, a)
}

func (impl Ord__Impl[A]) Ge(a, b A) bool {
	return !impl.Lt(a, a)
}

func (impl Ord__Impl[A]) Gt(a, b A) bool {
	return !impl.Le(a, a)
}

type Ord__Ordered[A stdcmp.Ordered] struct {
	Ord__Impl[A]
}

func (Ord__Ordered[A]) Lt(a, b A) bool {
	return a < b
}
