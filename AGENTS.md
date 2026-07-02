# AGENTS.md

## Project Shape

- `examples/main/main.mygo` is the canonical design sample for the language surface.
- `internal/mygo/parser/` owns syntax parsing; `internal/mygo/parser.go` is now a thin compatibility wrapper.
- `internal/mygo/compiler/` owns lowering to generated Go entry points; `internal/mygo/compiler.go` is now a thin compatibility wrapper.
- `internal/mygo/ast/` owns the shared AST types.
- `internal/mygo/prelude.mysrc` is the built-in prelude source that is merged into every package before lowering.
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
- The built-in prelude provides common typeclasses such as `Show[A]` and `Eq[A]`; prefer using those protocols rather than ad hoc `any` formatting or conversion.
- The built-in prelude also owns foundational algebraic data types like `Option[A]` and `Result[A, E]`; use those rather than redeclaring them in example packages.
- Generated Go should only include helper imports when they are actually needed; `reflect` is now a fallback for truly dynamic `any` function calls, not a blanket import.
- Typeclass-style `impl` blocks should lower to standalone helper functions plus explicit function parameters at call sites, not to method dictionaries.
- `Ref[T]` is the non-nil reference form at the Go boundary and should lower to `*T` in generated Go.
- `Ref[T]` remains a compiler-recognized boundary type, not a prelude-declared enum or struct.
- `Option[Ref[T]]` is the preferred shape for possibly-nil pointer returns and should be preserved rather than collapsed to a bare pointer.
- `Option` continues to represent absence for nilable Go values and comma-ok style results.
- `Result` is the dedicated shape for Go `error`-bearing flows and should be used instead of encoding failures as `Option`.

## Workflow Notes

- Prefer small, focused changes that keep the example file in sync with compiler behavior.
- Keep `examples/main/main.mygo` runnable after compiler changes; its `main` function should actually do work, not only return a value.
- When checking the build, use a writable Go cache if the default cache path is unavailable in this environment.
- The prelude should be authored in MyGO when possible; if a prelude fragment cannot yet be expressed in MyGO, it may be implemented in Go as the lowest-level fallback.

## Current Semantics

- Files start with `package <package_name>` to set the generated Go package name. The old file-level `module` wrapper is removed, and declarations follow directly after the package header.
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
- Typeclass lookup should respect lexical scope first: local bindings and function-value bindings shadow typeclass names, `where`-bound methods are visible inside nested blocks, and package-level dispatch is the fallback.
- When multiple typeclass candidates are visible, prefer the more specific binding by comparing concrete type coverage first, then type-parameter usage, then `any` usage; report ambiguity when candidates remain tied.

## Recent Work

- Split the monolithic AST, parser, and compiler implementation into dedicated subpackages while keeping root-package wrappers for compatibility.
- Moved the parser lexer/token machinery into `internal/mygo/parser/` and kept the root `internal/mygo/parser.go` as a forwarder.
- Added shared AST aliases and moved the canonical AST definitions into `internal/mygo/ast/`.
- Added `while` as an expression form with newline-delimited body parsing and Go `for`-loop lowering.
- Extended expression parsing and lowering to recognize `&&`, `||`, `-`, and `/`, while keeping comparison operators type-checked against `Eq` support.
- Improved numeric literal inference so expected integer and float types are preserved instead of defaulting too early.
- Added compiler coverage for `while` loops, arithmetic/logic operator precedence, and relation-operator rejection when `Eq` support is missing.
