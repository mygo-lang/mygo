## Context

See `proposal.md` for motivation. The design is shaped by the current bootstrap pipeline and by five confirmed constraints:

- The bootstrap pipeline is `parser2 → ast2 → typeinference2 → codegen2 → zz_*.gen.go`, with no IR between typing and Go generation.
- `codegen2` lowers ADTs to Go interfaces plus variant structs, matches via type assertions, and implements tail calls at the AST level with duplicated tail-position variants.
- Go is a structured-control-flow target; a full CPS IR would force state-machine or `goto` reconstruction for ordinary code.
- The compiler is self-hosted in MyGO and must keep compiling itself with the previous generation.
- Generated `.gen.go` files are treated as build artifacts. Cross-package inlining is out of scope for this change. Priority order is: runtime allocation and stack depth, then compile time, then generated-code readability.

`typeinference2.PackageInfo` already retains merged typed declarations and their originating source files through `TypedDecls` and `TypedDeclSources` (`PkgDeclSource`), which gives the IR a natural package-level unit while preserving per-file emission.

## Goals / Non-Goals

**Goals:**

- Introduce a package-level, structured-ANF IR with explicit control-flow joins, call kinds, tail markers, effects, source positions, and value representations.
- Make value representation a deterministic function of type declarations and compiler version, so independently compiled packages agree.
- Provide an optimizer pipeline that removes ADT boxing, compiles pattern matches to decision trees, generalizes tail-call lowering, performs local unboxing, and specializes within a package.
- Provide a Go backend that preserves behavior, per-file output, line directives, imports, metadata, FFI, and dynamic-typeclass dispatch.
- Keep `codegen2` as a differential baseline and rollback path until `codegen3` reaches parity.

**Non-Goals:**

- Cross-package inlining or metadata-carried function bodies.
- Full Flambda 2 CPS, regions, or unboxed-number kinds as a core IR requirement.
- A universal tail-call trampoline for unknown closures or dynamic dispatch.
- Whole-program monomorphization.
- Removing checked-in `.gen.go` files or changing the build workflow in this change.
- New MyGO language surface or changed language-level semantics.

## Decisions

### Decision 1: Structured ANF with join continuations, not full CPS

The IR uses `Let`/`Letrec`/`Function`/`Apply` plus structured `If`/`Switch`/`While`, and adds continuations only for joins and match failure:

- `Block(label, params, body)` and `Jump(label, args)` express join points.
- `Raise(label, args)` expresses guard or pattern-match failure.
- Ordinary sequential code remains structured.

Alternatives considered:

- **Classic Lambda tree only**: simplest, but forces repeated dataflow passes and keeps guard fallback as ad-hoc flags.
- **Full Flambda 2 CPS**: strongest optimization model, but Go code generation must rebuild structured control flow, and the self-hosted implementation cost is high.

Rationale: join continuations capture the optimization value of CPS for decision trees and merges while keeping `codegen3` able to emit ordinary Go `if`/`switch`/`for`. This matches the confirmed IR direction.

### Decision 2: Package-level IR, per-file emission

`ir3` is built from the package's merged `TypedDecls` and keeps `SourcePos` and originating path on every relevant node. `codegen3` groups top-level declarations back to their source files using `TypedDeclSources`.

Alternatives considered:

- **Per-file IR**: rejected because cross-file method dispatch, typeclass resolution, and package-level tail-call SCCs need the merged declaration universe.
- **Whole-program IR**: rejected because cross-package inlining is out of scope and the compiler deliberately compiles packages separately.

All map-backed traversals sort by source position or stable node ID so generated output and IR dumps are reproducible.

### Decision 3: Canonical value representation as a pure function of type declarations

The compiler computes a single Go representation per MyGO type from the type declaration plus `compilerABI`/`reprVersion`. It does not choose representations per package and does not emit stable ABI wrappers.

Canonical mappings:

| MyGO type | Generated Go representation |
| --- | --- |
| `Option[A]` | `struct { Value A; Ok bool }` |
| `Option[Ref[T]]` | `*T`, with `nil` meaning `None` |
| `Result[A, E]` | `struct { Value A; Err E; Ok bool }` |
| `Result[T, error]` at FFI boundary | Go `(T, error)` with an adapter |
| `List[A]` | `struct { node *listNode[A] }`, `nil` meaning `Nil` |
| Non-recursive enum | Tagged value struct with variant fields |
| Recursive enum | Tagged pointer node, or arena index for compiler-owned IR |
| `Ref[T]` | `*T` |
| Generic parameter `T` | Real Go type parameter, not `any` |
| HKT / dynamic typeclass value | Existing `any`/interface encoding for now |

