## Context

See proposal.md for motivation. The relevant current-state constraints, all
confirmed by tracing the bootstrap pipeline and reproducing the failure in a
`replace`-based scratch module:

- **Module import resolution works.**
  `resolveMyGoImport` / `resolveGoModuleImportDir` locate an imported MyGO
  package across a module boundary via the consumer `go.mod`'s `replace`
  (and the module cache for a plain `require`). So the failure is NOT in
  import resolution.
- **Free functions project across the boundary; impl methods do not.**
  An imported package's surface is built in `buildImportedPackageCacheEntry`
  (`internal/mygo/typeinference2/types.mygo`). Two projection helpers run there:
  - `exportMyGoPackageEntries` projects `FuncDecl` / `TypeAliasDecl` /
    `TypeDecl` / `StructDecl` / `EnumDecl` / `InterfaceDecl` as `alias.Name`
    env entries.
  - `myGoPackageStructSymbols` projects `StructDecl` fields as
    `Symbol.StructField` under `alias.Name`.
  Neither projects `ImplDecl` methods, so no `Symbol.ImplMethod` for the
  imported package is ever placed in the importer's `SymbolIndex`.
- **The call-site lookup is a bare symbol-index probe.**
  `inferField` computes the receiver key with `receiverQualifiedName` (which
  rebuilds the `alias.Name` spelling for an imported package-declared receiver
  such as `concurrency.IChannel`, but leaves a compiler-builtin receiver such
  as `Chan` bare, because a builtin arrives at the call site as a `TCon`)
  and then calls
  `findSymbol(typeName, field, state.SymbolIndex)`. With no projected
  `ImplMethod` entry, that probe returns `None` and inference reports
  `unknown field Chan.Send`. This is the exact diagnostic observed.
- **Local registration is the template to mirror.**
  Within a package, `implMethodSymbols` / `registerImplMethodsLoop` (in
  `internal/mygo/typeinference2/env.mygo`) build each method's `Scheme`
  against the package's *private* env and register it under
  `receiverName.method` with `implReceiverName(target, iface)` as the receiver
  key. The cross-package projection must produce an equivalent symbol whose
  `typeName` is the spelling `receiverQualifiedName` rebuilds at the call site
  (bare for a builtin receiver, `alias.Name` for a package-declared receiver),
  so the call-site probe keyed by `receiverQualifiedName` matches.
- **Symbol-table assembly.** `InferPackageWithExternal` and
  `InferPackageWithExternalInfo` build the initial `Symbols` as
  `concatSymbols(setup.Symbols, importedStructSymbols)` and derive
  `SymbolIndex` via `symbolIndexFromSlice`. `importedStructSymbols` comes from
  `myGoPackageStructSymbols`. Projected impl-method symbols must be folded into
  the same initial list so they reach the `SymbolIndex`.
- **Codegen2 dispatch is separate and must be re-verified.**
  `codegen2.translateReceiverImplMethodCall` dispatches through
  `ctx.inherentCandidates` and `ctx.typeclassCandidates`, which are seeded by
  `seedInherentCandidates` / `seedImplCandidates` (in `decls.mygo`) from the
  *local* package's impls. The emitted helper name is built by
  `implMethodSymbol(stem, method)`. Whether codegen2 (a) seeds a matching
  candidate for an imported package's impl, and (b) qualifies the emitted
  helper with the import alias, is NOT yet confirmed — inference currently
  fails before codegen runs. This is captured as decision D2 and validated in
  tasks.

## Goals / Non-Goals

**Goals:**
- Make bootstrap inference resolve interface, typeclass, and inherent impl
  methods on imported-package receiver types (sibling package and cross-module
  `replace`).
- Confirm and, if needed, complete the codegen2 side so the accepted call
  emits a qualified, buildable helper call.
- Add a bootstrap regression test covering the cross-package (and one cross-
  module `replace`) case.

**Non-Goals:**
- No changes to `lib/concurrency` production code or to the legacy (default)
  pipeline. The legacy pipeline has a related bare-symbol / missing-type-arg
  codegen gap; it is recorded below as a known sibling gap and deferred.
- No new language features; no change to how impls are declared or how
  same-package dispatch works.
- No change to import resolution, `go.mod` handling, or the module cache
  lookup path (already working).

## Decisions

### D1 — Project imported impl methods into the importer's symbol index
Mirror the local `implMethodSymbols` logic into the cross-package projection in
`types.mygo`. For each imported package's `ImplDecl`, compute each method's
`Scheme` against the package's *private* env (the same spelling its own source
uses, so receiver/param/return types resolve locally), wrap any
package-local type names with `TQualifiedName(path, ...)` via the existing
`wrapPkgTypesInMonoType`, and emit a `Symbol.ImplMethod` whose `field` is the
method name and whose `typeName` (the receiver key) is chosen to match exactly
what `receiverQualifiedName` rebuilds at the call site:
- if the receiver name `implReceiverName(target, iface)` is a type *declared*
  by the imported package (i.e. it is in the package's `collectMyGoTypeNames`,
  so the receiver arrives at the call site as `TQualifiedName(path, name)`),
  the key is `alias + "." + name` (e.g. `concurrency.IChannel`);
