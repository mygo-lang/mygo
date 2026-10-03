# Design

## Context

See proposal.md for the motivation. Bootstrap already wraps Go calls ending in `error` as MyGO `Result` values and records Go named function types in FFI metadata. The FFI type mapping must also recognize the equivalence between the Go unit/error forms and MyGO's `Result[(), Error]`. When such a Result-returning callback crosses into Go, its Go representation must preserve the callback's required result arity.

## Goals / Non-Goals

**Goals:**
- Normalize both Go error-only and unit-plus-error call results to `Result[(), Error]`.
- Use the same normalization when resolving Go function types and named callback aliases.
- Adapt MyGO Result-returning callbacks to Go callback signatures with one or two results.
- Keep argument and non-unit result checks strict.

**Non-Goals:**
- Change general Go FFI assignability or make unrelated named types structurally interchangeable.
- Change ordinary MyGO function types that do not cross a Go FFI boundary.

## Decisions

### Canonicalize Go unit/error shapes as Result unit

At the FFI type boundary, a trailing `error` with no preceding result and a pair `(struct{}, error)` both map to `Result[(), Error]`. The successful `struct{}` value is the Go representation of MyGO unit, not a distinct payload type. Use this shared shape for direct FFI call results and for error-bearing returns inside imported function signatures.

Alternative considered: leave `(struct{}, error)` as `Result[struct{}, Error]` and add a special unifier exception. That would leak Go's unit representation into MyGO types and create separate behavior for direct calls and callback types.

### Keep compatibility at Go FFI boundaries

Compare normalized callback signatures when checking a Go FFI argument. Do not weaken global unification for arbitrary MyGO function values. Resolve aliases and defined Go function types through their recorded underlying signatures.

Alternative considered: globally erase named Go function types to their underlying shapes. This could make values interchangeable in unrelated inference and assignment contexts.

### Generate a Go ABI adapter for callback results

If the MyGO callback returns `Result[(), Error]` but the Go callback ABI returns `error` or `(struct{}, error)`, emit a wrapper closure at the FFI call site. It invokes the original callback once, maps `Ok(())` to nil error (and a `struct{}{}` payload when required), and maps `Err(e)` to the Go error result. Preserve direct passing when the callback already has the exact Go ABI shape.

Alternative considered: pass the MyGO function value directly after inference accepts it. Go checks actual function value types, so this would generate code that does not compile whenever the return arities differ.

### Preserve strict mismatch behavior

Parameter count and types, plus all non-unit results, must match after FFI normalization. If normalization or signature resolution fails, retain the existing argument mismatch diagnostic.

## Risks / Trade-offs

- [Go type rendering may express unit structs with spacing or fields] → Normalize only the exact empty struct type identified by Go metadata; do not equate arbitrary structs with unit.
- [Generated wrappers can affect function identity and allocation] → Restrict wrappers to callback arguments whose result ABI requires conversion, and invoke the callback exactly once.

## Migration Plan

No source migration is required. Existing FFI calls retain their current behavior except that the Go unit/error pair adopts the MyGO unit result shape. Callback values receive an adapter only when their Go return ABI differs.
