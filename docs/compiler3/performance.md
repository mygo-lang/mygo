# compiler3 performance

## Self-hosted sync investigation (2026-11-01)

The executable's build backend and the backend selected at execution are
different choices. Both executables below ran **default codegen2** `sync .`;
one executable itself was generated with codegen3 O2. Measurements used Linux
x86-64, Go 1.27.1, a temporary repository copy and the same warmed `GOCACHE`.
They are individual runs, not statistical performance guarantees. The MyGO
codegen3 cache was used to build the self-hosted executable, not to run sync.

In an earlier unprofiled run the codegen2-built executable took 25.8 s and the
codegen3 O2-built executable took 59.3 s. Largest process RSS values were about
302 and 948 MiB respectively. Package-stage timing indicated type inference
was substantially slower in the self-hosted executable; timings include nested
dependency work and cannot be added together.

An instrumented full sync produced approximately 28.3 GB and 38.6 GB of
sampled cumulative allocations respectively. In the self-hosted profile,
`mtInsertString` and `Slice[String].Prepend` accounted for about 8.0 GB;
recursive insertion sorting in codegen2 mutual-tail-call analysis repeatedly
copied slices. A copied slice and `sort.Strings` removes that quadratic copy.
After rebuilding the O2 self-hosted compiler, sampled allocations were about
30.6 GB and the string-prepend allocation hotspot disappeared. CPU samples
were approximately 84.8 s before and 80.1 s after; these are samples across
threads and are not elapsed time. A subsequent paired, unprofiled run of the
self-hosted executable took 60.60 s before and 57.93 s after (one run each;
too little data to claim a stable speedup). Other large allocation sites include slice
append of AST expressions and environment bindings. This evidence supports a
sorting improvement but does not establish that sorting caused the gap between
the two compiler builds.

On this 64-bit platform, `unsafe.Sizeof` reported:

| Type | codegen2 | codegen3 O2 |
| --- | ---: | ---: |
| `ast2.ExprKind` | 16 | 208 |
| `ast2.MonoType` | 16 | 216 |
| `Option[ast2.MonoType]` | 16 | 240 |
| `ast2.Expr` | 136 | 552 |
| `ast2.Decl` | 16 | 96 |

The canonical enum stores all variant fields in a tagged Go value. Its copies,
and the GC scanning and allocation visible in the profile, remain an important
follow-up; these measurements do not isolate their causal contribution. Any
future layout change must update the representation version and verify FFI,
generics and cross-package behavior.

### Representation probe (task 9.2)

`tools/representationprobe` measures actual generated type sizes under both
backends and compares deliberately simplified current, compact,
pointer-per-variant, boxed variant-table and arena-index layouts. The source is
`tools/representationprobe/representation_test.mygo`. Run
`go test ./tools/representationprobe -run '^TestRepresentationSizes$' -v` for
the default backend, and
`./mygo --codegen3 --opt-level=2 test ./tools/representationprobe -run '^TestRepresentationSizes$' -v`
for codegen3. The benchmark names are
`BenchmarkSyntheticCurrent`, `BenchmarkSyntheticCompact`,
`BenchmarkSyntheticBoxed`, `BenchmarkSyntheticArena` and
`BenchmarkExprSliceCopy`.

The current synthetic 64-bit layout is 272 bytes with codegen2 and 144 bytes
with codegen3; compact, boxed, arena-handle and payload layouts are 24, 24, 16
and 48 bytes under both backends. Three 100 ms samples of the 4,096-node
construction-and-copy workload produced:

| Backend/layout | Time | Allocated bytes | Allocations |
| --- | ---: | ---: | ---: |
| codegen2 current | 2.146–2.185 ms | 5.69 MB | 4,113 |
| codegen2 compact | 229–236 µs | 649 KB | 4,112 |
| codegen2 boxed | 532–542 µs | 1.697 MB | 8,208 |
| codegen2 arena | 201–211 µs | 1.004 MB | 32 |
| codegen2 `Expr` copy | 76–101 µs | 557 KB | — |
| codegen3 current | 1.021–1.097 ms | 3.254 MB | 4,113 |
| codegen3 compact | 199–216 µs | 649 KB | 4,112 |
| codegen3 boxed | 407–424 µs | 1.173 MB | 8,208 |
| codegen3 arena | 204–215 µs | 1.004 MB | 32 |
| codegen3 `Expr` copy | 267–305 µs | 2.261 MB | — |

The boxed table retains statically typed variant fields but incurs an extra
allocation per node in this model. The compact model in this first probe
assumes all variants have the same typed payload and therefore does not solve
heterogeneous payload access. The arena model retains its pool along with
handles and still needs a safe lifetime and public-ABI design.

A second checked-in Go benchmark models heterogeneous variants with a shared
typed payload slot (the earlier string-tag layout plus `any`). Payloads include scalar, string
and slice fields, so the selected payload is large enough to escape in both
representations. Run it with:

