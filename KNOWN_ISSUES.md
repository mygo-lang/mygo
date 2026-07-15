# Known Issues

> Last updated: 2026-07-14

## Fixed Issues

- **AST `Col` vs `Column` inconsistency** ✅ FIXED: `MapLitPair` and `SetLitExpr` in `ast.go` now use `Column int` instead of `Col int`. `common.NodePos()` correctly extracts source position data via reflection for all AST node types.

## Current Issues

### Typeclass Impl — Multi-Argument Variants

- **Typeclass impl with multiple type parameters** (`prelude/prelude.mygo`): The `impl[A, E] ResultEq[A, E]: Eq[Result[A, E]]` pattern (and similar multi-parameter impls like `impl[K, V] MapIEnumerable[K, V]`) may not compile or generate correct Go code. The compiler's typeclass dispatch (`matchTypeclassHelper`) uses the first type argument's type string for receiver matching, which can cause ambiguity or missed matches when multiple type parameters are involved. Workaround: use single-parameter impls where possible, or implement via inline Go blocks.

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
