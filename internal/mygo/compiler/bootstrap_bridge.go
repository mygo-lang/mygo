package compiler

import (
	"github.com/mygo-lang/mygo/internal/mygo/abi"
	"github.com/mygo-lang/mygo/prelude"
)

// Go-to-MyGO Result bridges. Inline Go snippets must not depend on an ADT
// representation, so the wrap into Result happens here through the ABI-neutral
// constructors in internal/mygo/abi. Go allows a multi-value call to be
// forwarded as the argument list, so callers write
// bootstrapOutcome(someGoFunc(args)).

func bootstrapOutcome[A any](value A, err error) prelude.Result[A, error] {
	if err != nil {
		return abi.ErrOf[A, error](err)
	}
	return abi.OkOf[A, error](value)
}