```bash
GOCACHE=/tmp/mygo-gocache go test ./tools/representationprobe \
  -run '^TestRepresentationTradeoff$' \
  -bench '^BenchmarkRepresentation(Compact|Wide)$' \
  -benchmem -benchtime=100ms -count=3
```

On Go 1.26.0/linux amd64, the wide value is 208 B and the compact value is
32 B. Across three samples, wide construction, copy and selected-payload read
took 2.34–2.55 ms and allocated about 1.90 MB in 4,098 allocations; compact
took 1.15–1.21 ms and allocated about 459 KB in the same 4,098 allocations.
The compact payload assertion rejects the wrong concrete variant type, and
copy tests confirm payload values retain value-copy behavior. These
measurements include construction, copying and reading, but do not isolate GC
pause time or compiler self-host cost. Existing self-host profiles report
about 2.14 GB of cumulative sampled allocation at `Expr` appends and identify
GC scanning and `memmove` as hot work. This supports reducing copied enum
width but does not predict the exact end-to-end gain. Task 9.3 must measure the
real generated representation and paired self-host workload before this
choice is accepted.

The implementation now uses one-based integer ordinals from enum declaration
order for every enum, reserving zero as invalid. The discriminator width is
`uint8` through 255 variants, `uint16` through 65,535, then `uint32`; the
generated field name avoids variant field collisions. A compact enum stores
one GC-visible `any` holding its concrete variant value when an enum's
estimated wide value exceeds 24 B on 64-bit Go. Large recursive enums may use
this layout too; the interface payload breaks direct and mutual Go type cycles.
Small recursive enums retain pointer-node layout. Generic instantiations remain
concrete dynamic Go types. Matching dispatches on the ordinal before asserting
the selected payload type. Nullary variants carry no payload. Recursive values
remain finite because either payload pointer or interface payload adds
indirection. The compact
wrapper measures 24 B on linux/amd64 versus 32 B with a string discriminator.
The isolated scan and allocation effects are measured below; the paired
self-host result is not yet sufficient for acceptance. Imported ordinal and compact decisions are derived from dependency
source declarations. The model estimates GC consequences from allocation
bytes and pointer count; no standalone GC pause measurement is available yet.

An isolated Go probe using identical payloads measured the representation
effect on Go 1.26.0/linux amd64. Scanning 4,096 values by string tag took
4.30–4.58 µs; scanning by `uint8` ordinal took 3.28–3.45 µs (five 200 ms
samples, no allocations). The full construction/copy/read loop was nearly
time-neutral at 0.985–1.030 ms for string tags and 0.988–1.016 ms for ordinals,
but allocations fell from 458,752 B to 393,216 B per operation. This separates
the cheaper discriminant scan from the allocation-heavy surrounding work. The
reproducible probe is `BenchmarkRepresentationCompactTagScan`,
`BenchmarkRepresentationOrdinalTagScan`, `BenchmarkRepresentationCompact` and
`BenchmarkRepresentationOrdinal` in `tools/representationprobe`.

Two paired `sync internal/mygo/ast2` runs on fresh source copies compared the
current codegen2-built and codegen3 O2-built binaries. Medians were 3.10 s and
3.43 s respectively; their source trees were identical, but the codegen3
binary was slower on this workload. Comparing the current O2 binary against
the workspace's older O2 binary (built 2026-11-01) gave 3.31–3.43 s versus
4.52–4.65 s and 66–67 MiB versus 108–110 MiB RSS. Those binaries came from
different compiler source snapshots, so the change cannot be attributed to
integer tags. Raw commands, logs, source copies and binary hashes are under
`/tmp/mygo-enum-paired2` and `/tmp/mygo-enum-before-after`. These are small
workload diagnostics, not a full-repository result; 9.3 remains open pending a
same-revision, repeated full self-host comparison and the complete correctness
matrix.

The estimator treats generic parameters as zero-sized when choosing a
declaration-wide layout. This conservative lower bound avoids making a generic
enum's representation depend on its instantiation sites. The algorithm uses
representation version 4, so cache entries from the earlier estimate are
rejected.

Before integer ordinals (representation version 3), actual generated sizes in
the compact O2 output on Go 1.26.0/linux amd64 were measured with `unsafe.Sizeof`
inside a temporary test compiled through the codegen3 overlay:

| Type | codegen2 | codegen3 O2 |
| --- | ---: | ---: |
| `ast2.ExprKind` | 16 B | 32 B |
| `ast2.MonoType` | 16 B | 32 B |
| `Option[ast2.MonoType]` | 16 B | 56 B |
| `ast2.Expr` | 136 B | 192 B |
| `parser2.CstElement` | 16 B | 40 B |
| `parser2.CstNode` | 96 B | 104 B |

