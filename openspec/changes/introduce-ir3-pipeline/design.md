## Context

See `proposal.md` for motivation. The design is shaped by the current bootstrap pipeline and by five confirmed constraints:

- The bootstrap pipeline is `parser2 → ast2 → typeinference2 → codegen2 → zz_*.gen.go`, with no IR between typing and Go generation.
- `codegen2` lowers ADTs to Go interfaces plus variant structs, matches via type assertions, and implements tail calls at the AST level with duplicated tail-position variants.
- Go is a structured-control-flow target; a full CPS IR would force state-machine or `goto` reconstruction for ordinary code.
- The compiler is self-hosted in MyGO and must keep compiling itself with the previous generation.
- Generated Go is treated as a build artifact that never lands in the source tree: the compiler writes it to a user-level content-addressed cache and runs `go build` / `go test` against a temporary overlay, while `go.mod`/`go.sum` remain project inputs and dependencies resolve through `GOPROXY`. Cross-package inlining is out of scope for this change. Priority order is: runtime allocation and stack depth, then compile time, then generated-code readability.

`typeinference2.PackageInfo` already retains merged typed declarations and their originating source files through `TypedDecls` and `TypedDeclSources` (`PkgDeclSource`), which gives the IR a natural package-level unit while preserving per-file emission.

## Goals / Non-Goals

**Goals:**

- Introduce a package-level, structured-ANF IR with explicit control-flow joins, call kinds, tail markers, effects, source positions, and value representations.
- Make codegen3 value representation a deterministic function of type declarations and compiler version, so independently compiled codegen3 packages agree.
- Provide an optimizer pipeline that removes ADT boxing, compiles pattern matches to decision trees, generalizes tail-call lowering, performs local unboxing, and specializes within a package.
- Provide a Go backend that preserves behavior, per-file output, line directives, imports, FFI, and dynamic-typeclass dispatch without emitting a metadata artifact.
- Keep `codegen2` as the default backend, differential baseline, rollback path, and owner of the legacy interface-based ABI throughout this change.

**Non-Goals:**

- Cross-package inlining or precompiled function-body metadata.
- Full Flambda 2 CPS, regions, or unboxed-number kinds as a core IR requirement.
- A universal tail-call trampoline for unknown closures or dynamic dispatch.
- Whole-program monomorphization.
- Removing `codegen2`, changing its legacy ABI, or making codegen3 the default in this change.
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

### Decision 3: Codegen3 canonical value representation as a pure function of type declarations

For codegen3, the compiler computes a single Go representation per MyGO type from the type declaration plus the compiler representation version. It does not choose codegen3 representations per package and does not emit stable ABI wrappers. Codegen2 keeps its established interface-based ADT ABI; the two backends are selected per package and are never mixed.

Canonical mappings:

| MyGO type | Generated Go representation |
| --- | --- |
| `Option[A]` | Tagged struct with `Some(A)` and `None` variants |
| `Option[Ref[T]]` | `*T`, with `nil` meaning `None` |
| `Result[A, E]` | Tagged struct with `Ok(A)` and `Err(E)` variants |
| `Result[T, error]` at FFI boundary | Go `(T, error)` with an adapter |
| `List[A]` | `struct { node *listNode[A] }`, `nil` meaning `Nil` |
| Non-recursive enum | Tagged value struct; large declared payloads may be indirected per variant |
| Recursive enum | Tagged struct with indirect variant payloads, or arena index for compiler-owned IR |
| `Ref[T]` | `*T` |
| Generic parameter `T` | Real Go type parameter, not `any` |
| HKT / dynamic typeclass value | Existing `any`/interface encoding for now |

Recursion rule: build the type-reference graph and break direct cycles. A variant field that refers back to the same type (directly or through a mutually recursive cycle) becomes indirect. `Slice[T]`, `Map[K, V]`, and `Option[Ref[T]]` already introduce indirection and do not require an extra pointer.

Large-payload rule: a non-recursive enum also stores variant payloads indirectly when a variant directly contains another declared value type. This keeps large unions such as `ast2.Decl` from being copied through matches and calls. Once an enum uses indirect payloads, constructors accept source-level values and store pointers internally; match bindings retain source-level field types.

Alternatives considered:

- **Keep the current interface representation for codegen2**: retained as the compatibility and rollback path; it is not replaced by this change.
- **Per-package representation choice**: rejected because independently compiled packages must agree on the generated Go shape.
- **Stable public wrappers plus internal native functions**: rejected because the project accepts generated-ABI changes and wrappers add stack frames, which conflicts with the stack-depth priority.

