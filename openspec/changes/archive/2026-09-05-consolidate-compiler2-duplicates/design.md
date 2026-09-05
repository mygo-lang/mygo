## Context

The self-hosted compiler2 pipeline regenerates downstream packages from MyGO
sources (`*_test.mygo`/`*.mygo` → `zz_*.gen.{go,test.go}`). `typeinference2`
owns the shared `GoPrimitivePair` table and the Go-type-name→`MonoType`
resolution; `codegen2` imports `typeinference2`. Several functions in
`codegen2` and `typeinference2` re-implement the same mapping/traversal that
a single source already (or should) provide, and the compatibility entry
points are inconsistently named. Motivation is in proposal.md.

Bootstrap sync (`go run ./cmd/mygo --bootstrap sync <pkg>`) must run after any
edit to a self-hosted package so its generator output stays in step.

## Goals / Non-Goals

**Goals:**

- One source of truth for Go↔MyGO primitive spelling.
- One Go-type-name→`MonoType` resolver (package-aware) instead of base +
  `*WithPackage` copies.
- One FFI signature walker with a normalized naming convention.
- One type-rendering skeleton shared by the Go-string and MyGO-string paths.
- Shared utilities in a single leaf `internal/mygo/common2` package.
- Preserve every exported function name and all generated output shape.

**Non-Goals:**

- Not touching the v1 engines (`parser`, `typeinference`, `codegen`).
- Not consolidating the large `*UsesName`/`*UsesPreludeName` AST predicate
  family (future work).
- Not changing the `Any`/`Unit` boundary policy that keeps those two shapes
  outside the shared primitive table.
- No new language features or public behavior changes.

## Decisions

### D1 — primitive spelling (A1): delegate, don't duplicate

codegen2's `canonicalMyGoTypeName` currently re-implements the whole
Go→MyGO primitive switch and has drifted (missing `byte`/`rune`/`error`,
which the shared table already carries; extra local `any`). Add the missing
`any`↔`Any` pair to the shared `GoPrimitivePair` table (per the "no silent
`any` fallback" decision below, `any` becomes a first-class pair rather than a
wildcard), and replace codegen2's body with a pure lookup:

```text
canonicalMyGoTypeName(name) =
    PrimitiveMyGoSpelling(name).UnwrapOr(name)
```

This fixes `byte`/`rune`/`error` for free. `goPrimitiveType`'s local `Any`
entry becomes redundant (the table now answers `Any`→`any`) and is removed;
`Unit` stays local. Alternative considered: keep `any` out of the table and
preserve a local `any`→`Any` fallback. Rejected because the boundary no
longer treats `any` as a wildcard — it must be a real, declared pair.

### D2 — package-aware Go type resolution (A2): one parameterized resolver

Collapse `GoTypeName`/`GoTypeNameWithPackage`, `goSignatureTypes*` +
`goSignatureTypesWithPackage*`, `GoSignatureType`/`GoSignatureTypeWithPackage`,
and `goVariadicParamTypes`/`goVariadicParamTypesWithPackage` onto a single
internal resolver that takes `Option[GoPackageEntry]` (or a leaf-resolver
callback). Per the confirmed boundary decision, the resolver **returns
`Result[MonoType, String]`**: an unknown bare name, a malformed `map[`
(missing `]`), or an unknown package leaf name is an `Err`, never a silent
`TVar(0)`/`any` fallback. The exported entry points become thin wrappers
passing `None`/`Some(pkg)` (renamed to the `...WithPackage` convention per
D6). The composite parsing (`[]`, `map[`, `*`) recurses through the one
resolver instead of being copy-pasted.

The `Result` propagates through the signature builders
(`goSignatureTypes*`, `goVariadicParamTypes*`, `GoSignatureType*`) into the
env-seeding chain (`seedGoPackageFuncs` → `seedGoPackageMembers` →
`seedGoPackageEnv` → `seedMyGoPackageEnv` → `cachedImportedPackage` →
`buildImportedPackageCacheEntry` → `importedPackageEnv` →
`prepareInferenceSetup`). The five exported `Infer*` entry points already
return `Result[PackageInfo, String]`, so the error surfaces to the compiler
instead of silently lowering an unbound type to `any`.