Recursion rule: build the type-reference graph and break direct cycles. A variant field that refers back to the same type (directly or through a mutually recursive cycle) becomes `*T` or an arena index. `Slice[T]`, `Map[K, V]`, and `Option[Ref[T]]` already introduce indirection and do not require an extra pointer.

Alternatives considered:

- **Keep the current interface representation**: lowest risk but preserves the dominant allocation and type-assertion costs.
- **Per-package representation choice**: rejected because independently compiled packages must agree on the generated Go shape.
- **Stable public wrappers plus internal native functions**: rejected because the project accepts generated-ABI changes and wrappers add stack frames, which conflicts with the stack-depth priority.

The representation algorithm version is recorded in metadata and in cache keys. Optional per-type `layoutHash` values allow fast mismatch detection without requiring function bodies.

### Decision 4: Effect and coeffect annotations

`Apply`, `Prim`, `Allocate`, and `Foreign` carry effect annotations:

- `Effect = { Pure, Generative, Read, Write, Panic, Diverge, Foreign }`
- `Coeffect = { NoCoeffect, ReadsEnv, ArbitraryCoeffect }`

Rules that matter for optimization:

- Constructing `Option`/`Result` value structs is `Pure`.
- Constructing list nodes, `Ref.new`, and closures is `Generative`.
- Go FFI and `InlineGo` are `Foreign` (arbitrary effects by default).
- Panic-capable primitives include `Panic`; non-terminating forms include `Diverge`.

Alternatives considered:

- **Full Flambda 2 effect lattice**: more precise but heavier than the current optimizer needs.
- **No effects**: would make DCE, CSE, and loop hoisting unsound around FFI and mutation.

### Decision 5: Pass pipeline and optimization policy

The pipeline is:

```text
Lower3 → Represent3 / Closure3 → optimizer3 fixpoint → Verify3 → codegen3
```

Optimization levels:

- `O0`: `Lower3 → Verify3`
- `O1`: `Lower3 → Match3 → Simplify3 → Tailcall3 → Verify3`
- `O2`: `O1 → Represent3 → Closure3 → Inline3 → Specialize3 → Unbox3 → Simplify3`

Key policies:

- **Pattern matching**: compile to decision trees; bind the subject once; `Raise` to the next arm for failed guards; merge identical patterns so a later arm does not re-test the same tag.
- **Tail calls**: build a tail-call graph only from statically known targets (direct functions, statically resolved inherent/typeclass methods, known dictionary functions, and known function values). Optimize SCCs with a loop or state machine; leave unknown closure/interface calls as ordinary Go calls.
- **Inlining and specialization**: intra-package only, with code-size and recursion budgets. Dictionary specialization replaces known dictionary arguments, inlines, and removes unused parameters when profitable.
- **Unboxing**: local `Option`/`Result` values may become multi-return pairs when producer and consumer are in the same optimized function; `Ref.new(x)` followed immediately by a dereference may be reduced when the reference does not escape.
- **Determinism**: pass worklists, SCCs, map traversals, and dumps are ordered deterministically.

Alternatives considered:

- **Aggressive monomorphization**: rejected for compile time and code-size reasons.
- **Universal trampoline**: rejected because it adds per-call allocation/protocol overhead, conflicting with the allocation priority.
- **Go-compiler-only optimization**: insufficient because Go cannot see ADT tags or dictionary instances.

### Decision 6: codegen3 control-flow mapping

- Reducible joins are reconstructed as `if`/`else` or `switch`.
- Guard fallback and irreducible joins use labels plus `goto`, with cross-block locals hoisted to function scope so Go's declaration rules are satisfied.
- SCC trampolines become loops with a state variable, following the existing mutual-tail-call model but driven by IR tail positions.
- Per-file output, line directives, import inference, metadata emission, FFI adapters, and dynamic-typeclass adapters remain backend responsibilities.
- `codegen3` never performs optimization; it only selects among representations already decided by the IR.

Alternatives considered:

- **CPS-to-state-machine for all code**: rejected for readability and for the risk of hurting Go's own optimization.
- **Keep tail-call logic in the backend**: rejected because tail positions are an IR property and duplicating them in codegen recreates the current AST-level duplication.

### Decision 7: Self-hosted implementation form

- `ir3`, `optimizer3`, and `codegen3` are written in MyGO and compiled first by `codegen2`, then by `codegen3` as a self-host check.
- The initial IR data model follows the `ast2` pattern: a struct with a `Kind` enum plus payload fields, which is already proven to compile and pattern-match in the self-hosted pipeline.
- The IR API is kept independent of its physical representation so an arena/indexed node store can replace the initial model if compile-time profiling justifies it.
- No new MyGO language features are required; the implementation uses existing `Slice`, `Map`, `Set`, `Ref`, `Option`, and `Result`.

