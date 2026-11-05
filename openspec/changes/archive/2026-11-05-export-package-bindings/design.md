## Context

See `proposal.md` for motivation and `specs/language/exported-package-bindings/spec.md`
for the behavior contract.

Imported MyGO packages are parsed from source on every build. The bootstrap
loader flattens a dependency's `.mygo` files into `ast2.Decl` values, and
`buildImportedPackageCacheEntry` builds two environments from them: a private
environment shaped like the dependency's own source, and a projected surface
whose members are keyed `alias.Name`. The projection is what `inferField`
consults for a package-qualified reference. Today the seeding walk handles
functions, type aliases, nominal types, structs, enums, and interfaces, while
package-level `let` and `var` fall through the catch-all case. The projection
walk also extracts names only for those same declaration forms.

During a compiler run, each dependency is inferred before its importer: the
bootstrap import walk compiles dependencies with `codegen == false`, and
codegen3 builds packages in dependency order. That inference result contains
`TypedDeclSources`, where an unannotated binding's initializer carries the
inferred type on `Expr.Type`. The result is currently discarded after the
dependency's own pass, so the importer only ever sees freshly parsed
declarations and cannot recover an unannotated binding's type.

Package metadata already records the exported type and function surface,
filtering on an ASCII-uppercase first letter through `ast2Exported`.
`ExportedDecls` rebuilds declarations from that metadata when a dependency has
no MyGO sources. It has no representation for package-level values.

Generated Go already uses ordinary package variables: codegen2 emits `var X T`
plus an `init` assignment, and codegen3 emits `var X T = expr`. Both preserve
the source name, so an uppercase binding is a Go-exported variable while a
lowercase binding is unexported. Code generation can therefore spell
`dep.Name`; the missing pieces are the inference surface and codegen3's
annotation-only restriction for package values.

Package bindings that carry a type annotation are predeclared before function
bodies are inferred, which is what makes them visible to every declaration in
their package. An unannotated binding is only added to the environment when its
own declaration is inferred, so declarations that appear earlier in source
order cannot see it. This change preserves that declaration-order boundary.

The inference environment records types but not mutability, and `inferAssign`
only unifies the target and value types. The codegen2 context carries a mutable
flag, but no lookup consumes it. As a result, assigning to a package-level
`let` is currently accepted both locally and through a package qualifier.

## Goals / Non-Goals

**Goals:**

- Make uppercase package bindings part of the imported surface whether their
  type comes from an annotation or from the defining package's inference.
- Preserve package identity in imported binding types, including nested
  collection element types.
- Distinguish imported `var` (assignable) from imported `let` (immutable).
- Keep the codegen2 and codegen3 paths observably equivalent, including
  unannotated package values with resolved inferred types.

**Non-Goals:**

- Changing same-package assignment to `let`, which remains unenforced by this
  change.
- Giving an unannotated binding package-wide forward visibility inside its own
  package; the existing declaration-order behavior stays.
- Exporting lowercase package bindings or introducing a separate export
  keyword.
- Changing the Go-level representation of package variables.
- Unifying MyGO package values with Go FFI package constants or variables.
- Making editor dependency analysis infer every dependency in this change.

## Decisions

### Seed package values from annotations or inferred value types

Extend the imported-package seeding to bind `LetDecl` and `VarDecl` schemes,
and extend the projection to emit `alias.Name` entries for them. The seeding
walk becomes two phases: a declaration phase, refreshed twice like the
package's own predeclaration so aliases declared later are visible, registers
functions, aliases, nominal types, structs, enums, and interfaces; the second
phase binds package values in source order against the completed type
environment.

Each binding's private scheme uses `bind.Type` when present. Otherwise it uses
the inferred type carried by `bind.Value.Type` from the defining package's
inference result. No new diagnostic is introduced for a missing annotation. If
neither source yields a type, the binding is not projected; an importer then
gets the existing unresolved-member behavior rather than a new error at the
definition site.

The projection filters on the same ASCII-uppercase rule already used by
metadata. Lowercase bindings remain in the private environment for the
dependency's own seeding logic but never become `alias.Name` members. The
projection applies `wrapPkgTypesInMonoType` to the value's scheme, so a local
`Box` becomes `TQualifiedName(definingPath, TCon("Box"))` and a `List[Box]` is
rewritten recursively. Builtins, prelude names, and third-party qualified types
keep their existing paths.

Alternative considered: qualify the type while registering the private scheme.
That would corrupt references inside the dependency and is rejected. The
existing function export already qualifies at the projection boundary; values
reuse it.

### Reuse dependency inference through a typed-source cache

Add a typed-source cache to the bootstrap state, keyed by package directory.
After a package's inference completes, store `PackageInfo.TypedDeclSources`
there regardless of whether the pass is inference-only or full codegen. The
cache is per-build state, so source edits cannot leave a stale entry across
compiler invocations.

`bootstrapCollectMyGoPkgInfos` prefers the cached typed declarations for a
dependency and falls back to the parsed source inputs when no typed record
exists. This keeps the existing metadata-only and editor paths working. The
bootstrap import walk already infers direct and transitive dependencies before
the importer collects package information; codegen3 builds dependencies in
order. The cache therefore carries a resolved type for ordinary source builds
without re-running inference inside the seeder.

Alternative considered: infer a binding's initializer lazily while seeding the
importer. That would need the dependency's full type environment, struct field
symbols, impl dictionaries, typeclass resolution, and import overlays. Reusing
the dependency's own inference result is exact and avoids duplicating those
rules. Alternative considered: persist typed declarations in package metadata;
metadata describes the exported API, not full bodies, and would become a
second source of truth for inference.

### Record mutability in a package value index