The representation algorithm is versioned by the compiler build. No metadata artifact records it: all packages in one build use the same compiler, so cross-package layouts cannot diverge. Compilation cache keys include the compiler version and source digests; precompiled output from a different compiler version is not mixed into a build.

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
- Per-file output, line directives, import inference, FFI adapters, and dynamic-typeclass adapters remain backend responsibilities. No metadata file is emitted.
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

- Keep `codegen2` as the default path with its legacy interface ADT ABI; add `--codegen3` as an explicit opt-in path and do not change the default backend in this change.
- Differential tests compile the same sources with both backends and assert behavioral equivalence rather than textual equality.
- Add an IR textual printer, verifier, and per-pass golden tests.
- Add benchmarks for allocations per operation, recursion stack depth, and compile time.
- Add a debug representation mode (for example `--codegen3-repr=legacy`) for bisecting representation bugs from lowering bugs.

### Decision 9: Source-derived dependencies and repository-clean build boundary

`codegen3` does not emit a metadata file. Cross-package declarations, exported types, and dynamic-dispatch information are derived from source declarations compiled in the same build. The compiler version and source digests are the compatibility boundary; precompiled generated output from another compiler version is not mixed into the build. Cache keys include the compiler version, source digests, and optimization level; no metadata digest is part of the model.

Generated Go is written only to a user-level content-addressed cache described in Decision 11, never beside `.mygo` sources and never into a persistent project-local directory such as `build/.mygo3`. A per-build overlay file, created inside the cache's temporary area, maps logical source paths to the immutable cache entries. The compiler invokes the Go toolchain directly (`go build`, `go test`) against that overlay, so binaries and test results are produced without a checked-in generated layer and without materializing generated sources in the project. `--codegen3-emit=<dir>` is the only materialization path and is opt-in for debugging and IDE tooling.

The project keeps `go.mod` (and `go.sum` when present) as the module context for those invocations. Dependencies are downloaded through `GOPROXY`; the build does not need vendored or pre-generated MyGO metadata. Generated package directories may be linked into a synthetic module or exposed through a Go overlay, but that mechanism is an implementation detail and MUST NOT require writes under the source tree.

### Decision 10: `test` is a first-class CLI command

`mygo test` selects the same pipeline as `mygo build`, and `mygo --codegen3 test` selects the IR backend. The command deliberately mirrors `go test`:

- The first non-flag directory argument selects the working root, exactly as `build` already does; otherwise the root is the current directory. Package patterns default to `.`.
- Package selection uses `go list -deps -test` so test-only imports and external `_test` packages enter generation before the test binary is compiled.
- Generation and caching use the same dependency closure and cache entries as `build`; the generated test files are ordinary cache entries, not a separate artifact type.
- Execution is `go test -overlay <overlay> <remaining args...>` with stdin, stdout, and stderr inherited. The exit status is propagated unchanged, so a failing test produces a non-zero exit code and a toolchain failure is reported with the existing `go test (codegen3 overlay)` context.
- The repository-clean contract applies: `zz_*.gen_test.go` is never written beside the sources.

Test *results* are not stored in the MyGO cache in this change; repeated runs reuse Go's own test cache. The MyGO cache stores the generated test sources so a cold test run still avoids code generation work.

Alternatives considered:

- **Shell out with a separate `mygo-test` helper**: rejected because package selection, dependency scanning, and cache keys would duplicate `build` semantics.
- **Only run tests through `go test` after a manual `mygo sync`**: rejected because it writes generated files into the repository and defeats the repository-clean contract.

### Decision 11: Content-addressed, cross-module codegen3 cache

The cache is modeled on `GOCACHE`: immutable entries, content-addressed keys, atomic publication, and self-healing reads. It replaces the persistent project-local `build/.mygo3` directory.

**Root.** `MYGO_CACHE` selects the cache root; when unset the compiler uses `os.UserCacheDir()/mygo` (for example `~/.cache/mygo` on Linux). `MYGO_CACHE=off` disables caching. The root is never inside the project by default and is independent of any single Go module.

**Key.** A cache key is a domain-separated hash over everything that can change the generated text, in a fixed order:

