# Mutual Tail-Call Optimization

> Status: implemented for eligible top-level functions with an identical
> lowered calling convention, including self and mutual tail recursion.

## Goal

Make a mutually recursive tail-call cycle run in constant Go stack space while
preserving every source-level function symbol.  In particular, an exported
MyGO function such as `Even` must still be emitted as Go's `Even`; callers in
other Go/MyGO packages must not observe an API rename.

The transform is an optional code-generation optimization.  If a group is not
eligible, the compiler emits the current direct Go calls without reporting a
new user error.

```mygo
func Even(n: Int) -> Bool
  if n == 0 => true else Odd(n - 1)
end

func Odd(n: Int) -> Bool
  if n == 0 => false else Even(n - 1)
end
```

is conceptually emitted as:

```go
func Even(n int) bool { return __mygo_mt_pkg_Even_Odd(n, 0) }
func Odd(n int) bool  { return __mygo_mt_pkg_Even_Odd(n, 1) }

func __mygo_mt_pkg_Even_Odd(n int, __mygo_state uint8) bool {
	for {
		switch __mygo_state {
		case 0: // Even
			if n == 0 { return true }
			__mygo_next_n := n - 1 // evaluate before overwriting n
			n = __mygo_next_n
			__mygo_state = 1
			continue
		case 1: // Odd
			if n == 0 { return false }
			__mygo_next_n := n - 1
			n = __mygo_next_n
			__mygo_state = 0
			continue
		default:
			panic("mygo: invalid mutual-tailcall state")
		}
	}
}
```

`__mygo_` is reserved for compiler temporaries.  The exact helper suffix is
an implementation detail, and no generated helper is part of the public API.

## Analysis phase

Run this after type inference and after normal call resolution, before
per-file Go declarations are rendered.

1. Build a directed graph whose vertices are package-level `FuncDecl`s.  Add
   an edge only for a direct identifier call resolved to a package-level
   function.  Calls through variables, function fields, Go embedding, method
   dispatch, and typeclass dictionary values are not graph edges in v1.
2. Mark an edge as *tail* only when its call supplies the result of the current
   function: an explicit `return F(...)`, the final expression of a function
   block, or a final expression recursively contained in an `if`, `switch`, or
   block in such a position.  A call under an operator, tuple/struct
   construction, argument list, `let`, assignment, or non-final statement is
   not tail.
3. Find strongly connected components (Tarjan or Kosaraju) in the tail-edge
   graph. Optimize an SCC with at least two members, or a one-member SCC that
   has a tail edge to itself.
4. Check the group's ABI eligibility described below.  On failure, leave the
   complete SCC untouched.  Do not optimize only some of its edges: that makes
   performance unpredictable and can still grow the stack through the omitted
   member.

Call resolution must use the resolved declaration identity, not merely the
source spelling, so a shadowing local function value cannot accidentally join
the SCC.

## v1 ABI eligibility

A single Go loop has one fixed parameter and result layout.  Therefore every
member of an optimized SCC must have the same normalized lowered ABI:

- the same number, order, and Go-lowered types of ordinary parameters;
- the same Go-lowered result list, including unit versus multi-return tuples;
- alpha-equivalent type parameters and constraints; and
- the same normalized `using` constraints in the same dictionary-parameter
  order.

Parameter *names* may differ.  The current implementation requires generic
type-parameter spellings to match (for example, both functions use `[A]`), so
it can reuse the existing generated Go AST without type-variable rewriting.
Alpha-normalization of `[A]` and `[B]` is a compatible follow-up; their
constraints may not differ.  All non-generic ABI comparison is performed on
compiler-owned lowered types, not rendered Go strings.

This restriction is intentional.  Supporting heterogeneous cycles requires a
tagged continuation frame carrying a union of all argument and result shapes;
that entails allocations or `any`/type assertions, worsens generic typing, and
is a separate design.  Functions with different arities or result types retain
normal recursion in v1.

The initial scope is ordinary package-level functions.  Impl methods already
have mangled symbols and typeclass dispatch; they can join the mechanism in a
later phase only after their resolved receiver/dictionary ABI is modeled in
the call graph.

## Lowering contract

For each eligible SCC, allocate a stable state number by sorting members by
`SourceFile`, source line/column, then name.  Generate exactly one private
trampoline in a deterministic owner file: the lexicographically first source
file containing a member (the prelude is its own owner).  Its name is
`__mygo_mt_` plus a sanitized, deterministic package/SCC identity.  Add a
short hash if necessary to prevent collisions with user identifiers.

Every original declaration remains a wrapper with its original name and
signature.  The wrapper passes its parameters and any generated `using`
dictionary parameters to the trampoline with the member's state number.  This
preserves exports, Go imports, function values such as `f := Even`, and calls
from non-optimized code.

