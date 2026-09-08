## Why

The public API of a MyGO library package that exposes interface-backed methods
(e.g. `lib/concurrency`'s `Chan.Send` / `Receive` / `Len`) cannot be consumed
from another Go module (or another package in the same module) under the
bootstrap pipeline: the call fails at inference with `unknown field Chan.Send`.
Only the package's *free functions* (`MakeChan`, `AsSend`, `Spawn`) are
reachable across a package boundary today. Since a `replace`-based module
consumer is exactly how a downstream project uses `lib/concurrency`, this is a
gap in the primary (bootstrap) pipeline's public-API contract, not a test or
library bug.

## What Changes

- Bootstrap inference SHALL project an imported MyGO package's `impl`/typeclass
  methods into the importer's selector symbol index, so a receiver of an
  imported package type (e.g. `Chan[T]`) resolves its interface methods
  (`Send`, `Receive`, `Len`, `Cap`) instead of failing with `unknown field`.
- After the inference fix, bootstrap codegen2 SHALL be validated to emit a
  *package-qualified* mangled helper for a cross-package typeclass/method call
  (e.g. `concurrency.MygoIT...Send`), and to find a matching dispatch candidate
  for the imported package's impl. If codegen2 currently emits a bare,
  unqualified helper or cannot match the imported candidate, that second gap
  is fixed here so the call is end-to-end buildable.
- Add a regression test that drives a cross-package (and, as a one-off probe,
  a cross-module `replace`) consumer of an interface method through the
  bootstrap pipeline and asserts it infers and produces `go build`-able Go.
- No changes to `lib/concurrency` production code or to the legacy (default)
  pipeline. The legacy pipeline has a related bare-symbol codegen gap; it is
  recorded in design.md as a known sibling gap and deferred to a separate
  change.

## Capabilities

### New Capabilities
- `bootstrap-cross-package-method-dispatch`: bootstrap must type-check and
  generate buildable Go for calls to an imported MyGO package's
  interface/typeclass methods (and inherent impl methods), from both a
  sibling package in the same module and a separate module resolved via
  `go.mod` `replace`.

### Modified Capabilities
<!-- No existing requirement changes. The cross-package dispatch behavior is
     new; existing bootstrap capabilities (codegen correctness, workflow
     parity, enum-variant inference) are untouched. -->

## Impact

- `internal/mygo/typeinference2` (`types.mygo` projection:
  `buildImportedPackageCacheEntry` / `exportMyGoPackageEntries` /
  `myGoPackageStruct*Symbols`, and `InferPackageWithExternal`/`...Info`
  symbol-table assembly): importers gain projected impl-method symbols.
- `internal/mygo/codegen2` (candidate seeding in `decls.mygo` and
  `implMethodCallExpr`/helper naming in `translate_ast.mygo`): may need to seed
  dispatch candidates from imported packages' impls and qualify the emitted
  helper name with the importing alias (to be confirmed by the re-probe).
- New regression test under `internal/mygo` (bootstrap). No library or CLI
  surface changes. Bootstrap is the target; legacy is out of scope.