1. cache format version;
2. compiler identity (build ID/version) plus the representation version and adapter ABI;
3. optimization level and any backend options that affect emission (for example the canonical-prelude switch);
4. the package's ordered `(relative source path, content digest)` pairs for `.mygo` sources and the hand-written Go files that participate in the package;
5. dependency keys for imported MyGO packages, in deterministic import order, so an ABI change in a dependency invalidates its importers (Merkle-style);
6. Go module and toolchain inputs that feed FFI signature discovery: `go.mod`/`go.sum` digests, the Go toolchain version, and a digest of the resolved Go package metadata used by the package;
7. nothing path-absolute, time-based, or environment-derived beyond the inputs above.

The module root, module path, checkout location, and user name are therefore absent from the key. Two modules containing identical sources compiled by the same compiler with the same options and dependencies share every entry.

**Entry.** An entry is an immutable directory `<root>/build/<xx>/<key>-d/` containing `manifest.json` and the generated `zz_*.gen.go` / `zz_*.gen_test.go` files. The manifest records the format version, key, compiler identity, representation/adapter versions, optimization level, producing package path, source root (for diagnostics only), and each file's size and digest. The manifest is written last with a completion marker; readers verify the manifest and file digests before use.

**Publication and concurrency.** Generation writes into `<root>/tmp/<random>/` and then renames the directory into place. If the final entry already exists the temporary copy is discarded; content addressing guarantees both copies describe the same bytes. Builds hold a shared lock on the cache; cache cleaning holds an exclusive lock (Decision 12).

**Consumption.** The per-build overlay maps canonical source-tree paths such as `prelude/zz_option.gen.go` to the cache entry files. The overlay JSON lives in `<root>/tmp/` and is removed after the build. Nothing is written under the project except `go.mod`/`go.sum` changes the Go toolchain itself decides to make. The set of canonical packages is derived from the in-memory build manifest, not from scanning a build directory.

**Correctness boundary.** A cache miss, a disabled cache, a deleted cache, or an unreadable entry only costs time: the compiler regenerates and produces the same bytes. Entries whose format, compiler identity, representation version, or adapter ABI do not match are never mixed into a build.

**Determinism and path independence.** Generated text SHALL be independent of the absolute module root; line directives name package-relative paths. A determinism test compiles the same source from two different roots with a cold cache (`MYGO_CACHE=off`) and asserts byte-identical generated text.

Alternatives considered:

- **Keep `build/.mygo3` per module**: simplest, but duplicates output across modules, leaves artifacts in the working tree, and cannot share work between checkouts.
- **Cache absolute output paths**: rejected because it defeats cross-module reuse and leaks machine-specific paths into keys.
- **Cache archives instead of directories**: fewer inodes, but every build would pay extraction cost before `go build` could read the overlay targets.

### Decision 12: Cache clearing is an explicit, locked command

`mygo cache clean` removes every entry and temporary overlay under the active `MYGO_CACHE` root and recreates the empty `build/` and `tmp/` directories. `mygo clean -cache` is accepted as the Go-compatible spelling and performs the same operation. The command prints the cache root and a summary of the space or entries removed.

- Cleaning takes the cache's exclusive lock. If a build is in flight, the command waits for a bounded time; if the lock cannot be acquired it reports the active cache users and exits non-zero without deleting anything.
- `MYGO_CACHE=off` makes cleaning a successful no-op with an explanatory message.
- A missing or partially initialized cache is treated as empty.
- After cleaning, the next `build` or `test` performs a full regeneration and must produce the same output as before; a warm run afterwards reuses the new entries.

Alternatives considered:

- **Delete the cache directory without locking**: can tear down files a concurrent `go build` is reading; rejected.
- **Only document `rm -rf $MYGO_CACHE`**: loses the locking and summary behavior and is easy to mistype; kept only as an escape hatch.

## Risks / Trade-offs

