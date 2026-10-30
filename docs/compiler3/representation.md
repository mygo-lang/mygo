# compiler3 canonical value representation

This note freezes the representation rules used by `codegen3`. A representation
is a deterministic function of the MyGO type declaration and the compiler
representation version (`reprVersion`); packages compiled separately with the
same version must agree.

## Matrix

| MyGO type | Generated Go representation | Notes |
| --- | --- | --- |
| `Option[A]` | `struct { Value A; Ok bool }` | General case; no interface assertion and no per-value allocation. |
| `Option[Ref[T]]` | `*T` | `nil` denotes `None`; `Some(ref)` preserves the non-nil reference. |
| `Result[A, E]` | `struct { Value A; Err E; Ok bool }` | General case. |
| `Result[T, error]` at FFI boundary | Go `(T, error)` | Adapted at the call boundary; the MyGO value remains the struct form. |
| `List[A]` | `struct { node *listNode[A] }` with `listNode { head A; tail *listNode[A] }` | `nil` node denotes `Nil`; copying shares the chain. |
| Non-recursive enum | Tagged value struct with all variant fields | `Tag` selects the variant; fields are read directly. |
| Recursive enum | Tagged pointer node, or arena index for compiler-owned IR | Recursion must not create an infinitely sized Go type. |
| `Ref[T]` | `*T` | Unchanged. |
| `Slice[T]` | `[]T` | Native Go slice. |
| `Map[K, V]` | `map[K]V` | Native Go map. |
| `Set[A]` | `map[A]struct{}` | Native Go set encoding. |
| `T[N]` | `[N]T` | Native Go array. |
| First-order generic `T` | Go type parameter `T` | Do not erase to `any` when Go can express it. |
| HKT application | Existing erased `HKT`/`any` encoding | Go cannot express higher-kinded application directly. |
| Dynamic typeclass value | Existing `MygoDynamic*` wrapper | `any` payload remains; adapters are regenerated per `adapterABI`. |

## Recursion rule

Build a directed type-reference graph over named types. Ordinary field access
and tuple/slice element references create edges; `Ref[T]`, `Slice[T]`, and
`Map[K, V]` already introduce indirection.

- A type that participates in a direct or mutual cycle is emitted through a
  pointer node (or an arena index for compiler-owned IR).
- A non-recursive enum remains a value struct.
- A variant field that refers back to its own enum type becomes `*T`.
- Fields reachable only through `Ref`/`Slice`/`Map` do not require an extra
  pointer.

`Represent3` must reject any layout that would create an infinitely sized Go
type.

## Boundaries

- **Go FFI**: `Option[Ref[T]]` → `*T`, `Result[T, error]` → `(T, error)`,
  `Ref[T]` → `*T`, collections → native Go shapes.
- **Dynamic typeclass**: wrappers keep the `any` payload; adapters convert the
  canonical concrete type. `adapterABI` changes when adapter signatures change.
- **Inline Go**: snippets receive the canonical spelling. A snippet that
  requires the previous interface/type-assertion shape must be rejected with a
  diagnostic rather than silently miscompiled.

## Versioning

Generated metadata records `reprVersion` and optional per-type `layoutHash`.
Compilation cache keys include both `compilerABI` and `reprVersion`. A package
that imports metadata with a different `reprVersion` must be recompiled or
rejected; layouts from different versions are never mixed.
