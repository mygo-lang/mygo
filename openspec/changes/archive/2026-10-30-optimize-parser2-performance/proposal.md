## Why

The bootstrap parse stage runs at roughly 0.5 MB/s: 310 KB of source with 6000
top-level declarations costs 2.2 s, and 626 KB aborts with a goroutine stack
overflow inside the lexer. Profiling shows the grammar is not the cost.
Structural defects dominate. AST identification rebuilds every declaration and
statement list by prepending into a fresh array (quadratic). The CST sibling
walk grows one frame per element because its recursive call is not in tail
position. The lexer grows one frame per token even though every call in its
cycle is a tail call: the trampoline declines the component because its members
take different parameter lists. Each defect has a local fix that keeps the
existing recursive style and the existing observable output.

## What Changes

- Tail-call lowering: teach the mutual-tailcall trampoline to accept a
  component whose members share result types, type parameters, and `using`
  constraints but take different parameter lists. The trampoline carries one
  typed slot per member parameter and each member binds its own slots, so the
  lexer keeps its current recursive shape and still runs in constant Go stack.
  Components that already share one ABI keep their generated output byte for
  byte.
- CST construction: `buildDelimitedChildren` carries an accumulator for
  already-built siblings so advancing to the next sibling becomes a tail call,
  and `DelimitedBuildResult.Children` becomes `Slice[CstElement]` so that path
  stops round-tripping through `List`.
- AST identification: declaration, statement, method, and case lists are
  assigned in one index-bounded pass that keeps the exact numeric IDs produced
  today, replacing the prepend-and-copy fold.
- Recovery: skip the whole-tree rebuild when no diagnostics were produced, and
  look up element diagnostics by start offset instead of scanning every
  diagnostic per element.
- Add MyGO benchmarks for lexing, syntax parsing, lowering, and the whole parse
  stage at three sizes, with recorded `ns/op`, `allocs/op`, and `B/op`
  baselines.

Out of scope: components whose members return different result types (those
would need a result union), impl-method components, prelude `Option`
de-boxing, fusing the recovery passes, incremental parsing, and LSP parse
caching (already delivered by `AnalyzeEditorPackageCached`). The lexer's
per-byte string accumulation is also left alone; this change bounds its stack,
not its concatenation cost.

## Capabilities

### New Capabilities

- `parse-stage-performance`: requirements for linear parse-stage cost,
  bounded stack use on flat sequences, unchanged AST identifier values, and a
  benchmark suite that records the parse-stage baseline. Tail-call lowering of
  parse-stage recursion belongs here too, because it is what bounds the lexer's
  stack independent of its parameter lists.

### Modified Capabilities

None. The lossless-CST, diagnostic, and lowering requirements keep their
current behavior; this change adds performance requirements as a separate
capability and must preserve every existing parser2 and ast2 outcome.

## Impact

- `internal/mygo/parser2/syntax_cst_builders.mygo`, `syntax.mygo`
  (`DelimitedBuildResult`), and `syntax_recovery.mygo`
- `internal/mygo/ast2/ast2_ids.mygo`
- `internal/mygo/codegen2/tailcall.mygo`: component eligibility, per-member
  parameter slots, and trampoline emission. The lexer keeps its structure; only
  two missing `using StringRuneIndex` clauses were made explicit so the cycle
  shares one constraint list.
- New MyGO benchmarks under `internal/mygo/parser2`
- Generated `zz_*.gen.go` files must be regenerated through the bootstrap
  pipeline after every `.mygo` edit
- Gated by the existing differential corpus test, lossless tests, formatter
  idempotence, a full `sync` + `go build ./...`, and a byte-identical check of
  every already-emitted homogeneous trampoline
