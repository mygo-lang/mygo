## Context

See proposal.md - Why. The dynamic metadata pipeline is a third type-rendering path,
separate from ordinary code generation. Ordinary code generation already maps the
compiler channel types to Go channel types (`goSpecialType`/`unaryGoTypeStr` in
`internal/mygo/codegen2/types_util.mygo`). The dynamic path never learned that mapping.

The relevant pieces:

- `metadata.Type` describes types with kinds `builtin`, `typeParam`, `named`, `slice`,
  `map`, `tuple`, `function`, `reference`, and `applied`. There is no channel kind.
- Producer: `dynamicMetadataType` in `internal/mygo/codegen2/dynamic_metadata.go`
  encodes `Chan[T]`, `SendChan[T]`, and `RecvChan[T]` as `kind:"named"` with the channel
  library's import path, because they are neither compiler primitives in its switch nor
  prelude names.
- Consumers that render a descriptor to Go: `dynamicGoType`. Consumers that reconstruct
  MyGO declarations: `metadata.astContext.typeExpr` in `internal/mygo/metadata/ast.go`.
  Go-source-to-MyGO conversion of the generated API: `dynamicGoTypeExprToMygo` in
  `internal/mygo/codegen2/dynamic_api_decls.go`.
- Emission: `dynamicDispatchSourceWithLocal` emits a constructor for every
  implementation found in local and dependency metadata with no usability filter, while
  `Call*` dispatch functions are only emitted when every implementation sharing the
  interface supports the method.

The concurrency channel implementations carry `supported:false` on every adapter,
because the adapter builder rejects any implementation with target type parameters
(`adapter cannot capture implementation target type parameters`). So no `Call*` dispatch
is generated for them, yet their constructors are still emitted and still must compile.

## Goals / Non-Goals

**Goals:**

- Render the compiler channel types correctly in every dynamic path, in both directions.
- Stop emitting dynamic constructors that a compiling package can never use.
- Keep the metadata protocol, `CompilerABI`, and `AdapterABI` stable, so prebuilt
  dependencies and callers stay compatible without coordinated regeneration.
- Restore `go build ./...` for `tutorial-video-generation`.

**Non-Goals:**

- Adding a channel descriptor to the metadata protocol (rejected alternative A).
- Supporting generic implementation targets in adapters, i.e. the `adapter cannot
  capture implementation target type parameters` limitation. Whether the Chan family
  ever gains `Call*` dispatch depends on it; that is a separate change.
- Changing ordinary (non-dynamic) code generation or MyGO surface syntax.

## Decisions

### Fix rendering on the consumer side (chosen; producer descriptor rejected)

Recognize the reserved channel type names where the dynamic API interprets a type, rather
than teaching the producer a new descriptor shape.

Rationale: `Chan`, `SendChan`, and `RecvChan` are compiler-reserved names. The type
checker injects `TCon("SendChan")` and friends directly, and the dynamic path already
rewrites reserved names such as `Option`, `Result`, and `Range`. Matching the reserved
name together with the arity-1 application keeps the metadata encoding unchanged, which
keeps `CompilerABI`, `AdapterABI`, and every implementation ID (a hash over identity
including the target descriptor) stable. Callers and prebuilt dependencies need no
coordination.

The fix must be applied at three places so the rendered `chan<- T` survives the full
round trip: the descriptor-to-Go renderer, the metadata-to-MyGO reconstruction, and the
Go-source-to-MyGO conversion used to seed inference from the generated API.

Rejected alternative A: add a channel kind (direction plus element) to `metadata.Type`
and update `dynamicMetadataType`, `dynamicGoType`, `metadata/ast.go`, `validateType`, and
candidate matching. This is cleaner in the protocol but changes the encoding, changes
every implementation ID, forces regeneration of all downstream artifacts, and broadens
the change beyond the reported defect. The remaining protocol debt (metadata still
claims a non-existent named type) is accepted and documented.

### Governance: emit a constructor only when it is usable

Emit a constructor for implementation `R` of interface `I` if and only if either `I` has
at least one method that is dispatchable across the implementation group, or the
compiling package references `I`'s dynamic API. Extend the existing reference detection
(currently applied only to local interfaces) to dependency implementations.

Rationale: a constructor is useful only if a `Call*` dispatch exists or the package
names the API. The disjunction keeps explicitly referenced usages discoverable so a
genuinely unsupported but wanted capability can still be diagnosed by
`diagnoseDynamicAPIRequests`, while dropping the dead concurrency wrappers for callers
that never use them.

Rejected: the reference trigger alone, which silently drops a constructor for an
interface that is dispatchable but referenced indirectly. Rejected: the dispatchable
trigger alone, which would discard an explicit reference that should surface a
diagnostic.

When nothing qualifies, the emitter returns no declarations, so the package writes no
dynamic API file. The existing empty-`declarations` short-circuit already supports this.

## Risks / Trade-offs

- Metadata still declares a named type that does not exist in Go. -> Documented protocol
  debt; the renderer treats the reserved name/package pair as the channel type. A
  producer descriptor remains the clean end state if a non-concurrency package ever
  reuses these names.
- Name-based special casing could misfire on a user type. -> Match the exact reserved
  name and the arity-1 application; the compiler reserves these names, so MyGO source
  cannot declare them.
- The Go-to-MyGO conversion currently has no channel case, so a rendered `chan<- T` could
  fail to convert back. -> Add round-trip coverage in the tasks.
- Suppressing constructors could hide a real but currently unsupported dynamic use. ->
  An explicit reference still emits the constructor and keeps the diagnostic path alive.

## Migration Plan

Regenerate `lib/concurrency` and every importing project. No metadata or ABI version
change is involved, so prebuilt dependencies and callers do not need coordinated
regeneration. `tutorial-video-generation` `go build ./...` is the end-to-end check; the
targeted regression is the generated `zz_mygo_dynamic.gen.go` for a project importing
`lib/concurrency`.

## Open Questions

- Should the adapter ABI support generic implementation targets so the Chan family gains
  `Call*` dispatch? Deferred to a separate change; resolving it does not alter this
  change's specs, approach, or tasks.
