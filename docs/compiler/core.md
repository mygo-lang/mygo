# core.md — Project Core Rules

## Project Shape

- `examples/main/main.mygo` is the canonical design sample for the language surface.
- `internal/mygo/parser/` owns syntax parsing.
- `internal/mygo/compiler/` owns lowering to generated Go entry points.
- `internal/mygo/ast/` owns the shared AST types.
- `internal/mygo/common/pos.go` owns shared position and error helpers.
- `prelude/prelude.mygo` is the built-in prelude source that is merged into every package before lowering.
- Generated Go lives next to the source example, e.g. `examples/main/zz_mygo.gen.go`, and should be treated as disposable.

## Type Model

- Keep type parameters explicit in the AST and preserve them in generated Go.
- The current design follows Lisette-style nominal concrete types and structural interfaces.
- Generic enums, structs, interfaces, and functions should remain generic in emitted Go rather than collapsing to `any`.
- Prefer top-level generic functions over generic methods whenever the same behavior can be expressed that way. Use receiver methods only when Go requires them for type identity or interface conformance.
- Supported numeric types: `Int`, `Int8`, `Int16`, `Int32`, `Int64`, `UInt`, `UInt8`, `UInt16`, `UInt32`, `UInt64`, `Float32`, `Float64`. All map directly to Go primitives via `primitiveGoName`, `goType`, `hmTypeString`, and `typeString`.
- Integer literals support hex (`0xff`), octal (`0o777`), and binary (`0b1010`) prefixes via lexer rules in `parser_lex.l`, all producing `NUMBER` tokens with the raw literal string as value.
- Named primitive spellings like `Int`, `String`, and `Bool` map to Go primitives in generation.
- `()` (empty tuple) is the unit type in MyGO source and should lower to a Go function with no return values, not to `struct{}`. The old `Unit` spelling is no longer recognized.

## Workflow Notes

- Prefer small, focused changes that keep the example file in sync with compiler behavior.
- Keep `examples/main/main.mygo` runnable after compiler changes; its `main` function should actually do work, not only return a value.
- When checking the build, use a writable Go cache if the default cache path is unavailable in this environment.
- The prelude should be authored in MyGO when possible; if a prelude fragment cannot yet be expressed in MyGO, it may be implemented in Go as the lowest-level fallback.
