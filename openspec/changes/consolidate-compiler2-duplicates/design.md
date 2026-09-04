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
which the shared table already carries; extra local `any`). Replace its body
with a lookup through `typeinference2.PrimitiveMyGoSpelling` plus a tiny
`any → Any` fallback:

```text
canonicalMyGoTypeName(name) =
    PrimitiveMyGoSpelling(name).UnwrapOr( if name == "any" then "Any" else name )
```

This fixes `byte`/`rune`/`error` for free and keeps `Any`/`Unit` policy
unchanged (the error-alias table deliberately excludes general aliases such
as `any`, so the single-line fallback is the minimal local supplement).
Alternative considered: add `any` to `GoPrimitivePair`. Rejected because it
contradicts the error-alias non-goal of not introducing general type aliases
into that table.

### D2 — package-aware Go type resolution (A2): one parameterized resolver

Collapse `GoTypeName`/`GoTypeNameWithPackage`, `goSignatureTypes*` +
`goSignatureTypesWithPackage*`, `GoSignatureType`/`GoSignatureTypeWithPackage`,
and `goVariadicParamTypes`/`goVariadicParamTypesWithPackage` onto a single
internal resolver that takes `Option[GoPackageEntry]` (or a leaf-resolver
callback). The exported base names become thin wrappers passing
`None`/`Some(pkg)`, so `go_ffi_import` and tests keep working unchanged
(renamed to the `...WithPackage` convention per D6). The composite parsing
(`[]`, `map[`, `*`) recurses through the one resolver instead of being
copy-pasted. Alternative considered: keep both and have `WithPackage` call
the base for composites; rejected because the only differing site is the leaf,
so a shared walker is strictly less code and cannot diverge.

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