Add a per-package value index parallel to the existing enum index. Each entry
records whether the binding came from `let` or `var`. The index is built from
the flattened declarations when `MyGoPackageInfo` is constructed and rebuilt
from reconstructed declarations on the metadata path, so it never depends on
inference substitutions.

`inferAssign` consults that index when the assignment target is
`FieldExpr(IdentExpr(alias), name)`, using the same resolution order as
`inferField` so the mutability rule cannot diverge from how the target was
typed. An imported `let` produces a diagnostic; an imported `var` proceeds
through the existing unify step. Extending `EnvEntry` with a mutable flag was
considered and rejected: that structure is shared by locals, package members,
and type schemes, and changing it would touch many more construction sites
without improving local scoping.

Alternative considered: scan the dependency declarations on every assignment.
That is simple but turns an assignment inside a loop into a repeated linear
scan. The index has the same lifecycle as the enum index and keeps the lookup
constant time.

### Use inferred types in the codegen3 value declaration

Codegen3 currently rejects a package value whose source has no annotation.
The typed AST already carries the inferred type on the initializer. Lower the
IR `ValueDecl` annotation from `binding.Type` when present, otherwise from
`binding.Value.Type`; then emit the Go package variable from the IR annotation
or, as a second fallback, the lowered initializer's type. The existing
annotation-required error becomes a "no resolved type" error that only fires
when neither an annotation nor an inferred type exists, such as an untyped
fallback build.

Codegen2 already falls back to `bind.Value.Type` in `translatePackageBinding`,
so its emission does not change. No selector, import, or initialization-order
lowering changes are needed: codegen2 lowers `FieldExpr` to a Go selector and
codegen3 lowers `FieldNamedExpr` to one; assignment targets already have
selector lowering in both paths.

### Build metadata from typed declarations

Add a `Values` list to the metadata exports. Each entry carries the binding
name, a canonical type descriptor, and a mutable flag. The codegen2 metadata
writer receives the package's typed declarations in addition to its parsed
files, and selects the value's type from `bind.Value.Type` first, then
`bind.Type`. `ExportedDecls` reconstructs a `let` or `var` declaration with the
recorded type and a unit initializer; the initializer is irrelevant to
importers and never emitted.

A new MonoType-to-metadata converter preserves package identity for
`TQualifiedName` (including stripping the internal `go:` marker on Go FFI
paths), keeps prelude types unqualified so they resolve through the importer's
prelude environment, and handles builtins, generics, slices, maps, sets,
tuples, functions, and references using the existing metadata shape. A type
the metadata representation cannot express is omitted instead of being
degraded to `any`; the source-backed typed path remains authoritative for that
binding.

The field is additive JSON. Older readers ignore it, and newer readers treat a
missing list as empty, so the schema version stays at 1 and older metadata
files keep loading. The codegen2 metadata writer already filters types and
functions on the uppercase rule; value entries use the same filter.

Alternative considered: store only a rendered Go type string. The existing
canonical `metadata.Type` representation already preserves package identity and
generic arguments, and `ExportedDecls` already converts it back to a source
type expression, so a string would lose information for no benefit. Alternative
considered: record the annotation text only; that would omit unannotated
bindings entirely.

## Risks / Trade-offs

- **Typed cache availability** → A dependency that only has parsed sources
  cannot expose an unannotated binding's type. Prefer the cache and keep the
  parsed fallback so behavior degrades to today's unresolved member instead of
  failing the build. Cover a transitive dependency in the tests.
- **Editor dependency analysis** → The editor currently parses dependencies
  without running their inference, so unannotated imported bindings may remain
  unresolved there until typed sources are warmed. Keep this out of the
  compiler contract and record a follow-up to reuse the typed cache in editor
  analysis; annotated bindings are unaffected.
- **Metadata type fidelity** → Some MonoType shapes have no existing metadata
  descriptor. Omit those value entries rather than emitting `any`, and verify
  the common shapes (builtin, local struct, prelude collection, generic,
  qualified third-party, Go FFI).
- **Unannotated bindings keep declaration-order visibility** → A function
  declared before an unannotated binding still cannot reference it. Document
  this boundary and test both the annotated and unannotated cases so the change
  does not appear to promise package-wide predeclaration.
- **Codegen3 untyped fallback** → A build mode without an inference result
  still has no type to emit. Keep an explicit "no resolved type" error for that
  case; ordinary source builds always have typed declarations.
- **Imported `let` immutability differs from same-package behavior** → Scope
  this change to package-qualified assignment as the spec states, and record
  same-package enforcement as a follow-up.
- **Value index staleness** → Build it from the same declaration slice used by
  the enum index and rebuild it wherever `MyGoPackageInfo` is rebuilt; do not
  cache it across source edits.
- **Alias shadowing** → Reuse `inferField`'s resolution order in the
  immutability check instead of introducing a second rule. Add a fixture for a
  local binding that shares the import alias so the target's inferred type and
  the mutability decision stay aligned.
- **Codegen2 import pruning** → A value-only use of an import must keep the
  import. Cover it with a generated-source assertion.
- **Duplicate names across files** → The value index must follow the existing
  declaration-order behavior for duplicate package bindings instead of
  inventing a new resolution rule.

## Migration Plan

No source-level migration is required and there is no breaking removal: the
annotation rule from the earlier draft is dropped, and codegen3's annotation
rejection becomes a resolved-type check. Land the typed-source cache, seeding
and projection changes, metadata values, and codegen3 fallback as one
coordinated commit so an unannotated binding cannot be accepted by one path
while being rejected by another. Generated metadata with `Values` is readable
by older compilers and generated Go is unchanged, so rollback is a normal
revert of the commit. Verification runs focused type-inference, metadata, IR3,
and codegen tests first, then two-package fixtures through codegen2 and an
isolated codegen3 build, then the full compiler test suite.
