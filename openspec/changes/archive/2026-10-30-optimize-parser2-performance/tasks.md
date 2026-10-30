## 1. Baseline And Benchmarks

- [x] 1.1 Add `BenchmarkLexSource`, `BenchmarkParseSyntax`, and
  `BenchmarkLowerSyntax` to `internal/mygo/parser2`, each running a pinned
  real source fixture and a generated declaration-heavy fixture at 1×, 4×, and
  16×; verify with `go test -run '^$' -bench 'Benchmark(LexSource|ParseSyntax|LowerSyntax)$' -benchmem ./internal/mygo/parser2`
- [x] 1.2 Record the baseline table (ns/op, B/op, allocs/op, stage time per
  size) in the change notes and confirm the 6000-declaration stage time is
  reproducible within 15% across two runs
- [x] 1.3 Add a stack-regression test that parses a generated file with 12000
  top-level declarations; verify it currently fails with a stack overflow and
  is skipped under `-short`

Baseline (`-benchtime=3x -benchmem`, one run; stage time 2.22 s for 6000
declarations reproduced twice):

| Benchmark | 1× | 4× | 16× |
|---|---|---|---|
| `LexSource` real | 5.58 ms | 18.9 ms | 111 ms |
| `ParseSyntax` real | 10.2 ms | 49.9 ms | 236 ms |
| `LowerSyntax` real | 1.16 ms | 4.18 ms | 26.5 ms |
| `LexSource` decls | 34.6 ms (500) | 140 ms (2000) | 538 ms (8000) |
| `ParseSyntax` decls | 73.0 ms (500) | 358 ms (2000) | 1475 ms (8000) |
| `LowerSyntax` decls | 5.10 ms (500) | 27.9 ms (2000) | 138 ms (8000) |

`ParseSyntax` at 8000 declarations allocates 435 MB and 8.04 M objects for
411 KB of source; `LexSource` alone accounts for 538 ms and 155 MB of it.

The harness now lives in `benchmark_test.mygo` (generating
`zz_benchmark.gen_test.go`) rather than handwritten Go. It uses one benchmark
function per shape and size — `BenchmarkLexSourceReal1x`,
`BenchmarkLexSourceDecls8000`, `BenchmarkParseFileDecls8000`, and so on — with a
shared `bench*` helper, because sub-benchmarks would need a Go function value.
Re-running the same shapes through the MyGO harness reproduces the table above
within noise on the 4× and 16× cases.

## 2. Identifier Assignment

- [x] 2.1 Rewrite declaration identifier assignment as one index-bounded pass
  that keeps the existing reverse numbering order; verify
  `go test ./internal/mygo/parser2 ./internal/mygo/compiler` passes and the
  generated Go for the repository corpus is unchanged
- [x] 2.2 Apply the same treatment to statement and impl-method identifier
  assignment, preserving their forward numbering order; verify the differential
  corpus test reports the same counts as before the change
- [x] 2.3 Confirm the quadratic term is gone by benchmarking the
  declaration-heavy fixture at 1×/4×/16× and checking per-declaration cost is
  flat, then record the numbers

Per-declaration cost of the whole parse stage (`BenchmarkParseFile`) is flat:
177 µs at 500 declarations, 204 µs at 2000, 195 µs at 8000. The CLI stage for
6000 declarations dropped from 2.22 s to 1.73 s, while 1500 declarations moved
from 353 ms to 396 ms, so the growth curve is linear in declaration count.

## 3. CST Sibling Construction

- [x] 3.1 Change `DelimitedBuildResult` children to `Slice[CstElement]` and
  update every producer and consumer; verify `go build ./...` and the parser2
  unit tests pass after regeneration
- [x] 3.2 Restructure `buildDelimitedChildren` to take a sibling accumulator
  so ordinary sibling advancement is a tail call, keeping nested-region calls
  as ordinary recursion; verify nested-block lossless tests pass
- [x] 3.3 Verify the compiler emitted the trampoline for the builder by
  checking the generated Go contains its `__mygo_mt_` symbol, and add a test
  that fails when that symbol disappears