The paired full-repository `sync .` workload was run three times in alternating
order on the same Linux amd64 host (EPYC 7601), using Go 1.26.0, one warmed
`GOCACHE`, temporary checkout copies and `GOMAXPROCS` unset. Both executables
ran default codegen2 sync; only the compiler executable's build backend
differed. Codegen2-built wall times were 73.34–75.64 s (median 74.66 s) with
183.36–192.65 s CPU; codegen3 O2-built wall times were 99.43–100.98 s (median
99.54 s) with 187.49–194.17 s CPU. This run shows no self-host performance
gain from the compact representation and a slower wall time under these
conditions. It does not satisfy task 9.3's performance criterion, so that task
remains open while the cost is investigated. The earlier narrow `ast2` workload
also showed no gain (2.91–2.99 s vs. 3.73–3.81 s wall time).

The same-revision two-run `internal/mygo/ast2` comparison narrows the runtime
regression to front-end work. `--bootstrap-timing` logs show parser2 at
337–343 ms for the codegen2-built compiler versus 420–463 ms for the codegen3
O2-built compiler. Type inference took 620–655 ms versus 828 ms. In contrast,
the codegen2 emission stage took 541–546 ms versus 478 ms. Imports/FFI time was
similar (332–352 ms versus 339–345 ms). These stage timings are nested and
should not be summed, but they show that the slower total is already present
before emission. The raw logs and paired measurements are in
`/tmp/mygo-enum-paired2`.

The generated-size probe under representation version 4 reported 200 B
for `ast2.ExprKind`, 24 B for `ast2.MonoType`, 40 B for
`Option[ast2.MonoType]`, 344 B for `ast2.Expr`, 24 B for
`parser2.CstElement`, and 88 B for `parser2.CstNode`. Codegen2 reports 16 B,
16 B, 16 B, 136 B, 16 B, and 96 B respectively. This supersedes the earlier
version-3 size table above.

`ExprKind` has 24 variants and is recursive. The version-4 recursive enum layout
stores a tag plus a pointer slot for every variant, so its 200 B size is
consistent with 24 pointer slots and alignment. `Expr` embeds this enum and
`Option[MonoType]`, reaching 344 B. That gives a concrete explanation for the
parser-stage regression and for increased copying and GC scanning during
front-end work.

The existing 4,096-element `Expr` slice-copy probe measures this cost directly
on the same host. Codegen2 took 0.510–0.649 ms/op and copied 557,056 B/op;
codegen3 O2 took 1.516–1.780 ms/op and copied 1,409,033–1,409,035 B/op. Each
operation made one allocation. This isolates slice-copy cost, not parser
allocation or full compiler time, but confirms a 2.5x copied-byte increase
and about a 2.8x time increase for this operation.

### Recursive compact-payload prototype (representation version 5)

The representation estimator now allows any enum, including a recursive enum,
to use `Tag + any` when its estimated pointer-node value exceeds 24 B. Small
recursive enums keep their per-variant payload pointers. The interface payload
breaks direct and mutual recursive Go type cycles; constructors still store
concrete named payload structs, and matches dispatch by tag before asserting
the selected payload. This is a type-size rule applied uniformly, with no
repository or enum-name decision beyond the existing FFI layouts.

The same generated-size probe now reports codegen3 O2 sizes of 24 B for
`ExprKind`, 24 B for `MonoType`, 40 B for `Option[MonoType]`, and 168 B for
`Expr`; codegen2 reports 16/16/16/136 B. `CstElement` and `CstNode` measure
24/88 B under codegen3 O2 versus 16/96 B under codegen2. The 4,096-element
`Expr` copy probe fell to 568–696 µs/op and 688,129–688,131 B/op, still one
allocation. This is about half the bytes and roughly 2.2–3.1x faster than the
version-4 codegen3 copy probe on this host; it is close to the codegen2 copy
time of 545–655 µs while still copying about 24% more bytes.

Codegen3 O2 tests for `ast2`, `typeinference2`, and `codegen3` passed; the
parser2 large-flat-input test passed in 26.2 s. The full parser2 test invocation
did not complete in the available run and is not counted as passing. A two-run
paired `sync ./internal/mygo/ast2` comparison measured 3.01–3.10 s for the
codegen2-built compiler and 3.31–3.47 s for the version-5 codegen3-built
compiler. Against the earlier version-4 codegen3 samples (3.43 s), that is too
small and noisy to claim an end-to-end speedup. Stage timing suggests parser2
improves modestly, while type inference remains around 814–835 ms; the
large-`Scheme` environment lookup cost remains a separate lead. Raw version-5
paired logs are in `/tmp/mygo-repr5c-paired-ast2`.

The environment lookup benchmark independently reports oldest-binding lookup
at 5.46–5.68 µs for codegen3 O2 versus 0.97–1.02 µs for codegen2, with the
report attributing that path to full `Scheme` copies during batch-map probes.
These measurements point to two separate costs: wide recursive AST values in
parser work and large scheme values during inference. They do not yet apportion
the end-to-end gap between those costs.

