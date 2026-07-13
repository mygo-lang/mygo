## Known Issues

- **AST `Col` vs `Column` inconsistency**: `MapLitPair` and `SetLitExpr` in `ast.go` still use `Col int` instead of `Column int`. This causes `common.NodePos()` to silently return `(0, 0)` for these types via reflection, losing source position data for map/set literal error messages.
- **`sumList` type ergonomics**: `examples/data-structure/data-structure.mygo` currently accepts `List[Int]`, creates a traversal ref with `Ref.new(lst)`, and walks `tail: Option[Ref[List[Int]]]`. This is runnable and keeps construction explicit, but it still takes the address of a local parameter copy; a future design may prefer accepting `Option[Ref[List[Int]]]` or `Ref[List[Int]]` directly.
