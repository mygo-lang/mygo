# compiler3 optimizer passes

`optimizer3` rewrites an `ir3.Package` after representation decisions and before
verification/code generation. Pass order is explicit and deterministic.

## Levels and driver

| Level | Pass order |
| --- | --- |
| `O0` | No optimizer passes. |
| `O1` | `Match3`, `Simplify3`, `Tailcall3`. |
| `O2` | `O1`, then `Unbox3`, `Closure3`, `Inline3`, `Specialize3`. |

`Run` applies this ordered list once. `RunToFixpoint` is available for callers
that need repeated convergence and stops on a stable IR dump or an explicit
iteration limit. The normal codegen3 pipeline uses one ordered pass list so the
cost of whole-package dumps does not dominate compilation.

Every pass preserves declaration order and uses deterministic map/worklist
traversal. A changed IR dump is the pass driver's convergence signal.

## Match3

`Match3` removes unreachable and duplicate unguarded cases after an earlier arm
has already established the same tag/pattern. Guarded cases remain ordered and
are never dropped: a failed guard transfers to the next applicable arm through
the existing `RaiseExpr` continuation. The pass also merges identical terminal
arms where doing so cannot change which guard or binding executes.

The subject remains evaluated once. Match lowering does not introduce repeated
interface assertions.

## Simplify3

`Simplify3` performs local, semantics-preserving rewrites:

- constant-fold pure primitives and string/numeric operations;
- propagate immutable local copies;
- remove dead pure lets and unused pure allocations;
- reuse identical pure expressions inside a scope;
- hoist only expressions whose effects and dependencies permit it.

`Foreign` operations are ordering barriers. Unused FFI/inline-Go calls are not
deleted, and effectful calls are not reordered across reads, writes, panics, or
other foreign operations.

## Tailcall3

`Tailcall3` records statically known self/mutual tail calls, groups targets into
strongly connected components, and marks SCCs for codegen3 loop/state-machine
lowering. Exported functions retain their source-level names and signatures.

Unknown closures, dynamic interface calls, and heterogeneous SCCs remain
ordinary calls. A universal trampoline is intentionally out of scope because it
would add per-call allocation to cases the optimizer cannot prove.

## Unbox3

`Unbox3` converts local `Option`/`Result` producers and consumers to explicit
flag/payload multi-return nodes when both sides are visible in the same
function. It also removes a `Ref.new(x)` followed by local dereferences when the
reference does not escape.

The pass checks mutation, capture, type application, casts, and call escape
paths. Any uncertain use leaves the allocation/value struct untouched.

## Closure3

`Closure3` computes function free variables, lifts closed lambdas to package
declarations, and beta-reduces immediate function-literal calls when parameter
binding is safe. Capturing lambdas remain closures; the pass does not copy or
duplicate effectful arguments during beta reduction.

## Inline3

`Inline3` performs intra-package leaf inlining under a code-size budget. It
skips recursive functions, shadowed callees, large bodies, and callees whose
parameters are reassigned in a way that would duplicate evaluation. Cross-
package inlining is deliberately unsupported.

## Specialize3

`Specialize3` clones known generic dictionary call sites, substitutes concrete
instance helpers, and removes dictionary parameters that become unreachable. It
skips captured, reassigned, or unknown dictionary evidence and preserves the
original generic function as the rollback/export surface.

Specialization is intra-package. Repeated plans are deduplicated by a stable
name/order map, so map iteration cannot change generated declarations.

## Measured scope and deferred work

The checked-in performance report records end-to-end O0/O1/O2 compile-time,
allocation, generated-compiler compile time, and the 200,000-call stack-depth
test. Pass-specific deltas that are below one-run noise or lack a representative
workload are documented there as deferred.

Large value payloads use a declaration-shape rule rather than a fixed byte
threshold: non-recursive enums with direct declared value payloads use indirect
variant payloads. Further size-based thresholds and unknown-closure tail-call
trampolines remain open work.