The trampoline has the common ABI plus an integer state parameter.  Its body
is an infinite `for` containing a `switch state`; each case lowers the source
body of one member.  Nonrecursive terminal paths emit ordinary `return`s.
Only a tail call whose resolved callee is in the same SCC is rewritten:

1. Translate every argument in normal source order into fresh
   `__mygo_next_<state>_<slot>` temporaries.  This preserves argument side
   effects and the old values of all parameters.
2. Assign the common parameter variables from those temporaries.
3. Set the target state and `continue` the loop.

All other calls, including tail calls to functions outside the SCC, remain
ordinary calls/returns.  Because no transformed case calls an SCC member, the
cycle consumes no additional Go stack frames.

The generated default state panic is defensive only; no source program can
select it.  It avoids a silently looping corrupted helper and makes compiler
bugs diagnosable.

## Returned closures: continuation state machines

Parser combinators commonly return a closure that recreates its enclosing
function before invoking it, for example `F(captured...)(nextState)`. Such a
call can have source work after it, so it is not a strict tail call and cannot
join the top-level SCC trampoline. The code generator recognizes a conservative
closure-local form: a returned, single-argument closure invokes its enclosing
function with the unchanged captured arguments and one next-state argument.

It lowers that invocation to a loop with an explicit stack of typed
continuations. The continuation contains the remaining generated statements;
when a non-recursive path produces the closure result, the loop unwinds those
continuations in LIFO order. This is independent of library symbols and data
operations: it does not recognize `PMany`, `Reply`, `Prepend`, or any prelude
type by name. It preserves the normal captured variables and generated
typeclass dictionary parameters by using ordinary Go closures for frames.

The initial implementation accepts one recursive call site and one explicit
final return in the closure. Ambiguous shapes, altered captures, multi-state
closures, and multiple result values retain ordinary lowering.

## Integration points

- Add an immutable `mutualTailPlan` to `internal/mygo/codegen`, keyed by
  `*FuncDecl`, containing group identity, state number, owner file, and the
  canonical ABI.
- Build the plan in `GenerateFiles` immediately after `newGen` and before
  source-file rendering.  Do not mutate `Package`, `FuncDecl`, or typed AST
  nodes; generation is used by tests and may be repeated.
- Teach `genFuncDecl` to choose between the existing function body and a thin
  wrapper when its declaration has a plan entry.
- Add `genMutualTailTrampoline(plan)` while rendering the owner source file.
  Rendering must occur once even if declaration ordering changes.
- Thread an optional active trampoline context through the existing expression
  and block translators.  At a recognized tail position it intercepts only a
  direct call to a planned member and emits the temporary/assign/state/
  `continue` sequence.  Existing expression translation remains authoritative
  for argument typing, generic type arguments, and `using` dictionary calls.

Per-file output means the helper must be emitted in one chosen file rather
than beside every wrapper.  Go package scope permits wrappers in other
generated files to call it.

## Correctness and observability

The transform is valid only for tail calls because there is no caller work to
resume.  It preserves left-to-right evaluation by staging all arguments before
any parameter assignment.  It must also preserve named parameter shadowing by
using generated identifiers unavailable to MyGO source.

The observable Go function names and signatures are unchanged.  Stack traces
inside an optimized cycle will show the private trampoline rather than one Go
frame per source call; document this as the intended debugging trade-off.
Panics, deferred Go code, and inline Go expression side effects remain in the
same dynamic order, provided the compiler only recognizes direct tail calls
and stages their arguments as above.  v1 should conservatively reject a
candidate function whose tail path contains an inline-Go construct that emits
`defer` or otherwise requires a distinct Go activation record; alternatively,
formalize such constructs as non-tail for this pass.

## Tests

Add compiler/codegen tests covering:

- `Even`/`Odd`: original exported declarations exist, one `__mygo_mt_` helper
  exists, and generated Go passes `go test`/`go vet` compilation;
- a million-step mutual cycle executes without stack growth (run a generated
  test binary with a deliberately small maximum stack, or inspect that no SCC
  member calls another SCC member in the helper);
- tail calls nested in `if`, `switch`, block-final expressions, and explicit
  `return`;
- argument ordering: `F(x, x)` to a member with parameter names reordered,
  plus side-effecting inline-Go arguments, verifies staging;
- generic functions and `using` constraints with identical canonical ABI;
- exported wrappers callable from a second Go package and assignable as
  function values;
- negative cases: different arity/results/constraints, calls under `+`, local
  function values shadowing a package function, and SCCs containing an
  unsupported inline-Go activation behavior.  Each must retain the old direct
  lowering and compile successfully.

## Future extension

Heterogeneous mutual recursion should use a typed, compiler-generated
frame/continuation representation only after profiling demonstrates that its
allocation and type-assertion cost is acceptable. It must still retain the
source wrappers, so exported function names remain stable.
