## Why

During Bootstrap compilation, functions declaring a return value of `Result[(), String]` may generate `Err[any, string]` when `return Err("…")` is called early. Incorrect instances can still be assigned via Go interfaces, but `IsErr()` returns `false`, causing the caller to fail the check. Currently, explicit generics are needed to avoid this.

## What Changes

- Unify the return type of each explicit return value with the return type of the function it belongs to, and preserve the replacement result in the typed AST; nested closures use their own return context.
- Make explicit returns in conditional branches, loops, and nested blocks use a unified return generation path, preserving full generics and existing tuple/unit return rules.
- Generate source location diagnostics for incompatible early returns.
- Add code generation and runtime behavior regression, covering implicit/explicit generics, `Result`, `Option`, nested returns, and closure isolation.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `bootstrap-codegen-correctness`: Adds requirements for explicit returns to be constrained by function return types, generic instance consistency, and rejection of incompatible returns.

## Impact

- Return context, return value inference, and typed AST replacement for `internal/mygo/typeinference2`.
- Explicit return generation and related generated files for branch blocks in `internal/mygo/codegen2`.
- Bootstrap regression testing and bootstrap verification for `internal/mygo/compiler`.
- Does not change the Go representation or public API of Prelude `Result`/`Option`, and does not introduce dependencies.