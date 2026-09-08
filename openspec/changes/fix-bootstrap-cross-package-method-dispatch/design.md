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
  correctly rebuilds the `alias.Name` spelling for an imported generic
  receiver such as `concurrency.Chan`) and then calls
  `findSymbol(typeName, field, state.SymbolIndex)`. With no projected
  `ImplMethod` entry, that probe returns `None` and inference reports
  `unknown field Chan.Send`. This is the exact diagnostic observed.
- **Local registration is the template to mirror.**
  Within a package, `implMethodSymbols` / `registerImplMethodsLoop` (in
  `internal/mygo/typeinference2/env.mygo`) build each method's `Scheme`
  against the package's *private* env and register it under
  `receiverName.method` with `implReceiverName(target, iface)` as the receiver
  key. The cross-package projection must produce an equivalent symbol whose
  `typeName` is the `alias.Name` spelling, so the call-site probe keyed by
  `receiverQualifiedName` matches.
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
`wrapPkgTypesInMonoType`, and emit a `Symbol.ImplMethod` whose `typeName` is
`alias + "." + implReceiverName(target, iface)` and whose `field` is the method
name. This is the exact key `receiverQualifiedName` produces at the call site
for an imported generic receiver, so the `findSymbol` probe will match.

**Why this shape:** the local path already proves the `receiverName.method`
key convention and the private-env scheme construction work; the only
difference is that the receiver name must carry the import alias (e.g.
`concurrency.Chan`) rather than the bare `Chan`. Reusing `implReceiverName` +
`alias` keeps both spellings consistent.

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
  `receiverQualifiedName` rebuilds (`alias.Name` for `TQualifiedName` receivers,
  including the `Ref[...]` unwrap). Mitigation: build the key from the same
  `implReceiverName` + `alias` the local path uses; cover a `Ref`-wrapped and a
  generic receiver in the test.
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

- **codegen2 cross-package status (D2).** To be answered by the re-probe task:
  does codegen2 already seed dispatch candidates from imported packages' impls
  and qualify the emitted helper with the alias? This is the only unknown that
  could enlarge the change, and it is gated behind an explicit task rather than
  guessed.
- **Legacy pipeline gap (out of scope, separate change).** The legacy (default)
  pipeline, for the same cross-module consumer, emits a *bare* typeclass
  helper (`_M4Send(ch_1, 7)` / `_M7Receive`) that is `undefined` in the consumer
  and drops the generic type argument on `concurrency.MakeChan(4)`. That is a
  distinct codegen bug in `internal/mygo/codegen` (`translate_call.go` emits
  `ast.NewIdent(helperName)` without an alias selector and does not thread the
  import alias through typeclass dispatch). It should be fixed in a separate
  change so each pipeline's repair is independently reviewable. Deferred here
  because the project prioritizes bootstrap as the primary pipeline.
