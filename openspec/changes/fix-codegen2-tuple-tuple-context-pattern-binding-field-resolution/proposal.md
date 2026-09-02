## Why

Bootstrap-compiled code that pattern-matches a two-tuple `(state, event)` where
a case body has a tuple return type and passes a bound pattern variable (e.g.
`InitialMessage`) into a function call generated Go that does not compile:
the bound variable is wrongly emitted as the anonymous tuple's `.F1.InitialMessage`
field access instead of as the local pattern binding.

## What Changes

- Fix the bootstrap codegen2 tuple lowering so a pattern-bound variable used
  as a function-call argument inside a tuple-returning switch case compiles to
  the bound local value, not to `struct{...}{...}.F1.<Field>`.
- Add regression coverage that reproduces the `(State, Slice[Command])` tuple-return
  + enum-variant destructuring + function-argument-use combination.
- No MyGO source-level syntax or semantics change; generated Go for unaffected
  shapes is unchanged.

## Capabilities

### New Capabilities
- (none)

### Modified Capabilities
- `bootstrap-codegen-correctness`: extend the correctness guarantees for
  bootstrap-generated Go to cover pattern-bound variables referenced in the
  tuple-return position of a switch case.

## Impact

- `internal/mygo/codegen2/` — tuple literal lowering and/or expression
  translation that resolves pattern-bound identifiers when the expected return
  type is a tuple.
- `internal/mygo/codegen2/*_test.mygo` / bootstrap test corpus — regression
  test for the tuple-return + pattern-binding case.
- The bootstrap compiler (`./mygo --bootstrap build`) output for the affected
  shape changes from uncompilable to compilable.
