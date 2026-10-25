## Why

The bootstrap pipeline currently lowers typed `ast2` directly to Go source. That couples value representation, pattern matching, dictionary passing, and tail-call lowering inside `codegen2`, which forces interface boxing for ADTs, type-assertion-based matching, duplicated tail-position logic, and prevents representation-level optimization.

The project can now accept generated-Go ABI changes and treats `.gen.go` files as build artifacts. This removes the main blocker for a real middle end: a package-level IR with explicit representation and effect information, followed by a dedicated optimizer and Go backend modeled on OCaml's Lambda, Closure, and Flambda 2 layers but adapted to Go's structured control flow.

## What Changes

- Add `ir3`, a package-level structured-ANF IR with `let`/`letrec`/`function`/`apply`/`if`/`switch`, join continuations (`block`/`jump`), static match failure (`raise`), explicit call kinds, tail markers, effect/coeffect annotations, source positions, and canonical value representations.
- Add `optimizer3`, a fixpoint pass pipeline covering pattern-match decision trees, tail-call and SCC lowering, simplification/DCE/CSE, local unboxing, closure conversion, and intra-package inlining/specialization. Cross-package inlining is explicitly out of scope.
- Add `codegen3`, an IR-to-Go backend that preserves per-file output, line directives, imports, metadata, FFI/dynamic-typeclass adapters, and Go build correctness.
- **BREAKING** Adopt canonical value representations for generated Go: `Option[A]` and `Result[A, E]` become value structs, `Option[Ref[T]]` maps to `*T`, `List[A]` becomes a pointer node, enums become tagged structs with recursion broken by pointers, and generic parameters remain real Go type parameters instead of collapsing to `any`.
- Add an opt-in `--codegen3` pipeline switch plus IR dump/verification tooling; keep `codegen2` as the differential baseline until behavioral parity is established.
- Extend generated metadata with IR/representation version and layout information, and include those values in compilation cache keys.
- Defer removal of checked-in `.gen.go` files to a follow-up source-first build change; this change only guarantees that `codegen3` output is reproducible at build time.

## Capabilities

### New Capabilities

- `bootstrap-ir3-pipeline`: Package-level IR construction, pass contracts, effect annotations, tail-position semantics, source attribution, and IR verification.
- `bootstrap-value-representation`: Canonical generated-Go layouts for algebraic data types, `Option`, `Result`, `List`, enums, generics, and their FFI/dynamic-boundary adaptations.
- `bootstrap-codegen3`: Behavioral equivalence, per-file emission, line directives, imports, metadata, pattern-match lowering, tail-call lowering, and FFI handling for the IR-based Go backend.

### Modified Capabilities

None. The change preserves language-level semantics; existing specs such as `bootstrap-codegen-correctness`, `bootstrap-ffi-boundary`, and `switch-exhaustiveness` continue to apply as acceptance constraints.

## Impact

- New compiler packages: `internal/mygo/ir3/`, `internal/mygo/optimizer3/`, `internal/mygo/codegen3/`.
- Modified pipeline orchestration in `internal/mygo/compiler/bootstrap.mygo` and CLI/API surfaces in `cmd/mygo` and `internal/mygo/compiler`.
- Generated Go ABI changes for all MyGO packages; dependents must be recompiled with the same compiler and representation version.
- Prelude, dynamic-typeclass adapters, and FFI boundary code must be regenerated and revalidated.
- Inline Go snippets (`go[T] { ... }`) that assume the old interface/type-assertion shapes require an audit.
- Existing `codegen2` remains available for differential tests and rollback until `codegen3` reaches parity.
- Build-time `.gen.go` generation and repository cleanup are planned follow-ups, not part of this change.
