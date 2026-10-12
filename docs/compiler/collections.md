# collections.md — Collections and Literals

## Collection Types

- `List[A]`: singly-linked list using `Option[Ref[List[A]]]` for the tail field — `None` is the terminator, `Some(ref)` points to the next node. This avoids a separate `Nil` helper and keeps the nil-termination semantics explicit through `Option`.
- `Slice[A]`: MyGO syntax `Slice[Int]` → Go `[]int`. Lowered directly to Go's native slice type via `goType` / `typeString`.
- `Map[K, V]`: lowered directly to Go's native `map[K]V` via `goType` / `typeString`.
- `Set[A]`: lowered directly to Go's native `map[A]struct{}` via `goType` / `typeString`.

### Design Rationale

- **Why `Option[Ref[List[T]]]` instead of just `Ref[List[T]]`?**
  `Option` provides an explicit `None` terminator for list ends, avoiding nil-pointer dereferences. `Ref` ensures the tail is a non-nil pointer (Go `*List[T]`), so when the tail is `Some`, we always have a valid pointer to dereference. This separates "no next node" (`None`) from "points to next node" (`Some(ref)`), which is both safer and more idiomatic in the type system.

- **Why `Slice[A]` / `Map[K,V]` instead of MyGO struct declarations?**
  These types wrap Go's native slices, maps, and sets directly. Declaring them as MyGO structs would add unnecessary runtime overhead (extra fields, allocation, indirection) and wouldn't provide additional type safety beyond the Go type system. Instead, the compiler recognizes the type names `Slice`, `Map`, `Set` and lowers them directly to Go builtins.

- **Why not `[]A` prefix syntax?**
  The parser uses ordinary generic syntax for slices, so `Slice[Int]` is the single canonical form. This keeps slice types visually consistent with `Map[K, V]`, `Set[A]`, and other generic nominal types.

- **Why not `Slice` / `Map` / `Set` as prelude struct types?**
  Previously, these were declared as structs in `prelude.mysrc` with placeholder fields (`entries: String`, `size: Int`). They were removed because: (1) they served no runtime purpose — the prelude struct declarations had no usable fields; (2) `genStruct` would emit them as Go structs, conflicting with the `goType` lowering to native Go types; (3) keeping them only in the compiler's type lowering is cleaner and zero-cost. `List[A]` remains as a prelude struct because it needs actual runtime data structure semantics.

## Fixed-Length Arrays (Bootstrap)

`T[N]` maps directly to Go `[N]T`. The non-negative integer literal N is part of the type: `Int[2]` and `Int[3]` are distinct. Lengths and repeat results must fit the target Go `int`; negative lengths, runtime lengths, constant expressions, and generic length parameters are rejected.

```mygo
let values = [1, 2, 3] as Int32[3]
let zeros = [0] as Int[1] * 5
let sequence = [1, 2] as Int[2] * 3
let empty = [] as Int[0]
let rows: Int[2][2] = [[1, 2], [3, 4]]
```

An array literal must contain exactly N elements. `[1] as Int[5]` and `[] as Int[5]` fail; no elements are silently zero-filled. An array annotation, parameter, return type, or struct field can supply the expected type. Without array context, `[1, 2]` remains a Slice. Existing Slice values do not implicitly convert into arrays.

`array * K` repeats the entire input sequence K times: the sequence example above yields `[1, 2, 1, 2, 1, 2]` of type `Int[6]`. K must be a non-negative integer literal. The array expression is evaluated once, including when K is zero. Array elements use Go value copying; reference elements retain their shared references. Scalar-times-array and Slice repetition are unsupported.

Suffixes wrap from left to right: `Int32[5][3]` is Go `[3][5]int32`, and `Slice[Int32[5]]` is `[][5]int32`. Generic element types such as `T[5]`, aliases, and Go array parameters, returns, and fields preserve their native array type. Arrays use Go value semantics for assignment and parameter passing. This feature adds no new indexing or collection methods; Go interoperation can operate on array values.

When a function body begins with a list literal, place it on the next line after the return type, so it is not read as another type suffix.

## Slice, Map, and Set Literals

- Slice: `[1, 2, 3]: Slice[Int]` → Go `[]int{1, 2, 3}`
- Map: `{"a": "1", "b": "2"}: Map[String, String]` → Go `map[string]string{"a": "1", "b": "2"}`
- Set: `{"x", "y"}: Set[String]` → Go `map[string]struct{}{"x": {}, "y": {}}`
- No list literal syntax (as designed).
- Type inference strategy: infer from element expressions first; fall back to type annotation if inference fails; error if neither can determine the type. Mismatched element types produce an error.
- Empty `{}` is treated as an empty map by default; if the expected type is `map[A]struct{}`, it becomes an empty set.
- Parser uses a heuristic in `{...}`: if every entry uses `:` separator → `MapLitExpr`, otherwise → `SetLitExpr`.

### AST Nodes (`internal/mygo/ast/ast.go`)

- `SliceLitExpr` — `{Line, Column, Elem TypeExpr, Elems []Expr}`
- `MapLitExpr` — `{Line, Column, Key TypeExpr, Val TypeExpr, Pairs []MapLitPair}`
- `MapLitPair` — `{Line, Col, Key Expr, Value Expr}`
- `SetLitExpr` — `{Line, Col, Elem TypeExpr, Elems []Expr}`

### Key Files

- **Parser**: `internal/mygo/parser/parser.y` — collection literal grammar lowers to `SliceLitExpr`, `MapLitExpr`, and `SetLitExpr`.
- **Compiler**: `internal/mygo/compiler/translate_expr.go` — `translateSliceLit()`, `translateMapLit()`, `translateSetLit()`, `translateEmptyMapLit()`.
- **Type parsing update** (`internal/mygo/parser/parser.y`): `Slice[Int]` is the canonical slice type spelling and `Int[]` shorthand is no longer recognized.
