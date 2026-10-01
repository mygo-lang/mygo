## Context

See `proposal.md` for motivation and `specs/bootstrap-ffi-boundary/spec.md` for the behavior contract. The compiler already distinguishes ordinary Result-shaped FFI calls from explicit raw tuple bindings. Result inference and call lowering currently assume one success value followed by an error; the design must extend the success payload while keeping both paths consistent.

## Goals / Non-Goals

**Goals:**

- Use one Result boundary for any Go FFI signature whose final result is `error`, with all preceding results carried in the Ok payload.
- Preserve raw multi-return behavior for explicit tuple destructuring.
- Keep the existing two-result `(T, bool)` Option convention and lone-error unit payload.
- Keep bootstrap and self-hosted compiler behavior aligned.

**Non-Goals:**

- Automatically wrapping arbitrary multi-result signatures without a trailing error.
- Changing Go call behavior in statement position or changing the `(T, bool)` convention.
- Adding a new tuple or Result representation to the language.

## Decisions

### Represent two or more successful values as the existing MyGO tuple type

For a signature with `n` successful values and a final error, use `Result[TTuple(successes), Error]` when `n >= 2`, the existing `Result[T, Error]` when `n == 1`, and `Result[(), Error]` when `n == 0`. This gives inference, annotations, and pattern matching one payload representation without inventing a wrapper record. The alternative of synthesizing a named result struct was rejected because the boundary has no stable user-facing name for such a type.

### Construct the complete Ok payload from a single native Go call

Generated Go should bind all successful return values and the trailing error from one call, branch on the error, and construct the tuple payload only on success. The error branch constructs `Err` directly. This avoids re-evaluating calls and preserves Go's native return types, including imported named types. The alternative of calling once for error inspection and again to obtain values is invalid for side-effecting calls.

### Keep tuple destructuring as the explicit raw-return escape hatch

The tuple-let inference and codegen paths should continue to inspect the raw Go signature instead of its Result-shaped ordinary-call type. Arity validation therefore remains against the original Go result count, and nested tuples/wildcards continue using the existing raw binding rules. This preserves current call sites that intentionally bind the error separately.

### Apply the same shape rule across compiler pipelines

The legacy compiler and bootstrap/self-hosted compiler should agree on the FFI result shape and generated behavior. Tests should cover inference shape and emitted Go, including package functions and methods, since both are user-visible entry points to the same boundary.

## Risks / Trade-offs

- [Tuple elements may include package-local or qualified Go types] -> Resolve every success result using the existing package-aware signature conversion before constructing the tuple type and Go expression.
- [Changing ordinary multi-result call types can break code relying on the previous raw tuple expression] -> Preserve explicit tuple destructuring as the documented raw path and make the new ordinary-call behavior explicit in the FFI spec.
- [Compiler pipelines can drift] -> Add matching conformance coverage for inference and code generation in both paths.

## Migration Plan

No data migration is required. Existing `(T, error)`, lone-error, and `(T, bool)` calls retain their shapes. For a multi-result signature ending in `error`, ordinary expression use becomes Result-shaped; callers that need the raw values including the error can use explicit tuple destructuring with one binding per Go result.

## Open Questions
