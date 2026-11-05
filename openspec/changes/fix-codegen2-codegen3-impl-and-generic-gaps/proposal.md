## Why

The codegen2 and codegen3 paths still accept MyGO programs whose inferred types
or impl calls cannot be lowered into buildable Go. A generic type application
such as `Result[U, E]` can capture `U` into the enclosing `Result` parameter and
infer `Result[E, E]`; an impl declared in a consumer package for an imported
type can produce a helper whose call is never rewritten; and impl methods can be
used as values in inference but are emitted as invalid Go selectors. These are
silent acceptance failures that surface later as `go build` errors or incorrect
inferred types.

## What Changes

- Correct generic type-argument application so substitutions are simultaneous
  with respect to the applied type's own parameters. A caller type parameter
  used in `Result[U, E]` SHALL remain `U`, rather than being resolved through
  the `Result` parameter with the same positional ID.
- Give impl methods a package-independent receiver identity that distinguishes
  `lib.Box` from a local `Box`, and use that identity consistently when seeding
  method tables, matching call receivers, and qualifying emitted helpers in
  codegen2 and codegen3.
- Support impl method values and method expressions. `value.Method` SHALL
  produce a function value that evaluates its receiver once; `Type.Method`
  SHALL produce the unbound receiver-first function; and direct calls through
  either form SHALL lower to the mangled helper rather than a Go selector.
- Preserve the existing codegen2/codegen3 backend selection and legacy ABI.
  The legacy backend is out of scope.

## Capabilities

### New Capabilities

- `bootstrap-generic-type-application`: simultaneous substitution and capture
  avoidance for user-defined generic type applications, including nested
  `Result`/`Option`/collection types in function signatures and callbacks.
- `language/impl-method-values`: bound method values, unbound method
  expressions, and their use as arguments or immediately-called functions for
  inherent and interface impl methods.

### Modified Capabilities

- `bootstrap-cross-package-method-dispatch`: cross-package impl identity now
  includes impl blocks declared in a consumer package whose target type belongs
  to an imported package, and requires buildable helper calls for that case.

## Impact

- `internal/mygo/typeinference2`: generic type application, type-parameter IDs,
  substitution, and method-value type inference.
- `internal/mygo/codegen2`: receiver candidate tables, receiver matching,
  helper qualification, and non-call selector lowering.
- `internal/mygo/ir3` and `internal/mygo/codegen3`: instance-method identity,
  receiver rewriting, and closure/function emission for method values.
- `internal/mygo/compiler`: build-pipeline tests covering both codegen2 and
  codegen3, including generated-Go compilation and runtime behavior.
- `docs/compiler/semantics.md`: remove the current field-only restriction on
  non-call selectors and document method values and method expressions.
- Tests: focused inference cases, codegen source assertions, `go build`
  fixtures for both backends, and at least one runtime check of a bound method
  value that evaluates its receiver once.
