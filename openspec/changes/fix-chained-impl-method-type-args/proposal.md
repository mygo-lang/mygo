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

- Spell the type arguments of every chained receiver impl/typeclass method call
  whose parameters are not inferable in Go, resolved from the inferred receiver
  type, regardless of whether the source carries a type annotation.
- Ensure type inference records fully substituted types on intermediate call
  expressions of a chain so code generation can recover those type arguments
  without a declared or expected type.
- Replace the silent fallbacks: when the required type arguments cannot be
  resolved, produce a source-located diagnostic instead of lowering an
  unresolved impl/typeclass method call to a Go receiver-method selector such as
  `x.Unwrap()` or to a plain call that relies on Go's inference for
  interface-shaped type parameters.
- Add code generation and build regression coverage for non-annotated chains
  such as `Receive().Unwrap().Unwrap()`, including same-shape chains whose
  receiver type is defined in the Prelude and in an imported package.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-codegen-correctness`: adds requirements that chained impl/typeclass
  receiver-method calls spell non-inferable type arguments without a source
  annotation, and that unresolved method dispatch fails with a diagnostic
  instead of emitting uncompilable Go.

## Impact

- `internal/mygo/codegen2/translate_ast.mygo` for receiver-method resolution and
  `implMethodCallExpr`, and `internal/mygo/codegen2/decls.mygo` for seed-time
  type-argument analysis.
- `internal/mygo/typeinference2/infer.mygo` for typed-expression substitution of
  intermediate call expressions.
- Regenerated `zz_*.gen.go` for the packages above, bootstrap sync, and
  regression tests.
- No change to the Go representation or public API of Prelude `Option`/`Result`,
  and no new dependencies.
