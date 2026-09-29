## Context

See [proposal.md](proposal.md) for the motivating call-site failure. Go package loading already uses `go/types` to inspect imported named types and their method sets. The inference layer converts these types into compiler `MonoType` values, where distinct named interfaces currently remain distinct constructors and structural `Unify` rejects them. Interface argument satisfaction must be checked where the expected Go parameter type and the inferred argument type are both available.

## Goals / Non-Goals

**Goals:**
- Make imported Go interface compatibility follow Go's method-set satisfaction rules at FFI call arguments.
- Reuse complete method sets so embedded-interface methods participate.
- Retain nominal mismatch errors where a required method or signature does not match.

**Non-Goals:**
- General structural typing for MyGO interfaces or user-defined typeclasses.
- Treating arbitrary concrete MyGO types as satisfying imported Go interfaces in this change.
- Changing assignment rules outside Go FFI call arguments.

## Decisions

### Compare source and target method sets at the FFI argument boundary

When ordinary unification fails for an argument, check whether both types are named imported Go interfaces. Accept the argument if every target method is present on the source interface with a Go-compatible signature; otherwise preserve the existing mismatch. This keeps the exception scoped to Go boundary assignability instead of weakening global `Unify` semantics.

Alternative considered: make named interfaces structurally unify everywhere. That would silently affect MyGO type equality and unrelated inference paths, so it is rejected.

### Use completed Go method sets, including embedded interfaces

The Go metadata layer should expose the complete method sets from `go/types` rather than reconstructing promotion by walking source declarations. Go's method-set API already accounts for embedded interfaces, signature identity, and alias resolution.

Alternative considered: special-case common pairs such as `io.ReadCloser` to `io.Reader`. That does not generalize and misses custom interface embeddings.

### Keep the argument position explicit in diagnostics

If method-set satisfaction fails, report the normal call argument mismatch and include the target/source interface names plus a missing or incompatible method when available. This makes failures actionable without claiming two interfaces are structurally identical.

## Risks / Trade-offs

- [Method names and signatures may be represented differently by the current bootstrap and self-hosted metadata layers] → Normalize only for the Go assignability check and add coverage at the loader and inference boundary.
- [A too-broad fallback could accept non-Go values structurally] → Require both argument and parameter types to resolve to imported Go interface metadata before applying method-set satisfaction.
- [Go's identical-signature rules include package identity for unexported methods] → Use canonical `go/types` objects or signature identities from the loader instead of comparing display strings alone.

## Migration Plan

No migration is needed. The change only admits previously rejected, Go-valid FFI calls; existing generated calls retain their source argument expressions.