Alternatives considered:

- **Go prototype first**: faster iteration, but creates a second implementation and does not exercise self-hosting.
- **Arena from day one**: better locality and less boxing, but more implementation risk before correctness is established.

### Decision 8: Verification and rollback

- Keep `codegen2` available behind the existing pipeline controls; add `--codegen3` as an opt-in path.
- Differential tests compile the same sources with both backends and assert behavioral equivalence rather than textual equality.
- Add an IR textual printer, verifier, and per-pass golden tests.
- Add benchmarks for allocations per operation, recursion stack depth, and compile time.
- Add a debug representation mode (for example `--codegen3-repr=legacy`) for bisecting representation bugs from lowering bugs.
- Extend metadata with `reprVersion` (and optional `layoutHash`) and include these in cache keys.

### Decision 9: Metadata and build boundary

This change does not remove `.gen.go` files from the repository, but it must not assume they persist. Metadata generation stays with `codegen3`; representation and IR versions are recorded so a later source-first build change can reuse the same contract. Cache keys include source digests, imported metadata digests, `compilerABI`, `reprVersion`, and optimization level.

## Risks / Trade-offs

- **[Inline Go snippets assume old ADT shapes]** → Audit every `go[T] { ... }` occurrence; add representation-version notes; keep a legacy representation debug mode for bisecting.
- **[Large payloads make value structs expensive to copy]** → Start with uniform structs, measure, and defer a size-based pointer representation. Any such change updates `reprVersion`.
- **[Recursive enums cannot be flattened naively]** → Require cycle detection and pointer/arena insertion. Add verifier checks that reject infinitely sized generated types.
- **[No cross-package inlining leaves prelude dictionary overhead]** → Optimize prelude internally and accept cross-package dictionary calls initially; consider specialized common-instance entry points only with benchmark evidence.
- **[Unknown closure or interface tail calls can still grow the stack]** → Document the limitation; SCC and statically known targets cover the dominant recursion cases.
- **[Self-hosted compiler compile time grows]** → Keep the initial IR model simple, add pass budgets, and measure before introducing an arena representation.
- **[Differential testing can hide representation-specific bugs]** → Add IR golden tests, verifier checks, FFI/dynamic-dispatch conformance tests, and allocation/stack benchmarks, not only behavioral tests.
- **[Metadata or cache version mismatch]** → Include `compilerABI`/`reprVersion` in cache keys and reject mismatched imported metadata rather than silently mixing layouts.
- **[Go compiler interactions are hard to predict]** → Measure generated code with `testing.AllocsPerRun` and stack-depth tests; avoid reimplementing optimizations Go already performs well.
- **[Generated code becomes less readable]** → Accept `goto`/state-machine output where the priority order requires it; keep line directives and deterministic formatting for debuggability.

## Migration Plan

1. **Phase 0 — baseline and contracts**: record allocation, stack-depth, and compile-time baselines; freeze the canonical representation rules and recursion detection; audit `InlineGo` and FFI/dynamic adapters.
2. **Phase 1 — IR core**: add `ir3` data model, printer, verifier, and `Lower3`; no behavioral change yet.
3. **Phase 2 — codegen3 parity**: add `codegen3` with canonical representations behind `--codegen3`; run differential behavior tests against `codegen2`.
4. **Phase 3 — match and tail calls**: land `Match3` and `Tailcall3`; verify allocation and stack-depth improvements.
5. **Phase 4 — local de-allocation**: land `Unbox3`, `Simplify3`, and `Closure3`.
6. **Phase 5 — intra-package specialization**: land `Inline3` and `Specialize3` with budgets and code-size monitoring.
7. **Phase 6 — default and self-host**: make `codegen3` the default after parity, verify self-hosting, then retire `codegen2` from the default path. Rollback remains `--codegen2`/legacy pipeline plus a full recompile.

Every phase requires a full recompile of MyGO packages because generated-Go ABI changes with `reprVersion`. Binary-only third-party MyGO dependencies are not supported under this model.

## Open Questions

- Exact payload-size threshold for switching an `Option`/`Result` field to a pointer representation; defer until allocation and copy benchmarks exist.
- Whether the IR physical model should move to an arena earlier than Phase 4; depends on self-host compile-time profiling.
- Whether a future opt-in trampoline should cover unknown closure tail calls; not required for this change and would trade allocations for stack safety.
- The build workflow for repository-clean `.gen.go` generation (`-overlay` versus a temporary generation directory); deliberately deferred to a follow-up build-contract change.
