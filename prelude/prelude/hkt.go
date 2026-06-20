package prelude

type HKT[F, A any] interface {
	HKT1(F)
	HKT2(A)
}
