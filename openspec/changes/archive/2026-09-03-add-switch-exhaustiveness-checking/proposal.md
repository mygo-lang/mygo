## Why

MyGO's `switch` statement currently does not require exhaustive pattern checking. When a case is missing, it either generates a `panic("unreachable")` (in the old compiler's expression context) or leaves an undefined execution path (in compiler 2). This causes the switch branch to be missed when an enumeration variant is added, only to be exposed at runtime via a panic, and cannot be caught at compile time. This change allows both compilers to detect and reject non-exhaustive `switch` statements at compile time.

## What Changes

- **BREAKING**: For enumeration type `switch` statements, the compiler requires exhaustive checking of all variants; otherwise, a compilation error will occur.
- **BREAKING**: The `switch` statement form (not at the end of the expression) also requires exhaustive checking; only explicit `_` wildcards or overriding all enumeration variants are valid.
- Recursively check patterns nested in tuples and variant fields; for example, in `switch (a, b)`, exhaustiveness is checked when `a` is an enumeration.
- Covers two compiler paths:
    - Legacy compiler `internal/mygo/compiler/validate.go`
    - Bootstrap compiler `internal/mygo/typeinference2/`
- Adds compile-time error messages for both compilers, indicating missing enumeration variants (e.g., `non-exhaustive switch: missing variant(s) Foo, Bar`).
- Existing code and tests that rely on "non-exhaustive switch compileable" need to be explicitly changed to wildcards or have additional cases added.
- **Not included**: Language parsing support for the `Bool` literal pattern (`case true`); should be proposed separately if needed later.

## Capabilities

### New Capabilities

- `switch-exhaustiveness`: Defines the exhaustiveness requirements for switch pattern matching in MyGO, including enumeration variant coverage, wildcard fallback, and recursive checks in nested patterns.

### Modified Capabilities

- None

## Impact

- Affects two compiler implementations:
    - `internal/mygo/compiler/validate.go` (old compiler static validation)
    - `internal/mygo/typeinference2/infer.mygo` (bootstrap compiler type inference)
- Affects code generation:
    - `internal/mygo/codegen/translate_control.go` (old compiler, can retain `panic("unreachable")` as a defense)
    - `internal/mygo/codegen2/translate_ast.mygo` (bootstrap compiler, no fallback is needed after exhaustive checks pass)
- Affects existing tests and examples: Any `switch` statement that is not exhaustive and lacks an underscore will become a compilation error.
- Requires synchronous updates or additions: Compilation error tests for both backends, generated code golden tests, and bootstrap golden tests for the bootstrap compiler.