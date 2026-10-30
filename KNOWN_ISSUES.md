# Known Issues

> Last updated: 2026-10-30

## Fixed Issues

- **AST `Col` vs `Column` inconsistency** ✅ FIXED: `MapLitPair` and `SetLitExpr` in `ast.go` now use `Column int` instead of `Col int`. `common.NodePos()` correctly extracts source position data via reflection for all AST node types.

## Current Issues

### codegen3 deferred optimization work

- **Fixed payload-size threshold**: Codegen3 indirections recursive enum
  payloads and non-recursive enum variants that directly contain another
  declared value type. A fixed byte threshold for `Option`/`Result` and other
  payload-heavy fields remains deferred until representative allocation and
  copy benchmarks justify the extra indirection cost. Follow-up: measure the
  generated compiler and representative AST/IR workloads before selecting a
  threshold.
- **Unknown-closure and dynamic tail calls**: Statically known self/mutual
  tail-call SCCs are lowered to bounded loops. Calls through unknown closures
  or dynamic interface dispatch remain ordinary Go calls and may still grow
  the stack. Follow-up: evaluate an opt-in trampoline only if profiling shows
  this path matters; it would trade stack safety for allocations.

### codegen3 scope

- **Repository-clean builds are in scope, not deferred**: Codegen3 build and
  test output lives in `MYGO_CACHE` entries and is consumed through a temporary
  overlay. It must not create `zz_*.gen.go`, `zz_*.gen_test.go`, a project-local
  `build/.mygo3`, or a metadata artifact in the repository.
- **codegen2 remains the default ABI**: Codegen2 continues to emit the legacy
  interface-based ADT representation. Canonical tagged representations apply
  only to explicitly selected codegen3 builds; the defaults are not switched
  by the IR3 rollout.

### Typeclass Impl — Multi-Argument Variants

- **Typeclass impl with multiple type parameters** (`prelude/prelude.mygo`): The `impl[A, E] ResultEq[A, E]: Eq[Result[A, E]]` pattern (and similar multi-parameter impls like `impl[K, V] MapIEnumerable[K, V]`) may not compile or generate correct Go code. The compiler's typeclass dispatch (`matchTypeclassHelper`) uses the first type argument's type string for receiver matching, which can cause ambiguity or missed matches when multiple type parameters are involved. Workaround: use single-parameter impls where possible, or implement via inline Go blocks.

### Inherent Impl `using` Constraint Dispatch — zero-arg members

- **`using Default[A]` on an inherent impl method does not resolve** (`prelude`): A constraint-annotated inherent method such as `Result.UnwrapOrDefault` / `Option.UnwrapOrDefault` cannot invoke the constrained member (`Default[A]()` fails with `unknown identifier Default`). The `Default`-constrained extraction methods were dropped from the prelude combinator surface (see `openspec/changes/enhance-prelude-result-api`, tasks 4.1/4.2) and re-scoped as a follow-up once the inherent-impl `using` dispatch path supports zero-arg typeclass members.

### Type Ergonomics

- **`sumList` parameter ergonomics** (`examples/data-structure/data-structure.mygo`): The function accepts `List[Int]` by value, creates a traversal ref with `Ref.new(lst)`, and walks `tail: Option[Ref[List[Int]]]`. While functional, this design takes the address of a local parameter copy. A future design may prefer accepting `Option[Ref[List[Int]]]` or `Ref[ListInt]]` directly to avoid the extra indirection.

## Potential Improvements

### Parser & Lexer

- **Yacc conflicts**: `parser.y` has ~39 shift/reduce + 5 reduce/reduce conflicts (pre-existing, by design). These should be documented and reviewed periodically to ensure they don't cause unexpected parsing behavior.

### Type System

- **Numeric type coverage**: While `Int8`, `Int16`, `Int32`, `UInt8`, `UInt16`, `UInt32`, `Float32` are now supported, ensure all compiler passes (type inference, codegen, Jennifer generation) maintain consistent coverage when new types are added.

### Collection Types

- **Map/Set literal type inference**: The parser uses a heuristic in `{...}` blocks — if every entry uses `:` separator → `MapLitExpr`, otherwise → `SetLitExpr`. This may produce unexpected results for ambiguous cases like `{key: value}` where the type cannot be inferred from context.

### Inline Go

- **`goExprCode` regex patterns**: Three regex patterns (`goSimpleCallRE`, `goSliceFromRE`, `goSliceToLenEqRE`) handle simple Go expression parsing. Complex expressions (nested calls, chained methods) may fall through to manual `go[T]{}` blocks.