- otherwise (the receiver is a compiler builtin such as `Chan` / `SendChan` /
  `RecvChan`, which arrives at the call site as a bare `TCon` and so is
  unqualified by `receiverQualifiedName`), the key is the bare `name` (e.g.
  `Chan`).
This mirrors the local `implMethodSymbols` key convention plus the import
alias rule, so the `findSymbol` probe in `inferField` matches.

**Why this shape:** the local path already proves the `receiverName.method`
key convention and the private-env scheme construction work. The only
cross-package difference is that a *package-declared* receiver must carry the
import alias (e.g. `concurrency.IChannel`) to match the `TQualifiedName`
spelling `receiverQualifiedName` rebuilds, whereas a *builtin* receiver
(e.g. `Chan`) is unqualified at the call site and so keeps its bare name.
Reusing `implReceiverName` plus the declared-vs-builtin test keeps both
spellings consistent with the call site.

**Receiver-stripping constraint.** After `findSymbol` returns the projected
`ImplMethod`, `inferField` calls `stripReceiverArg(substed, typeName, args,
...)` to drop the receiver from the method type. `stripReceiverArg` only
recognizes a *bare* `TCon(name)` / `TApp(TCon(name), _)` receiver whose `name`
equals the `typeName` key, and returns `None` otherwise. This has two
consequences for the projected scheme:
- For a *builtin* receiver (`Chan` / `SendChan` / `RecvChan`) the key is the
  bare name and `wrapPkgTypesInMonoType` leaves the receiver as a bare `TCon`,
  so `recvName == typeName` and the receiver is stripped and unified
  correctly. This is the channel case the motivating scenario exercises.
- For a *package-declared* struct receiver (the inherent-impl-on-exported-
  struct scenario), the projected receiver is wrapped to a `TQualifiedName`, so
  `stripReceiverArg` returns `None` and the receiver is not stripped. That
  path therefore needs `stripReceiverArg` (or the projection's receiver
  spelling) adjusted so a package-qualified receiver still strips and unifies
  against `alias.Name`. This is in scope for tasks 2.2/2.3 and is covered by
  the inherent-impl verification rather than left implicit.

**Alternatives considered:**
- *Register the method in the importer's env under `alias.Name.method` only
  (no symbol index).* Rejected: `inferField` resolves method *calls* through
  `findSymbol` on the `SymbolIndex`, not through the value env, so an env-only
  entry would still fail with `unknown field`.
- *Special-case `Chan`/channels in the projection.* Rejected: the user's
  project convention forbids special-case workarounds; the general fix is to
  project every `ImplDecl`, which also covers inherent impls and any future
  library.

### D2 — Re-probe codegen2 after the inference fix, then complete if needed
Once inference accepts the call, bootstrap codegen2 takes over. Validate the
two codegen2 questions:
1. *Candidate seeding* — does `seedImplCandidates` / `seedInherentCandidates`
   see the imported package's impls (so `translateReceiverImplMethodCall` finds
   a matching receiver candidate), or only the local package's?
2. *Helper qualification* — does `implMethodCallExpr` emit the helper with the
   import alias (e.g. `concurrency.MygoIT...`), or a bare unqualified symbol
   (which would be `undefined` in the consumer, the same failure class as the
   legacy pipeline)?

If either is missing, fix it here: seed dispatch candidates from the imported
packages' impls (keyed by the same `alias.Name` receiver spelling) and qualify
the emitted helper identifier with the alias. If both are already present, the
change is inference-only and this decision records the confirmation.

**Why defer to a re-probe:** inference currently hard-fails before codegen,
so codegen2 behavior for an accepted cross-package method call is unobserved.
Guessing would risk an incorrect design; the tasks gate the design on an
actual re-probe.

### D3 — Regression test shape
Add a bootstrap test that (a) compiles a consumer package importing a library
package's interface method in the same module, and (b) reproduces the
cross-module `replace` consumer, asserting inference succeeds and the generated
Go is `go build`-able (or, where a full build is impractical in the unit test,
asserts the emitted helper is alias-qualified). Keep the fixture minimal
(a tiny interface + impl + one method) rather than depending on
`lib/concurrency`, so the test is hermetic and fast; `lib/concurrency` remains
the motivating example in the proposal.

