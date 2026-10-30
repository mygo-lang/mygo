## Context

See `proposal.md` for motivation. The facts that shape this design:

- The `parser2` timing label covers one contiguous stage: lexing,
  `buildDelimitedSyntax` (CST construction plus eleven recovery passes),
  lowering to `ast2`, then `ast2` loop validation and identifier assignment.
  All of it runs for every file of every package, including the prelude.
- The hot code is written in MyGO and compiled by the previously built
  compiler, so every edit requires regenerating `zz_*.gen.go` and rebooting the
  pipeline. A change to the trampoline generator cannot be tested with the new
  generator before that generator exists.
- The compiler already lowers mutually recursive tail calls to a trampoline,
  but only when every member of a strongly connected component shares one
  lowered ABI and every intra-component call is in tail position. Otherwise the
  component is left as ordinary recursion. This change keeps the tail-position
  rule and the result-type rule, and drops the parameter-list half of the ABI
  requirement.
- Measured baseline (self-hosted CLI, `--bootstrap-timing sync`, this machine):

| Shape | Size | Parse stage |
|---|---|---|
| 375 declarations | 19 KB | 89 ms |
| 1500 declarations | 76 KB | 353 ms |
| 6000 declarations | 310 KB | 2.22 s |
| 12000 declarations | 626 KB | stack overflow |
| one real source file | 109 KB | 245 ms |

- Sample attribution for the 6000-declaration input: about 14% in `ast2`
  identifier assignment, 6% in loop validation, 21% in allocation, zeroing, and
  copying, 5% in stack growth and unwinding, 7% in the lexer, 5% in the CST
  builder, 4% in lowering. Background GC marking occupies parallel workers and
  about 3% of wall time.

## Goals / Non-Goals

**Goals:**

- Remove the quadratic list rebuild from `ast2` identifier assignment while
  keeping every identifier value exactly as it is today.
- Bound parse-stage stack by syntax nesting depth rather than by token or
  sibling count: make the CST sibling walk a tail call, and extend tail-call
  lowering to recursive components whose members differ in parameter lists so
  the lexer keeps its present shape.
- Remove the unconditional full-tree rebuild in recovery for valid input.
- Establish a benchmark baseline for the parse stage.

**Non-Goals:**

- Components whose members return different result types. A shared loop needs
  one result shape; covering differing results means a result union, and the
  hot parse-stage cycles do not need one.
- Impl-method and typeclass components, and components whose generic
  type-parameter spellings differ.
- De-boxing `Option` in the prelude or specializing `Slice.Get`. Hot paths
  keep the explicit `Option` handling required by
  `openspec/specs/mygo-option-index-handling`.
- Fusing the eleven recovery passes into one, or replacing `List` in
  `joinCstElements` / `syntaxTypeRunsFrom`. Measure after the three changes;
  revisit only if they still dominate.
- Incremental parsing and LSP-side caching (the latter already landed).
- Lexer-internal string accumulation (`raw + next` per byte). This change
  bounds the lexer's stack; shortening its concatenation is a separate edit.

## Decisions

### Decision: the trampoline accepts components with different parameter lists

The lexer's cycle is entirely tail-recursive and every member returns
`LexedSource`, yet it recurses through Go frames. Three checks block it:
`mtSameABIPair` demands identical parameter count and types, `mtMemberSetup`
binds parameter `i` to the shared name `__mygo_mt_p<i>`, and
`isTailSelfCall` compares a call's arity against the *current* function's. The
only real difference inside the component is the parameter shape: `lexStep`
takes one argument, `lexEmitToken` four, `lexQuoted` five, and the middle
slots carry different types (`CstTokenKind`, `CstTriviaKind`, `String`,
`Bool`).

The change keeps one trampoline per component and gives each member its own
typed slots, named `__mygo_mt_s<state>_p<i>`. The trampoline declares the union
of those slots plus `__mygo_state`; each case binds its slots to the member's
parameter names on entry, so the existing body translation is unchanged. A
rewritten tail call stages arguments into temporaries left to right, then
writes the *callee's* slots and jumps to its state.

Eligibility becomes: identical generic type parameters, identical `using`
constraints, identical lowered result types, and every intra-component call in
tail position. Parameter lists no longer participate. Groups that already share
one ABI keep the existing emission path, so the 971 trampolines already in the
tree stay byte-identical.

Constraints stay part of the rule because they decide the dictionary prefix each
member expects; a member that resolves its dictionary from the module instead of
declaring the constraint would receive a different prefix. The lexer needed two
functions to state the `StringRuneIndex` constraint the rest of the cycle already
declared, which is the whole source change this route required.

Cost: the trampoline takes one pointer to a frame struct that escape analysis
keeps on the stack; each iteration is a state switch plus slot assignments. No
allocation, no interface boxing, no type assertion.

Alternatives considered:

- Making scanners return data so `lexStep` becomes a single-member cycle. Not
  chosen: it rewrites a working lexer to satisfy a generator restriction, and
  the restriction is the part worth changing.
- Carrying the union through `any`: rejected, because it boxes every argument
  on every step.
- Explicit `while` loops in the lexer: rejected earlier for the same reason.

### Decision: the CST builder carries a sibling accumulator

`buildDelimitedChildren` currently calls itself for the rest of the siblings and
then wraps the result with `List.Cons`, so the call is not in tail position and
each element adds a frame. It also builds `Option[List[CstElement]]` and
converts it back to a slice.