**Behavior change (intentional):** Go FFI signature types that cannot be
resolved (unknown type name at the boundary) previously degenerated to
`TVar(0)`/`any`; they now fail compilation with an explicit error. Declared
`any` still resolves through the new `any`↔`Any` table pair. This is the one
deliberate external behavior change in an otherwise behavior-preserving
refactor.

**Generic FFI type parameters (D2 extension):** go/types renders a generic
declaration's own parameters bare inside its signature (`maps.Clone`'s `m M`,
`maps.All`'s `m Map` — the maps package names its map-shaped type parameter
`Map`). The shared resolver therefore carries a parameter map
(`typeParams: Map[String, Int]`) built from the declaration's type-parameter
list in order (`goTypeParamMap`: position `i` → `TParam(-(i+1))`). A bare leaf
that names one of these parameters resolves to the TParam before any package
resolution; a bare leaf with no matching parameter is still an `Err`, never a
placeholder. `GoFuncSignature` gains a `TypeParams: Slice[String]` field so
the resolver is reachable end to end:

- `go_ffi_import.go` extracts the names from go/types
  (`goFuncTypeParams`; for methods on generic types the receiver type's
  parameters are the ones rendered bare, so they are prepended).
- The self-hosted `bootstrap.mygo` path needs no such extraction: the
  Go-package signature strings it used to derive for MyGO dependencies
  (`bootstrapMyGoPackageSignatures` / `bootstrapTypeName`) were removed as
  dead code — their `packagesRef.value().Append(...)` calls discarded the
  result (`Ref[Slice[T]]` lowers to Go `*[]T`, so the appended entries never
  reached `seedGoPackageEnv`). MyGO dependencies already typecheck through the
  real-declaration path (`MyGoPackageInfo` → `seedMyGoPackageEnv`), so the
  string-signature duplicate was both lossy and redundant. All shape
  conversion now lives in the one shared resolver: `[]T`→`Slice[T]`,
  `map[K]V`→`Map[K, V]` (with bracket-matched keys, so composite keys such as
  `map[[]int]string` parse), `*T`→`Ref[T]`, `func(...)`→`TFunc`, chan
  variants, and `()`→`TUnit`.
- `seedGoPackageFuncs` binds them (`typeParamIDs(fn.TypeParams, 1)`) so each
  call instantiates fresh variables, and the `GoMethod` selector path binds +
  instantiates the method's parameters the same way.
