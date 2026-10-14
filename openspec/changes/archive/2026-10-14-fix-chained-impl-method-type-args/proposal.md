## Why

Bootstrap compilation of a receiver-style method chain whose type parameters
appear only inside interface-shaped containers (such as `Option[A]` or
`Result[A, E]`) can emit Go that does not compile. For example,
`results.Receive().Unwrap().Unwrap()` currently lowers the inner calls without
explicit type arguments and the outermost call to a Go receiver-method
expression, so `go build` fails with `cannot infer A` even though the source is
valid. Adding a declared type on an intermediate binding makes the same chain
generate correct code, which means code generation currently depends on user
annotations.

## What Changes

- Apply the type arguments written at a call site during type inference, binding
  the callee's type parameters to those types, so a call such as
  `empty[Result[Int, String]]()` is inferred with a concrete result type instead
  of a free variable.
- Spell the type arguments of every chained receiver impl/typeclass method call
  whose parameters are not inferable in Go, resolved from the now-concrete
  inferred receiver type, regardless of whether the source carries a type
  annotation on the receiver or the result.
- Add code generation and build regression coverage for non-annotated chains
  such as `Receive().Unwrap().Unwrap()`, including same-shape chains whose
  receiver type is defined in the Prelude and in an imported package.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-codegen-correctness`: adds a requirement that a written type
  argument is applied during inference and that chained impl/typeclass
  receiver-method calls spell non-inferable type arguments without a source
  annotation.

## Impact

- `internal/mygo/typeinference2/infer.mygo`: apply written type arguments to the
  callee's instantiated parameters in `inferOrdinaryCall`.
- Regenerated `internal/mygo/typeinference2` outputs and new end-to-end fixtures
  in `internal/mygo/compiler`.
- No change to the Go representation or public API of Prelude `Option`/`Result`,
  and no new dependencies.