The new shape appends to an accumulator before recursing:

```
buildDelimitedChildren(items, index, close, acc)
  ... on an ordinary element:
    let next = acc.Append(item)
    buildDelimitedChildren(items, index + 1, close, next)    # tail
  ... on a nested opener:
    let nested = buildDelimitedChildren(items, index + 1, terminator, [])  # bounded by nesting
    buildDelimitedChildren(items, nested.Next, close, acc.Append(node))    # tail
```

Appending keeps source order, so no reversal is needed. `DelimitedBuildResult`
carries `Slice[CstElement]` instead of `Option[List[CstElement]]`, which removes
the `Cons`/`cstListToSlice` round trip on this path. Calls into the nested
region go through `buildNestedRegion`, a separate one-line function that enters
the accumulator form with an empty list. The split matters: a single non-tail
self-call inside the walker would disqualify the whole component, since the
analysis refuses to optimize only part of a cycle. Nesting depth, not element
count, is what still consumes stack.

Alternative considered: keep `List` and add the accumulator on top. Rejected
because the accumulator has to be materialized anyway, and the conversion cost
vanishes only if the accumulator is the result type.

### Decision: identifier assignment walks lists by index in one pass

`assignDeclIDs`, `assignStmtIDs`, and `assignImplMethodIDs` recurse to the end
and then prepend each element into a freshly allocated array, so an n-element
list performs n array allocations and n²/2 element copies.

Each list is replaced by one index-bounded pass that carries the running
`NextID` and writes results in place. The current numbering order is preserved
exactly, including the existing asymmetry: declaration and method lists are
numbered from the last element backwards, while statement lists are numbered
forwards. That asymmetry is observable in generated identifiers, so it is
treated as a contract, not a bug to fix here.

Index access keeps the explicit `Option` handling: the loop condition is
`index < items.Len()`, and a `None` from `Get` stays an explicit error path
rather than a fabricated element.

Alternatives considered: two passes that first count nodes per declaration and
then fill identifiers left to right. Rejected: more code, and it still has to
reproduce the reverse order.

### Decision: recovery short-circuits on valid input

`buildDelimitedSyntax` rebuilds the entire CST through `recoverCstElements`
even when no diagnostic was produced, and `cstElementHasDiagnostic` scans all
diagnostics for every element. The rebuild is an identity transform when the
diagnostic list is empty, so it is skipped in that case; when diagnostics exist,
they are indexed by start offset once and looked up per element.

### Decision: benchmarks live in the parser package as MyGO tests

Lexing, syntax parsing, lowering, and the whole parse stage are each benchmarked
over two shapes: a pinned real source fixture repeated 1×/4×/16×, and generated
declaration-heavy input at 500/2000/8000 declarations. They report `ns/op`,
`B/op`, and `allocs/op` so each step can be compared against the recorded
baseline. They are written in MyGO like the rest of the package's tests
(`func BenchmarkX(b: Ref[testing.B])`), with one function per shape and size
because sub-benchmarks need a Go function value.

## Risks / Trade-offs

- **The relaxed eligibility may still refuse the lexer**, for example if one
  member's call is not in tail position or its result type differs → assert the
  outcome instead of assuming it: after regeneration, check that the generated
  lexer file contains the trampoline symbol and no `lexStep` call inside a
  case body, and keep a test that fails if the symbol disappears.
- **Widening eligibility can silently rewrite existing generated code**: 971
  trampolines already exist, and every one of them must stay byte-identical →
  keep the old emission path for groups that share one ABI, regenerate the whole
  repository, and require an empty `git diff` on `zz_*.gen.go` apart from the
  intended files.
- **Wide trampolines**: a component with many differently typed parameters
  produces one slot per member parameter, so a large component yields a long
  signature and frame → keep slots typed and flat, and record the generated
  shape for the lexer (about thirty slots) as the expected upper end.
- **Bootstrap ordering**: the new `.mygo` sources are compiled by the old
  compiler, so a broken intermediate state can look like a compiler bug →
  regenerate, build, then re-run `sync` with the new binary and require the
  second generation to be byte-identical (fixed point) before running tests.
- **Identifier drift**: any change in numbering order rewrites generated Go
  across the repository → gate on the differential corpus plus a generated-Go
  comparison for a sample of packages, and diff `zz_*.gen.go` after each step.
- **Deep nesting still consumes stack**: the fix bounds depth by nesting, not
  by depth. A deliberately deep file still recurses; that is accepted and
  documented rather than solved here.
- **Measured target may not be reached by these three items alone** → measure
  after each step and record what remains, instead of expanding scope silently.

## Migration Plan

Four behavior-preserving steps, each independently shippable and revertible:
identifier assignment, then the CST builder, then heterogeneous trampoline
support, then the recovery fast paths. Benchmarks are added first so every step
has a before/after number. The trampoline step touches generated code for the
whole repository, so it lands with a full regeneration and a byte-identical
comparison of every pre-existing trampoline. No on-disk format changes, no
cache invalidation, no flag. Rollback is a revert of the step; generated files
are rebuilt from source in the same commit.

## Open Questions

- Which repository file to pin as the real-world benchmark fixture. Any source
  with representative comment, string, and nesting density works; the choice
  does not affect the approach.