**Alternatives considered:** depending on `lib/concurrency` directly (rejected
— couples the inference test to one library's surface and to its Go build);
internal-only `_test` style (rejected — that path shares the local symbol
table and would not exercise the projection, which is the bug).

## Risks / Trade-offs

- [Projected scheme mismatch] A method `Scheme` built in the private env must
  wrap every package-local type name with `TQualifiedName(path, ...)`; missing
  one leaves a bare `TCon` that fails to unify with the call-site
  `TQualifiedName`. Mitigation: reuse the exact wrapping already applied to
  exported functions (`wrapPkgTypesInMonoType`) and the existing
  `myGoPackageStructSymbols` wrapping, and assert unification in the test.
- [Receiver-key drift] The projected `typeName` must exactly match what
  `receiverQualifiedName` rebuilds: bare `Name` for a builtin `TCon` receiver
  (e.g. `Chan`), `alias.Name` for a package-declared `TQualifiedName` receiver
  (e.g. `concurrency.IChannel`), including the `Ref[...]` unwrap. Mitigation:
  build the key from the same `implReceiverName` plus the declared-vs-builtin
  test the call site uses; cover a builtin (`Chan`), a package-declared
  interface receiver, a `Ref`-wrapped, and a generic receiver in the test.
- [Name collisions] Two imported packages with the same interface method
  receiver name. Mitigation: the symbol key is `alias.Name.method`, so distinct
  aliases do not collide; a same-alias re-import is already an error today.
- [codegen2 gap larger than expected] The D2 re-probe may reveal codegen2 also
  needs candidate seeding and helper qualification, enlarging the change. This
  is scoped into this change (it is the same user-facing behavior) and gated by
  an explicit task; if it proves large enough to be a separate concern, split it
  into a follow-up change and note that here.
- [Scope creep into legacy] The legacy pipeline has a sibling bare-symbol
  codegen gap. Mitigation: explicitly a Non-Goal; record it in Open Questions
  for a separate change.

## Migration Plan

Additive only. Steps: (1) implement the impl-method projection in
`types.mygo` and fold it into the initial symbol list used by
`InferPackageWithExternal` / `...Info`; (2) re-probe the `replace` consumer
through `--bootstrap` and confirm inference now accepts `ch.Send` / `ch.Receive`;
(3) per D2, confirm or complete codegen2 candidate seeding + helper
qualification; (4) add the regression test; (5) run `go test ./internal/mygo/...`
and the `replace`-module `go build`. Rollback is reverting the `types.mygo`
(and, if touched, `decls.mygo` / `translate_ast.mygo`) edits. No data, deploy,
or public-API migration steps.

## Open Questions

- **codegen2 cross-package status (D2) — answered by the 3.1 re-probe.**
  After the inference fix, a cross-module `replace` consumer that imports
  `concurrency` and calls `ch.Send(7)` /`ch.Receive()` etc. now *infers*
  cleanly, but codegen2 emits the method call as a **bare Go method call**
  (`ch.Send(7)`, `ch.Receive()`) rather than the mangled helper, so
  `go build` fails with `type chan int has no field or method Send`.
  Root cause, confirmed:
  - Candidate seeding (`newPackageIndex` → `seedPackageDictionaries`) only
    sees the local package's visible decls (`ExternalTypedDecls` +
    `TypedDecls`). For a cross-module consumer the imported package
    (`concurrency`) is *not* among those, so no `ImplDictionaryCandidate`
    exists for `Chan`/`SendChan`/`RecvChan` receivers and
    `matchingReceiverCandidate` returns `None`, falling through to an
    ordinary (method) call.
  - The emitted helper name (`implMethodSymbol`) is **not alias-qualified**.
    A same-module external test works only because it imports the sibling
    package with a *dot import* (`import . ".../concurrency"`), so the bare
    mangled helper resolves in scope. A cross-module consumer uses a *named*
    import (`import concurrency "..."`), so the helper must be emitted as
    `concurrency.<helper>` (a `goast.Selector`), mirroring how the free
    function `concurrency.MakeChan[int](4)` is already qualified.
  - `PackageInfo` does not carry the imported packages'
    (`MyGoPackageInfo`) decls, so codegen2 currently has no access to them;
    completing D2 requires threading the imported decls (plus alias/path) into
    `GenerateFiles`/`newPackageIndex`, seeding candidates from the imported
    impls (keyed by the same receiver name the inference projection uses), and
    alias-qualifying the emitted helper.
  This is a real, non-trivial enlargement of the change beyond the inference
  fix. It is the remaining work for tasks 3.2/3.3.
- **Legacy pipeline gap (out of scope, separate change).** The legacy (default)
  pipeline, for the same cross-module consumer, emits a *bare* typeclass
  helper (`_M4Send(ch_1, 7)` / `_M7Receive`) that is `undefined` in the consumer
  and drops the generic type argument on `concurrency.MakeChan(4)`. That is a
  distinct codegen bug in `internal/mygo/codegen` (`translate_call.go` emits
  `ast.NewIdent(helperName)` without an alias selector and does not thread the
  import alias through typeclass dispatch). It should be fixed in a separate
  change so each pipeline's repair is independently reviewable. Deferred here
  because the project prioritizes bootstrap as the primary pipeline.