One generic layout candidate follows from Go's trailing zero-sized-field rule.
Before representation version 6, the emitter wrote the tag first and variant
payload fields in source order; `Option[A]` therefore ended in `None struct{}`
after `Some struct{F0 A}`.
On amd64, a small Go-only model measured this ordering at 24 B for `A=uint64`
40 B for a 24 B payload, and 48 B for a 32 B payload. Moving the discriminator
to the last field reduced each model by 8 B (to 16 B, 32 B, and 40 B).
The 24 B model matches the measured version-4 `MonoType` size. Version 6 now
emits the discriminator after variant fields for every non-compact enum,
without a type-name condition. The generated-size probe confirms
`Option[MonoType]` drops from 40 B to 32 B and `Expr` from 168 B to 160 B.
Constructors and matches use named fields, while tag IDs remain derived from
source declaration order. A codegen3 O2 full `typeinference2` test run passed,
including the Option/Result and FFI cases. The `Expr` copy benchmark moved from
688,129–688,131 B/op under version 5 to 655,361–655,362 B/op under version 6;
time ranged from 571–1,253 µs/op with a 676 µs median, too noisy to claim a
time improvement. A new six-run paired `sync ./internal/mygo/ast2` comparison
on Go 1.26.0/Linux amd64 used the same temporary source copy and warmed Go
cache, alternating current codegen2-built and version-6 codegen3 O2-built
compilers. Both ran default codegen2 sync. Codegen2 wall time was 3.002–3.083 s
(3.021 s median), versus 2.938–3.078 s (2.963 s median) for codegen3 O2; user
CPU medians were 5.371 s and 4.827 s. Peak RSS medians were 58,972 KiB and
59,504 KiB. This is a 1.9% wall-time and 10.1% user-CPU improvement on this
narrow package workload, with slightly higher RSS. Stage logs show parser2
still slower (median 365 ms versus 336 ms), type inference about even (623 ms
versus 625 ms), and codegen faster (494 ms versus 537 ms). This is encouraging
but does not establish a full-repository sync improvement. Raw paired data is
in `/tmp/mygo-repr6b-paired-ast2`.

### Alignment-aware enum size estimate (representation version 7)

The compact-layout estimator now includes alignment between fields in each
variant payload struct. It uses a conservative size for generic parameters
because an instantiation can be arbitrarily large; generated payload types
remain concrete. This can compact small generic instantiations as a tradeoff
for avoiding large inline values. The representation version is 7. Targeted
IR and codegen3 tests pass. The paired results and remaining coverage limits
are recorded below.

Version 7 `ir3` and `codegen3` tests, a codegen3 O2 `typeinference2` test run,
and the generated-size probe pass. Sizes remain 24/24/32/160/24/88 B for
`ExprKind`/`MonoType`/`Option[MonoType]`/`Expr`/`CstElement`/`CstNode`.

Three alternating paired runs of `sync ./internal/mygo/ast2` on a temporary
source copy used the same warmed Go cache; both binaries ran default codegen2
sync. Codegen2 wall times were 2.978/3.115/2.909 s (2.978 s median) and user
CPU times were 5.421/5.623/5.433 s (5.433 s median). Codegen3 O2 wall times
were 2.951/2.949/2.946 s (2.949 s median) and user CPU times were
4.780/4.816/4.787 s (4.787 s median). This is about 1.0% lower median wall
time and 11.9% lower median user CPU for this single-package workload. RSS was
not captured, and the previous version-6 pair had similar results; this does
not establish that the estimator change itself improved performance or that a
full-repository build is faster. Raw runs are in `/tmp/mygo-repr7-paired.txt`.

The size probe uses one small `go[Int]` boundary around
`unsafe.Sizeof(Zero[T]())`. This use of `unsafe.Sizeof` is safe: it evaluates
the static layout of a zero value and neither dereferences nor mutates data. It
includes struct padding, excludes referenced heap storage and is
architecture-dependent. The inline boundary remains because the current MyGO
surface cannot yet resolve `unsafe.Sizeof` as a direct member. The probe still
does not compare real implementations, end-to-end self-host cost, or FFI,
recursive, generic and cross-package behavior, and it does not independently
select a replacement ABI. Task 9.2's comparison is complete; task 9.3 remains
open pending the full workload and correctness matrix.

### Enum declaration lookup (task 9.5)

`PkgInfo` and each `MyGoPackageInfo` now carry a source-derived enum index
built once when the package value is created. The index stores type parameters
and variants by enum name, keeps the first declaration for duplicate names,
ignores type aliases, and is scoped to one package path and alias. Test-created
package values that intentionally omit an index use an explicit `None` and
retain the linear fallback; production source construction supplies the index.
Because the index is part of the immutable package value, changing source
declarations produces a different value rather than reusing stale index state.
Generic, missing-name, duplicate-first, alias, package-scope, qualified-enum,
imported-scope and backend-parity tests cover these cases.

