package monoid

type Monoid interface {
	MEmpty() Monoid
	MAppend(Monoid, Monoid) Monoid
}