- `map[K]V` and other composite shapes recurse through the same parameter map,
  so `maps.Collect`'s `map[K]V` result becomes `TApp(Map, [TParam(-1),
  TParam(-2)])` — the Go `map` keyword maps to MyGO's `Map` constructor, and a
  bare type parameter such as `maps.All`'s `Map` resolves to its `TParam`
  before any package lookup.

Alternative considered: keep both and have `WithPackage` call the base for
composites; rejected because the only differing site is the leaf, so a shared
walker is strictly less code and cannot diverge.

Alternative considered: keep the resolver returning `MonoType` and report
errors at a single call site. Rejected because the unknown type is only
detectable at the leaf inside the recursion, so any "report later" scheme
would need to smuggle the error out through the type value anyway.

### D3 — FFI signature lookup (A3/B1): one walker + predicate

The `ffiOptionSignature` / `ffiResultSignature` / `ffiMultiResultSignature`
families and the parallel `ffiRawTupleResultType*` scan share the identical
package/func scan and differ only in a results predicate (`Results[1]=="bool"`,
`Results[1]=="error"`, `Results.Len()>1`, raw-tuple selection). Introduce a
single `ffiSignatureForCallee` walker + `InPackages`/`InFuncs` that take a
predicate, and keep thin accessors. Rename the option walker (`ffiSignature*`
→ `ffiOptionSignature*`) so the family follows the same
`*Option/*Result/*Multi/*RawTuple …InPackages/…InFuncs` convention. The
`...FromCallee` input-shape variants are folded into the shared walker (they
already differ only by package parameterization), eliminating the
`FromCallee` suffix.

### D4 — one type-rendering skeleton (A4)

The Go-string vs MyGO-string renderers share traversal but duplicate bodies:

- `substituteTypeParamsInTypeExpr` / `substituteTypeParamsMyGO` and
  `funcTypeSubstitutedResult` / `funcTypeSubstitutedResultMyGO` (decls.mygo).
- `monoTypeToGoStrIn` vs `monoTypeToGoStrWithParamsIn` (types.mygo) — layers
  added progressively as more context (params/aliases) accumulated.

Unify each pair behind one recursion parameterized by render style (Go vs
MyGO) and available context (type params, path aliases). Keep the widely
used public entry points (`monoTypeToGoStr`, `monoTypeToGoStrWithParams`,
etc.) as thin wrappers. `monoTypeToMygoStr` already delegates to
`typeinference2.MonoStringFull` and stays as-is.

### D5 — shared utilities package (A5): new `internal/mygo/common2`

`sliceDrop`, `joinStrings`, `containsString` (and any other helpers defined
identically in more than one compiler2 package) move into a new leaf package
`internal/mygo/common2`, written as a self-hosted MyGO package (its own
`.mygo` sources → `zz_*.gen.go`). `typeinference2` and `codegen2` import it
and drop their local copies under a `common2.` prefix. `common2` imports
nothing from the compiler2 family, so no cycle is introduced. The new package
is added to the bootstrap directory set so `--bootstrap sync` regenerates it
in dependency order.

### D6 — context-suffix naming policy (B2)

The `...For`/`...In`/`...WithParams`/`...Into`/`FromCallee` suffixes each
started as "carry one more piece of context", but the meaning is not
documented and a few names drifted (notably `...For`, which has no parallel in
the codebase). Adopt and record one suffix per context axis, and stop mixing
axes in one name:

- `...WithPackage` — package-aware resolution: carries a `GoPackageEntry` and
  resolves bare names to `TQualifiedName(pkg.Path, …)`. This follows the
  dominant `With<Context>` convention (`TypeFromASTWithParams`,
  `monoTypeToGoStrWithParams`) and is the single retained "extra context"
  suffix for the Go-type-name entry points
  (`GoTypeNameWithPackage`, `GoSignatureTypeWithPackage`).
- `...WithParams` — type-parameter-name context present.
- `...In` / `...InEnv` — environment or import/alias table present.
- `...Into` — tail-recursion accumulator (the intentional MyGO manual-recursion
  idiom). Kept as-is across the codebase; not a rename target.
- `...FromCallee` — removed as a suffix: input-shape variants are folded into
  the shared FFI walkers so the base accessors already operate on a callee.

This policy is captured as a short comment at the consolidated FFI walker and
the package-aware Go-type resolver. It is a naming policy, not a mass rename:
the `...Into` accumulator family and other consistently-named non-`WithPackage`
context variants are left untouched.

## Risks / Trade-offs

- [Generated symbol names change where `byte`/`rune`/`error` now canonicalize through the shared table (D1)] → Regenerate affected packages and run the codegen2/typeinference2 golden tests; the drift fix is the intended correction.
- [A single multi-context renderer (D4) could regress a corner case the layered versions handled] → Keep public wrappers; lean on the existing unit/golden tests for mono and type-expr rendering.
- [Adding a new regenerated package (common2) complicates the bootstrap loop] → Add it to the bootstrap sync dir list first and verify a clean `--bootstrap sync` before wiring imports.
- [Wide blast radius across codegen2/typeinference2] → Land A1/A3/B1 (smallest, tight to recent work) before A2/A4/A5.

## Migration Plan

1. Implement in order A1, A3/B1, A2, A5, A4 (small → large).
2. After each package change run
   `GOCACHE=/tmp/mygo-go-build go test ./internal/mygo/typeinference2 ./internal/mygo/codegen2 ./internal/mygo/compiler`.
3. Regenerate with `go run ./cmd/mygo --bootstrap sync` for each touched
   self-hosted package, then re-run the test suites above.
4. Nothing is user-facing; no data migration. Rollback = revert the relevant
   `.mygo` + regenerated `.gen.go` pair.

## Open Questions

None. The `any` handling and the bootstrap wiring for `common2` are resolved
in D1/D5 and reflected in tasks.