Three 100 ms samples on Go 1.27.1/amd64 scanned 1,024 enum declarations for a
missing name in 3.88–4.04 µs with codegen2 and 62.8–63.9 µs with codegen3 O2,
with no allocations. The indexed miss took 9.45–9.60 ns with codegen2 and
57.7–58.1 ns with codegen3 O2, also with no allocations. Building the index
over those 1,024 enums took 109–111 µs, 299,305 B and 22 allocations with
codegen2, and 157.5–159.6 µs with the same bytes and allocations under
codegen3 O2. The measured codegen2 break-even is about 28 missing-name
lookups; codegen3 reaches it in roughly three lookups. The index trades bounded
package-local memory for removing repeated source-order scans.

### Struct literal completeness and spans

While validating the index implementation, typeinference2 was found to accept
omitted, duplicate and unknown struct-literal fields; omitted fields were
silently filled with Go zero values. The type checker now requires the exact
declared field set for MyGO structs and Go FFI structs, reporting missing,
duplicate and unknown fields. Embedded fields are represented through their
resolved field names, so literals initialize them explicitly. This prevents an
invalid canonical-ADT zero value from entering a struct.

`Bind.Type` and `Bind.TypeSpan` are now both optional: an absent annotation is
represented by `None` instead of a synthetic empty `SourceSpan`. The parser no
longer exposes an `emptySpan()` helper. Existing AST reconstruction preserves
the original span, while code paths without a source node use an explicit
`Option` or a real boundary position. The new validator and span model pass
the complete repository self-host `sync .` and `go test ./...`.

### Persistent environment (task 9.4)

Local environments still form immutable batches of at most 16 bindings.
Unsealed local batches use a linear lookup over their small entry slice and do
not rebuild a map after each `Put`; the sixteenth entry builds the batch index
once. Imported batches keep their prebuilt map. `ToSlice` collects each layer's
entries as a slice chunk and calls the prelude's allocation-once `Flatten`,
preserving the newest-first order without per-entry `Get` and `Append`.

Before this change, three 100 ms codegen2 samples for a 512-binding environment
took 17.80–18.44 µs, 98,720 B and 44 allocations for `Env.ToSlice`, and
2.26–2.30 µs, 4,256 B and 47 allocations for a `Put` into a partial local
batch. The codegen2 result after the change is:

| Benchmark | Before | After |
| --- | ---: | ---: |
| partial-batch `Env.Put` | 2.26–2.30 µs; 4,256 B; 47 allocs | 317–323 ns; 1,112 B; 6 allocs |
| sealing a 16-entry batch | included in every rebuild | 2.155–2.177 µs; 6,544 B; 13 allocs |
| starting a new batch | — | 182.8–188.3 ns; 296 B; 6 allocs |
| `Env.ToSlice` | 17.80–18.44 µs; 98,720 B; 44 allocs | 6.276–6.649 µs; 50,792 B; 7 allocs |
| recent lookup | 87.7–91.0 ns; 128 B; 2 allocs | 95.4–96.1 ns; 128 B; 2 allocs |
| oldest lookup | 972–1,021 ns; 128 B; 2 allocs | 992–1,011 ns; 128 B; 2 allocs |

The codegen3 O2 build also passed the same benchmarks. Its partial-batch `Put`
was 640–691 ns, 3,648 B and 2 allocations; sealing a batch was 4.58–4.70 µs,
11,560 B and 25 allocations; starting a batch was 300–306 ns, 736 B and 2
allocations. `ToSlice` was 34.2–35.8 µs, 157,288 B and 7 allocations. Recent
lookup was 180–182 ns with no allocation, while oldest lookup was 5.46–5.68 µs
with no allocation. The codegen3 old-name cost is dominated by copying the
canonical `Scheme` value through full-batch map probes; reducing it depends on
the representation decision tracked by tasks 9.2 and 9.3.

New tests cover newest-first `ToSlice` order, nested shadowing, sibling
isolation and preserved snapshots. Existing tests cover cached free-variable
generalization, long local binding chains and imported scope overlays.

The optimized compiler passed the targeted mutual-tail-call sort, backend
differential, guarded tail switch and generated-output determinism tests. A
codegen3 O2 self-hosted compiler was rebuilt and completed `sync .` successfully.

### Source provenance microbenchmark

On Linux amd64, Go 1.27.1, Ryzen 7 7840HS, three independent single-iteration
runs of `go test ./internal/mygo/codegen2 -run '^$' -bench
'^BenchmarkApplyLineDirectives$' -benchtime=1x -count=3` on 10,000 alternating
source/support directives took 34.19–35.01 ms before and 27.03–29.22 ms after
replacing repeated prefix scans with one ordered line/column scan. Both used
about 27 MB and 190k allocations per operation. This is a synthetic local
benchmark, not a measured end-to-end sync improvement. Existing source mapping
and determinism tests and new multiline, byte-column and repeated-node tests
passed.

To reproduce profiles without changing the checkout, copy the repository to a
temporary directory, exclude `.git` and existing binaries, and add the following
temporary Go file as `cmd/mygo/perf_temp.go`. Add `defer stopPerformanceProfile()`
at the start of `main` in that copy. Successful runs write CPU and sampled heap
profiles to the paths selected by the environment variables:

