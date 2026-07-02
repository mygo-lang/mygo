# AGENTS.md

## Project Shape

- `examples/main/main.mygo` is the canonical design sample for the language surface.
- `internal/mygo/parser.go` owns syntax.
- `internal/mygo/compiler.go` owns lowering to generated Go.
- Generated Go lives next to the source example, e.g. `examples/main/zz_mygo.gen.go`, and should be treated as disposable.

## Type Model

- Keep type parameters explicit in the AST and preserve them in generated Go.
- The current design follows Lisette-style nominal concrete types and structural interfaces.
- Generic enums, structs, interfaces, and functions should remain generic in emitted Go rather than collapsing to `any`.
- Prefer top-level generic functions over generic methods whenever the same behavior can be expressed that way. Use receiver methods only when Go requires them for type identity or interface conformance.
- Named primitive spellings like `Int`, `String`, and `Bool` map to Go primitives in generation.
- `Unit` is a return-only marker in MyGO and should lower to a Go function with no return values, not to `struct{}`.

## Go FFI

- Use `import "go:pkg/name"` for Go packages.
- Allow an optional alias form like `import fmt "go:fmt"` when the Go package name should be explicit.
- Package-qualified selectors such as `fmt.Sprint(...)` should lower as Go selectors, not as struct field access.
- Generated Go should only include helper imports when they are actually needed; `reflect` is now a fallback for truly dynamic `any` function calls, not a blanket import.
- Typeclass-style `impl` blocks should lower to standalone helper functions plus explicit function parameters at call sites, not to method dictionaries.
- `Ref[T]` is the non-nil reference form at the Go boundary and should lower to `*T` in generated Go.
- `Option[Ref[T]]` is the preferred shape for possibly-nil pointer returns and should be preserved rather than collapsed to a bare pointer.
- `Option` continues to represent absence for nilable Go values and comma-ok style results.
- `Result` is the dedicated shape for Go `error`-bearing flows and should be used instead of encoding failures as `Option`.

## Workflow Notes

- Prefer small, focused changes that keep the example file in sync with compiler behavior.
- Keep `examples/main/main.mygo` runnable after compiler changes; its `main` function should actually do work, not only return a value.
- When checking the build, use a writable Go cache if the default cache path is unavailable in this environment.

## Current Semantics

- Function bodies and other block forms are newline-separated statement lists; the last plain expression in a block is the return value.
- `if` now supports a single-line expression form like `if cond then a else b`, and that form does not require `end`.
- `let` introduces an immutable binding. Rebinding the same source name must use a later `let` and is treated as shadowing, not assignment.
- `var` introduces a mutable binding and may be assigned again later in the same scope.
- `let` may omit its type annotation when the initializer provides enough information for inference.
- `let _ = ...` is the supported discard form for return values that should not be bound.
- Pipe operators `<|` and `|>` are both supported in expression lowering.
- Struct literals support a constructor-like form such as `ABC { aaa: 123 }`.
- Generic struct literals can also carry explicit type arguments, such as `Box[Int64] { value: 123 }`.
- When a generic struct literal omits its type arguments, the compiler should infer them from the expected type or field values when possible.
- Keep `examples/main/main.mygo` aligned with the compiler's current boundary behavior, especially for `Ref`, `Option`, and `Result`.
