# AGENTS.md

## Project Shape

- `example.mygo` is the canonical design sample for the language surface.
- `internal/mygo/parser.go` owns syntax.
- `internal/mygo/compiler.go` owns lowering to generated Go.
- `zz_mygo.gen.go` is generated output and should be treated as disposable.

## Type Model

- Keep type parameters explicit in the AST and preserve them in generated Go.
- The current design follows Lisette-style nominal concrete types and structural interfaces.
- Generic enums, structs, interfaces, and functions should remain generic in emitted Go rather than collapsing to `any`.
- Prefer top-level generic functions over generic methods whenever the same behavior can be expressed that way. Use receiver methods only when Go requires them for type identity or interface conformance.
- Named primitive spellings like `Int`, `String`, `Bool`, and `Unit` map to Go primitives in generation.

## Go FFI

- Use `import "go:pkg/name"` for Go packages.
- Allow an optional alias form like `import fmt "go:fmt"` when the Go package name should be explicit.
- Package-qualified selectors such as `fmt.Sprint(...)` should lower as Go selectors, not as struct field access.
- Generated Go must continue to include helper imports required by the compiler, such as `reflect`.
- Typeclass-style `impl` blocks should lower to standalone helper functions plus explicit function parameters at call sites, not to method dictionaries.

## Workflow Notes

- Prefer small, focused changes that keep the example file in sync with compiler behavior.
- When checking the build, use a writable Go cache if the default cache path is unavailable in this environment.
