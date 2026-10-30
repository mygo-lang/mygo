# collections.md — Collections and Literals

## Collection Types

- `List[A]`: a source-level `Nil | Node(A, Option[Ref[List[A]]])` singly-linked list. The tail is absent at `Nil`; `Node` stores an optional pointer to the next node so list values share the same chain when copied. `Cons` and the other list methods keep these semantics across both backends.
- `Slice[A]`: MyGO syntax `Slice[Int]` → Go `[]int`. Lowered directly to Go's native slice type via `goType` / `typeString`.
- `Map[K, V]`: lowered directly to Go's native `map[K]V` via `goType` / `typeString`.
- `Set[A]`: lowered directly to Go's native `map[A]struct{}` via `goType` / `typeString`.

### Design Rationale

- **Why `Option[Ref[List[T]]]` in the node tail?**
  `Option` distinguishes an absent next node from a present one. `Ref` makes the present edge indirect and guarantees pointer identity for the shared chain. Empty lists are represented by `List.Nil`, not by a null `Ref`; `Tail()` converts the source-level result back to `Option[List[T]]`.

- **Why `Slice[A]` / `Map[K,V]` instead of MyGO struct declarations?**
  These types wrap Go's native slices, maps, and sets directly. Declaring them as MyGO structs would add unnecessary runtime overhead (extra fields, allocation, indirection) and wouldn't provide additional type safety beyond the Go type system. Instead, the compiler recognizes the type names `Slice`, `Map`, `Set` and lowers them directly to Go builtins.

- **Why not `[]A` prefix syntax?**
  The parser uses ordinary generic syntax for slices, so `Slice[Int]` is the single canonical form. This keeps slice types visually consistent with `Map[K, V]`, `Set[A]`, and other generic nominal types.

- **Why not `Slice` / `Map` / `Set` as prelude struct types?**
  Previously, these were declared as structs in `prelude.mysrc` with placeholder fields (`entries: String`, `size: Int`). They were removed because: (1) they served no runtime purpose — the prelude struct declarations had no usable fields; (2) `genStruct` would emit them as Go structs, conflicting with the `goType` lowering to native Go types; (3) keeping them only in the compiler's type lowering is cleaner and zero-cost. `List[A]` remains as a prelude struct because it needs actual runtime data structure semantics.

## Backend ABI

Collection semantics are language-level and unchanged by backend selection. Generated layouts differ:

| Source value | Codegen2 (default) | Codegen3 (opt-in) |
| --- | --- | --- |
| `List[A]` | Legacy interface enum with `List__Nil` / `List__Node` variant structs. | Canonical tagged enum; `Node` payload is indirect when required by the canonical representation rules. |
| `Option[A]` | Legacy interface with `Option__Some` / `Option__None` variant structs. | Tagged value struct; `Option[Ref[T]]` uses `*T`/`nil`. |
| `Result[A, E]` | Legacy interface with `Result__Ok` / `Result__Err` variant structs. | Tagged value struct; FFI `(T, error)` is adapted at the boundary. |
| `Slice[A]` / `Map[K,V]` / `Set[A]` | Native Go slice/map/set shapes. | Native Go slice/map/set shapes. |

Codegen2 remains the default and retains its legacy interface ABI. Codegen3 activates only through explicit selection; generated outputs from the two backends are never mixed in one package.

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

An array value can be ascribed to a slice with `value as Slice[T]`, which lowers to the Go slice expression `value[:]`:

```mygo
let values: Int[4] = [1, 2, 3, 4]
let slice: Slice[Int] = values as Slice[Int]
let rows: Slice[Int[3]] = matrix as Slice[Int[3]]   # Int[3][2] -> [][3]int
```

The element types must agree, including nested element types. The slice shares the operand's backing array, so writing through the slice is visible through the array. When the operand is not addressable (for example a call result or an array literal), the compiler copies it into a local first, since Go cannot slice such a value directly. Slice-to-array ascription remains unsupported.

When a function body begins with a list literal, place it on the next line after the return type, so it is not read as another type suffix.

## Slice, Map, and Set Literals

- Slice: `[1, 2, 3]: Slice[Int]` → Go `[]int{1, 2, 3}`
- Map: `{"a": "1", "b": "2"}: Map[String, String]` → Go `map[string]string{"a": "1", "b": "2"}`
- Set: `{"x", "y"}: Set[String]` → Go `map[string]struct{}{"x": {}, "y": {}}`
- No list literal syntax (as designed).
- Type inference strategy: infer from element expressions first; fall back to type annotation if inference fails; error if neither can determine the type. Mismatched element types produce an error.
- Empty `{}` is treated as an empty map by default; if the expected type is `map[A]struct{}`, it becomes an empty set.
- Parser uses a heuristic in `{...}`: if every entry uses `:` separator → `MapLitExpr`, otherwise → `SetLitExpr`.

### AST Nodes

The active bootstrap AST is `internal/mygo/ast2`: `SliceLitExpr(Slice[Expr])`, `MapLitExpr(Slice[(Expr, Expr)])`, `SetLitExpr(Slice[Expr])`, and fixed-length array types represented by `MonoType.TArray`. Literal AST nodes do not carry duplicated element/key type fields; inference attaches the resolved `MonoType` to the expression.

### Key Files

- **Parser**: `internal/mygo/parser2/syntax_parse_expressions.mygo` builds collection CST nodes; `syntax_lower_expressions.mygo` lowers them to `ast2` literals.
- **Inference**: `internal/mygo/typeinference2/infer.mygo` handles element unification, empty literal context, and fixed-length arrays.
- **Codegen2**: `internal/mygo/codegen2/translate_ast.mygo` emits legacy collection/enum shapes.
- **Codegen3**: `internal/mygo/ir3/lower.mygo` lowers literals to IR; `internal/mygo/codegen3/literals.mygo` emits native slice/map/set/array literals and canonical enum constructors.
