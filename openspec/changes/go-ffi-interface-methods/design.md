## Context

See `proposal.md` — Why. The Go FFI surface is built in
`internal/mygo/compiler/go_ffi_import.go`: `bootstrapLoadGoPackageGo` reads a
Go package with `go/packages` and renders each exported named type (and each
exported type alias) into a `GoTypeSignature` whose `Methods` come from
`goTypeMethods`. That collector currently calls
`types.NewMethodSet(types.NewPointer(named))` unconditionally. For a concrete
type the pointer method set is right; for an **interface** the pointer-to-
interface method set is empty in `go/types` (the interface's own method set is
only reachable as `types.NewMethodSet(named)`).

Downstream — symbol registration (`goMethodsFromSigs` in `types.mygo`),
resolution (`GoMethodSignatureInPackages` and the `GoMethod` case of
`inferOrdinaryField` in `infer.mygo`), and lowering (`FieldAccess` →
`goast.Selector` in `codegen2/translate_ast.mygo`) — already dispatch Go
methods generically, keyed off the populated `GoTypeSignature.Methods`. They
are interface-agnostic, so they need no changes.

## Goals / Non-Goals

**Goals:**
- Make method calls on `go:`-imported Go **interface** values type-check and
  lower to direct Go calls.
- Surface interface methods for both named interfaces and interfaces reached
  through exported type aliases (user requirement: 支持类型别名).
- Cover the behavior with loader, inference, and codegen tests (user
  requirement: 编写测试).

**Non-Goals:**
- No MyGO-side interface declarations or `impl` blocks against Go interfaces;
  we only dispatch on already-imported Go interface types.
- No struct/field reinterpretation — an interface stays a nominal value type.
- No change to how `Result`/`Option` boundary wrapping works for concretes; the
  interface path reuses the exact same behavior.

## Decisions

### D1. Collect the interface method set from the interface type, not the pointer
In `goTypeMethods`, branch on the underlying type: if
`named.Underlying()` is a `*types.Interface`, use
`types.NewMethodSet(named)`; otherwise keep
`types.NewMethodSet(types.NewPointer(named))`.

Rationale: empirically, `net/http.ResponseWriter`/
`io.Reader`-style interfaces report `pointer-set=0` but `value-set=N`. This is
the minimal, surgical change.

Alternatives considered: hard-coding well-known interfaces, or widening the
loader's `Underlying` handling — both rejected (brittle / unnecessary).

### D2. Apply the fix inside `goTypeMethods` so the alias pass benefits too
Both the exported-alias first pass and the named-type second pass in
`bootstrapGoPackageInfoFromTypes` call `goTypeMethods`. Fixing it once means an
exported alias whose unaliased target is an interface (e.g. Go AST aliases)
automatically exposes the target's methods. This satisfies the alias focus with
no separate alias-specific code path.

### D3. No changes to registration, resolution, or codegen
`goMethodsFromSigs` registers every `GoFuncSignature` in `Methods` as a
`GoMethod` symbol under the `alias.Name` key; `receiverQualifiedName` rebuilds
that same key from the receiver's `TQualifiedName`; `inferOrdinaryField` falls
through to the `GoMethod` case; and `FieldAccess` lowers to `goast.Selector`,
which is valid for interface value receivers. The only reason these didn't fire
was the empty method table.

### D4. `(T, error)` interface methods wrap into `Result`
Once an interface method like `io.Reader.Read` is present in the method table,
the existing FFI boundary wrapping (`Result[T, error]`) applies to it just as
it does to struct methods/package functions. No new wrapping logic.

## Risks / Trade-offs

- [Interface method signatures naming package-local/generic types could fail to
  render into a resolvable MyGO type] → These surface as the same targeted
  `unknown ...` inference error they would today; registration of other methods
  is unaffected. No regression for previously-compiling code.
- [Behavior widens: an interface method call that errored before now compiles
  and produces a `Result`-wrapped value] → Only affects code that previously
  did not compile, so no existing programs change meaning. If a caller ignores
  the result in statement position, the existing discard/value handling
  applies (e.g. `let _ = w.Write(b)` where the value matters).
- [A `(T, error)` interface method used as a branch value differs from `()`] →
  Orthogonal to this change; callers who need the write to be ignored should
  bind and drop it (`let _ = ...`). Tracked as an integration note in tasks.

## Migration Plan

None required. The change is internal to the bootstrap FFI loader and only
widens accepted input; existing generated output is unchanged.

## Open Questions

- None that change the spec/approach/task breakdown. The complete-method-set
  behavior of `types.NewMethodSet` for interfaces already includes promoted
  methods from embedded interfaces (e.g. `io.ReadCloser` ⊃ `Reader`+`Closer`),
  so no per-interface handling is needed.