```go
package main

import ("os"; "runtime"; "runtime/pprof")

var perfCPU *os.File
func init() {
    if p := os.Getenv("MYGO_PERF_CPU"); p != "" {
        perfCPU, _ = os.Create(p)
        if perfCPU != nil { _ = pprof.StartCPUProfile(perfCPU) }
    }
}
func stopPerformanceProfile() {
    if perfCPU != nil { pprof.StopCPUProfile(); _ = perfCPU.Close() }
    if p := os.Getenv("MYGO_PERF_MEM"); p != "" {
        runtime.GC()
        if f, err := os.Create(p); err == nil { _ = pprof.WriteHeapProfile(f); _ = f.Close() }
    }
}
```

Build a baseline with `go build -o /tmp/mygo-profile2 ./cmd/mygo`, then build
the other executable with `/tmp/mygo-profile2 --codegen3 --opt-level=2 build
-o /tmp/mygo-profile3 ./cmd/mygo`. Use the same writable `GOCACHE` for both,
warm it first, and run each in the same copy with `MYGO_PERF_CPU` and
`MYGO_PERF_MEM` set to distinct files. Inspect CPU and `alloc_space` with Go
pprof. To obtain sizes, build a temporary Go command in that copy with each
backend and print `unsafe.Sizeof(*new(ast2.ExprKind))`, `ast2.MonoType`,
`prelude.Option[ast2.MonoType]`, `ast2.Expr`, and `ast2.Decl`. Do not compare
profiled wall time with unprofiled wall time.

`tools/profile_selfhost.py` automates an isolated workload copy and paired runs
using two binaries built with the profiling hooks above:

```bash
python3 tools/profile_selfhost.py --codegen2 /tmp/mygo-profile2 \
  --codegen3 /tmp/mygo-profile3 --output /tmp/mygo-profile-results \
  --workload examples/main --count 2
```

Build both binaries from the same source revision and settings before using the
tool; it records their hashes but does not prove their sources match. The output
directory must not exist and must be outside the checkout. `metadata.json`
records workload and toolchain information; `runs.json` contains per-run wall,
CPU and maximum RSS measurements, and `summary.json` gives medians and ranges.
Separate `timed` and `profiled` runs retain GC traces, stage logs and raw
CPU/heap profiles. Set `MYGO_PROFILE_TIMING=1` to enable codegen3 front-end,
IR phase and per-pass timings; the harness sets it only for profiled runs.
Timings in `runs.json` are nanoseconds and `optimize` includes its individual
pass timings, so these values must not be summed together. Warm cache hits can
have no lowering or pass timings. The tool tests O0/O1/O2 with
fresh cold, immediately warm and disabled caches; it requests debug emission
outside the copied checkout and verifies that codegen3 did not change generated
Go in the copy. For cache-disabled profiled runs, generated size is summed from
the generator's timing log because `MYGO_CACHE=off` currently does not populate
`--codegen3-emit`; the record identifies the measurement source. A null size
means it was unavailable, not zero. The debug copy and GC tracing add overhead
to those runs. The process resource figures are those reported by `wait4`, not
a sum of maximum RSS across the process tree. Keep raw output outside the source
tree and retain it with the binary hashes for later comparisons.

One harness validation used `internal/mygo/ast2` with two repetitions on
Linux/amd64 and Go 1.27.1. Default-codegen2 sync elapsed medians/ranges were
0.645 s (0.633–0.658 s) for the codegen2-built compiler and 1.316 s
(1.306–1.325 s) for the codegen3 O2-built compiler. The 44 raw runs, profiles,
GC logs, phase/pass samples and metadata for that local run are under
`/tmp/mygo-profile-ast2-current`; recreate them with the command above and
`--workload internal/mygo/ast2`. The binaries were built from the same
instrumented temporary source tree; this is a small-package baseline, not a
replacement for measuring full `sync .` or a performance guarantee.

Recorded on 2026-10-30 against `docs/compiler3/baseline.md`.

Environment:

- Linux x86-64
- Go `go1.27.1 linux/amd64`
- AMD Ryzen 9 PRO 7940HS

## End-to-end compiler generation

The codegen3 benchmark copies the prelude into a temporary module and disables
`MYGO_CACHE`, so every iteration performs a cold generation without reading or
writing generated files in this checkout:

```bash
go test ./internal/mygo/compiler \
  -run '^$' \
  -bench '^BenchmarkBootstrapIR3Codegen3$' \
  -benchtime=1x -count=1
```