- **[Inline Go snippets assume old ADT shapes]** → Audit every `go[T] { ... }` occurrence; add representation-version notes; keep a legacy representation debug mode for bisecting.
- **[Large payloads make enums expensive to copy]** → Use indirect variant payloads when a non-recursive enum directly contains another declared value type. Keep a fixed byte threshold for Option/Result and other payload-size cuts as future measured work; representation changes bump the codegen3 representation version.
- **[Recursive enums cannot be flattened naively]** → Require cycle detection and pointer/arena insertion. Add verifier checks that reject infinitely sized generated types.
- **[No cross-package inlining leaves prelude dictionary overhead]** → Optimize prelude internally and accept cross-package dictionary calls initially; consider specialized common-instance entry points only with benchmark evidence.
- **[Unknown closure or interface tail calls can still grow the stack]** → Document the limitation; SCC and statically known targets cover the dominant recursion cases.
- **[Self-hosted compiler compile time grows]** → Keep the initial IR model simple, add pass budgets, and measure before introducing an arena representation.
- **[Differential testing can hide representation-specific bugs]** → Add IR golden tests, verifier checks, FFI/dynamic-dispatch conformance tests, and allocation/stack benchmarks, not only behavioral tests.
- **[Source-derived dependency information requires source availability]** → Compile all MyGO dependencies from source in the same build; reject binary-only or metadata-only dependencies instead of guessing layouts.
- **[Go compiler interactions are hard to predict]** → Measure generated code with `testing.AllocsPerRun` and stack-depth tests; avoid reimplementing optimizations Go already performs well.
- **[Generated code becomes less readable]** → Accept `goto`/state-machine output where the priority order requires it; keep line directives and deterministic formatting for debuggability.
- **[A shared cache can be corrupted or poisoned]** → Every entry carries a manifest with the key, versions, and per-file digests; readers ignore incomplete or mismatched entries and regenerate. A corrupt entry is a performance problem, never a correctness problem.
- **[The key can omit an input that changes output]** → Keep the key list explicit in this design, test cold-versus-warm byte identity, and prefer false misses over false hits when a new input is added.
- **[Cached text embeds machine-specific paths]** → Require generated text to use package-relative line directives and add a cross-root determinism test; an absolute path in generated text is a defect.
- **[Cleaning races with a concurrent build]** → Builds take a shared cache lock and cleaning takes the exclusive lock; cleaning never deletes files that an active build is reading.
- **[The shared cache grows without bound]** → Entries are immutable and individually replaceable; document an explicit size/eviction policy before the cache becomes the default. `mygo cache clean` remains the manual escape hatch.
- **[IDEs and gopls cannot see generated sources inside the cache]** → Keep `--codegen3-emit=<dir>` as the documented, opt-in materialization path for tooling; the default remains cache-only.

## Migration Plan

1. **Phase 0 — baseline and contracts**: record allocation, stack-depth, and compile-time baselines; freeze the canonical representation rules and recursion detection; audit `InlineGo` and FFI/dynamic adapters.
2. **Phase 1 — IR core**: add `ir3` data model, printer, verifier, and `Lower3`; no behavioral change yet.
3. **Phase 2 — codegen3 parity**: add `codegen3` with canonical representations behind `--codegen3`; run differential behavior tests against `codegen2`.
4. **Phase 3 — match and tail calls**: land `Match3` and `Tailcall3`; verify allocation and stack-depth improvements.
5. **Phase 4 — local de-allocation**: land `Unbox3`, `Simplify3`, and `Closure3`.
6. **Phase 5 — intra-package specialization**: land `Inline3` and `Specialize3` with budgets and code-size monitoring.
7. **Phase 6 — codegen3 opt-in self-host and rollback**: build codegen3-generated Go only into the user-level cache, run `go build`/`go test` against a temporary cache overlay, stop creating `build/.mygo3` for codegen3, remove checked-in generated files when the opt-in path no longer needs them, and verify self-hosting. `codegen2` remains the default backend with its legacy interface ABI; this change does not switch the default or retire codegen2.
8. **Phase 7 — test command, shared cache, and cache lifecycle**: add `mygo test` / `mygo --codegen3 test`, the content-addressed cache with cross-module reuse, the locked `mygo cache clean` (and `mygo clean -cache`) commands, and the `--codegen3-emit` debug escape hatch. Verify two-module cold/warm runs, concurrent builds, interrupted entries, and a cold rebuild after cleaning.

Codegen3 phases require a full recompile of MyGO packages because canonical generated-Go ABI changes with the compiler version. Codegen2 packages continue using the established ABI and default path. All MyGO dependencies built through codegen3 must be compiled from source in the same build; binary-only or metadata-only third-party dependencies are not supported under this model. Go module inputs (`go.mod`/`go.sum`) remain in the project, and third-party Go dependencies are fetched through `GOPROXY`.

## Open Questions

- Exact payload-size threshold for switching an `Option`/`Result` field to a pointer representation; defer until allocation and copy benchmarks exist.
- Whether the IR physical model should move to an arena earlier than Phase 4; depends on self-host compile-time profiling.
- Whether a future opt-in trampoline should cover unknown closure tail calls; not required for this change and would trade allocations for stack safety.
- Cache eviction policy and default size cap; the change fixes immutability, atomic publication, and explicit cleaning, and defers automatic trimming until real usage data exists.
- Whether a future revision should support a remote or shared cache service; out of scope here.
- Whether `test` results should ever be cached by MyGO in addition to Go's own test cache; deferred.
