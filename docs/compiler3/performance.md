# compiler3 performance

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