| Level/workload | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| O0 option-list | 605,210,675 | 253,884,400 | 2,712,799 |
| O0 dictionary | 523,208,338 | 196,278,752 | 2,684,829 |
| O0 tail-recursion | 560,476,187 | 195,816,032 | 2,681,303 |
| O1 option-list | 493,271,607 | 197,099,680 | 2,693,216 |
| O1 dictionary | 529,624,185 | 196,674,048 | 2,685,230 |
| O1 tail-recursion | 527,656,980 | 196,472,568 | 2,683,716 |
| O2 option-list | 534,473,834 | 197,446,832 | 2,695,271 |
| O2 dictionary | 514,809,115 | 196,849,440 | 2,686,953 |
| O2 tail-recursion | 579,598,369 | 196,614,016 | 2,685,402 |

The Phase 1 codegen2 reference is 588–596 ms/op, 215–216 MB/op, and about
3.33 million allocations/op. The isolated codegen3 runs use about 196–254 MB/op
and 2.68–2.71 million allocations/op. Single-iteration timings vary enough that
the O0/O1/O2 wall-time order is not used as a pass-specific claim; use a larger
`-benchtime` for optimization-level comparisons.

## Generated compiler compile time

The same current `cmd/mygo` source was built through codegen2 and codegen3 O2,
then each resulting compiler was used for a full `sync .`:

| Compiler implementation | Wall time | User CPU |
| --- | ---: | ---: |
| codegen2-built compiler | 72.9 s | 302.7 s |
| improved codegen3 O2 compiler | 82.5 s | 153.1 s |
| codegen3 O2 before large-payload fix | 313 s | 674 s |

The large-payload fix uses indirect variant payloads for non-recursive enums
whose variants contain declared value types. In the compiler AST this reduces
`ast2.Decl` from 3560 bytes to 96 bytes and removes repeated multi-kilobyte
copies and GC scans.

The parser2 cold-sync comparison after the fix is:

| Compiler implementation | Wall time: parser2 sync |
| --- | ---: |
| codegen2-built compiler | 8.8 s |
| codegen3 O2 before large-payload fix | 68.1 s |
| improved codegen3 O2 compiler | 11.5 s |

## Stack depth

The codegen3 O2 stack test compiles and runs self and mutual tail recursion at
depth 200,000:

```bash
go test ./internal/mygo/compiler \
  -run '^TestBootstrapIR3Codegen3StackDepth$' \
  -count=1
```

The test passes. These calls therefore remain bounded under the current
`Tailcall3` lowering.

## Optimizer pass accounting

### Codegen3 phase and pass timing (task 9.7)

Cold codegen3 runs over `internal/mygo/ast2` on Go 1.27.1/amd64, with
`MYGO_PROFILE_TIMING=1` and separate `MYGO_CACHE` roots, attributed the
following median-stage measurements. Values are single-run diagnostic samples;
they are not additive across phases because nested dependency work is included.

| Phase/pass | O0 | O1 | O2 before Inline3 fix |
| --- | ---: | ---: | ---: |
| prepare | 0.090 ms | 0.096 ms | 0.134 ms |
| lower | 13.965 ms | 14.671 ms | 16.859 ms |
| link | 4.081 ms | 5.333 ms | 4.721 ms |
| `Match3` | — | 1.817 ms | 1.437 ms |
| `Simplify3` | — | 1.653 ms | 2.520 ms |
| `Tailcall3` | — | 32.596 ms | 33.453 ms |
| `Unbox3` | — | — | 2.215 ms |
| `Closure3` | — | — | 3.447 ms |
| `Inline3` | — | — | 35.801 ms |
| `Specialize3` | — | — | 11.131 ms |
| optimize total | 0.000351 ms | 36.114 ms | 90.222 ms |
| verify | 0.803 ms | 0.929 ms | 0.766 ms |
| emit | 148.010 ms | 139.562 ms | 128.111 ms |
| generated bytes | 242,907 | 243,000 | 243,376 |

`Inline3` was the largest O2-only pass. Planning previously walked every
function body twice to obtain the same direct-call names, once for the
recursive-call graph and again to decide whether a function called an
inlineable leaf. The pass now computes an immutable direct-call index once and
shares it between recursive analysis, leaf classification and rewrite
selection. Functions that call no inlineable leaf retain their original body
instead of being copied node by node.

Four independent cold-cache runs after the change took 25.342–31.893 ms in
`Inline3` and 63.115–82.733 ms for the full O2 optimize phase, compared with
the single pre-change sample of 35.801 and 90.222 ms. Generated size remained
243,376 bytes. The full optimizer suite, codegen3 differential behavior test,
200,000-call stack-depth test and repository-wide determinism checks pass.

| Pass | Measured result |
| --- | --- |
| `Match3` | Correctness and decision-tree compaction covered by unit tests; per-pass allocation delta not isolated. |
| `Simplify3` | Constant-fold, copy, DCE, CSE, and hoisting behavior covered by unit tests; end-to-end allocation delta included above. |
| `Tailcall3` | Measured at 200,000 recursive calls without stack growth. |
| `Unbox3` | Local Option/Result and non-escaping Ref behavior covered by tests; end-to-end allocation delta included above. |
| `Closure3` | Capture and closed-lambda lifting covered by tests; compile-time delta deferred because representative workloads do not contain enough closures for a stable measurement. |
| `Inline3` | Budget, shadowing, and recursion exclusion covered by tests; real-workload inlining delta deferred until compiler hot-spot inlining is introduced. |
| `Specialize3` | Known-dictionary specialization and safety exclusions covered by tests; representative workload delta is below single-run noise. |