- [x] 3.4 Add a flat-sequence fixture with thousands of sibling elements and
  verify the builder no longer grows stack with element count

## 4. Heterogeneous Trampoline Lowering

The lexer keeps its current shape. The trampoline accepts the component instead:
its members differ only in parameter lists, share one result type, and call each
other in tail position.

- [x] 4.1 Relax component eligibility to require identical generic type
  parameters, identical `using` constraints, and identical lowered result
  types, ignoring parameter lists; verify with a MyGO test containing a
  tail-recursive cycle whose members take one and three parameters, whose
  generated Go must contain exactly one trampoline
- [x] 4.2 Give each member its own typed parameter slots named
  `__mygo_mt_s<state>_p<i>`, declare their union in the trampoline signature,
  and bind a case's slots to the member's parameter names on entry; verify
  `go build ./...` passes and the homogeneous output check stays clean
- [x] 4.3 Rewrite tail calls into the callee's slots and validate arity against
  the callee instead of the current member, keeping left-to-right argument
  staging; verify with a cycle whose arguments read the caller's parameters and
  with an argument that has a side effect
- [x] 4.4 Confirm the lexer component is trampolined: the generated
  `zz_syntax_lexer.gen.go` contains the trampoline symbol and no case body
  calls `lexStep`, then verify the 12000-declaration stack-regression test
  passes and record the 6000-declaration stage time
- [x] 4.5 Regenerate every package and confirm the already-emitted homogeneous
  trampolines and wrappers are byte-identical, with an empty `git diff` outside
  the intended files

The trampoline uses one frame struct per component rather than a widened
parameter list, so a member's slots are frame fields and each case binds them to
the member's own parameter names.  Two lexer functions (`lexEmitToken`,
`lexEmitTrivia`) had to declare the `using StringRuneIndex` constraint the rest
of the cycle already carried; without that the component's constraint lists
differ and eligibility still fails.  No scanner or driver was restructured.

Relaxed eligibility also trampolined 13 previously rejected cycles in
`typeinference2` (15 new frame structs).  No pre-existing trampoline definition
changed: `git diff` removes zero `func __mygo_mt_` lines.

## 5. Recovery Fast Paths

- [x] 5.1 Skip the CST rebuild when the diagnostic list is empty; verify valid
  sources produce identical trees and malformed-source tests still produce
  their diagnostics
- [x] 5.2 Index diagnostics by start offset once and use it for element
  lookup; verify diagnostic text, span, and ordering are byte-identical on the
  malformed-input corpus

## 6. Integration Validation

- [x] 6.1 Regenerate all `zz_*.gen.go`, rebuild the CLI, re-run `sync` with
  the new binary, and confirm the second generation is byte-identical (fixed
  point) and `git diff` shows only intended changes
- [x] 6.2 Run the parser2 unit tests, the differential corpus test, the
  formatter idempotence tests, and `go build ./...`; all must pass
- [x] 6.3 Compare final benchmarks against the 1.2 baseline and report the
  achieved speedup, allocation reduction, and the remaining hot spots in the
  change summary

Results (`-benchtime=3x`, same machine):

| Benchmark | Before | After | Speedup |
|---|---|---|---|
| `LexSource` 8000 declarations | 560 ms | 90.3 ms | 6.2× |
| `ParseSyntax` 8000 declarations | 1475 ms | 477 ms | 3.1× |
| `ParseFile` 8000 declarations | 1560 ms | 619 ms | 2.5× |
| `ParseFile` real/16× | 272 ms | 102 ms | 2.7× |

The CLI stage for 6000 declarations went from 2.22 s to 1.50 s, and the
12000-declaration file that used to overflow the stack now parses in 0.74 s.
Allocation per `ParseFile` at 8000 declarations fell from 467 MB to 405 MB.

Remaining hot spot: a single block with thousands of statements still costs
roughly quadratic time through `joinCstElements` inside `syntaxTypeRunsFrom`
(8000 statements in one block take about 8 s).  That code was a declared
non-goal of this change; the measurement is recorded here for a follow-up.