Single-run O0/O1/O2 values are retained for trend checking, not as optimization
guarantees.

### Warm codegen3 front-end cache (task 9.8)

The package cache was previously checked after type inference, so a warm hit
avoided IR generation but still repeated inference. The compiler now computes
the package key after parsing, dependency discovery and Go FFI collection, then
checks the verified cache entry before inference. A hit skips type inference
and code generation. The key also includes Go target and build environment
inputs (`GOFLAGS`, target architecture settings, cgo, GOROOT and toolchain
selection) along with the existing compiler, representation, source,
dependency, module and optimization inputs. Parsing and dependency discovery
remain in the warm path.

On Linux/amd64 with Go 1.27.1, two baseline runs and one updated run of O2
`sync internal/mygo/ast2` gave these end-to-end times:

| Cache mode | Before | After |
| --- | ---: | ---: |
| cold | 4.238–4.265 s | 4.315 s |
| warm | 2.270–2.435 s | 1.409 s |
| off | 4.274–4.401 s | 4.388 s |

The before warm run reported 244 ms of prelude inference and 629 ms for
`ast2`; after the change neither package logged a `typeinference2` stage on a
warm hit. Cold work remains unchanged within run variation, and cache-off
continues through inference. These are local measurements on one package, not
a full-repository speed guarantee. Recreate the measurements by running with
`--bootstrap-timing` and `MYGO_PROFILE_TIMING=1` against a fresh cache, then
repeating with that cache. The before values are two repetitions; the
post-change value is one sample.

The cache regression tests cover changed-source invalidation, generated-file
exclusion, Go build-environment invalidation, corrupt-entry recovery,
cross-module sharing, locking, disabled cache, and generated-output placement.
The compiler package tests pass through the codegen3 backend.

### Verified warm early hits and indexed emission

The previous warm path still parsed every package and collected Go FFI
signatures before consulting the output cache. The compiler now publishes a
checksummed summary keyed by a cheap source fingerprint. The summary records
recursive dependency input keys and the generated-output key, while importer
metadata is stored in the cache entry. A verified hit therefore skips parsing,
FFI collection, inference, IR lowering, optimization, and code generation.
Summary validation covers MyGO sources, transitive imports, external tests,
module/toolchain options, hand-written Go inputs, and local replacements.

Per-file emission now builds declaration/import indexes and package helper data
once. `GenerateFile` remains as the single-file compatibility entry point and
produces byte-identical output.

Measurements were taken on Linux/amd64 with Go 1.27.1 and
`--codegen3 --no-prelude --opt-level=2`. The workload had eight MyGO files with
one function each. End-to-end time is dominated by the Go toolchain and is
reported to one decimal millisecond; the MyGO stage is the meaningful
work-avoidance signal.

| Warm measurement | Before | After |
| --- | ---: | ---: |
| total MyGO stage | 1.263 ms | 0.223 ms |
| parser2 | 0.643 ms | absent |
| Go FFI collection | 0.002 ms | absent |
| type inference | warm cache hit | absent |
| end-to-end (3 samples) | 0.067–0.071 s | 0.069–0.071 s |

The all-cache-disabled O0/O1/O2 IR benchmark was also sampled before the change:
O0 workloads took 0.917–1.002 s and 10.95–10.98M allocations; O2 workloads took
0.863–0.880 s. The identical post-change command reported O0 workloads at
0.783–0.829 s and O2 workloads at 0.785–0.798 s, with 10.96–10.99M allocations.
The cold work remains in the same range because the new path adds summary
validation and metadata generation rather than changing the optimizer.

Controlled emission scaling used 128 declarations, five iterations, and only
the source-file count varied:

| Files | Indexed emission | Single-file-entry rebuild |
| ---: | ---: | ---: |
| 1 | 2.76 ms / 22,361 allocs | 2.65 ms / 22,358 allocs |
| 4 | 2.70 ms / 22,999 allocs | 2.85 ms / 23,517 allocs |
| 16 | 2.64 ms / 24,959 allocs | 3.37 ms / 28,089 allocs |
| 64 | 3.00 ms / 30,392 allocs | 5.67 ms / 44,950 allocs |

The 64-file case reduced emission time by about 47% and allocations by about
32% relative to rebuilding the package context for each file. These are local
single-run trend measurements, not guarantees. Recreate them with:

```sh
go test -run '^$' -bench 'BenchmarkCodegen3FileScaling|BenchmarkCodegen3SingleFileEntry' \
  -benchmem -benchtime=5x ./internal/mygo/codegen3
go test -run 'TestCodegen3Summary' -count=1 ./internal/mygo/compiler
```
