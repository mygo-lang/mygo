# compiler3 canonical value representation

This note freezes the representation rules used by `codegen3`. A representation
is a deterministic function of the MyGO type declaration and the compiler
representation version (`reprVersion`); packages compiled separately with the
same version must agree.

## Matrix

| MyGO type | Generated Go representation | Notes |
| --- | --- | --- |
| `Option[A]` | Tagged struct `{ Tag string; Some struct { F0 A }; None struct {} }` | General case; no interface assertion. The compiler emits canonical helper names and the legacy `Some`/`None` aliases. |
| `Option[Ref[T]]` | `*T` | `nil` denotes `None`; `Some(ref)` preserves the non-nil reference. |
| `Result[A, E]` | Tagged struct `{ Tag string; Ok struct { F0 A }; Err struct { F0 E } }` | General case. |
| `Result[T, error]` at FFI boundary | Go `(T, error)` | Adapted at the call boundary; the MyGO value remains the struct form. |
| `List[A]` | `struct { node *listNode[A] }` with `listNode { head A; tail *listNode[A] }` | `nil` node denotes `Nil`; copying shares the chain. |
| Non-recursive enum | Tagged value struct | `Tag` selects the variant. Variant payloads are stored inline unless a variant directly contains another declared value type. |
| Large non-recursive enum | Tagged struct with per-variant payload pointers | Keeps multi-kilobyte union copies out of the generated code. Constructors allocate payloads; matches keep source-level field types. |
| Recursive enum | Tagged struct with per-variant payload pointers | Recursion must not create an infinitely sized Go type. |
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
- A non-recursive enum normally remains a tagged value struct.
- A non-recursive enum also uses indirect variant payloads when a variant
  directly contains another declared value type. This prevents large unions
  such as `ast2.Decl` from being copied through every match and function call.
- Once an enum uses indirect payloads, constructors take source-level value
  arguments and store pointers internally; match bindings retain source types.
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

`compiler.RepresentationVersion` versions the layout algorithm. It participates
in the codegen3 content-addressed cache key and in mixed-build compatibility
checks. codegen3 emits no metadata artifact; all dependencies in one build are
compiled from source with the same compiler. Cached output from a different
representation version is never mixed into the build.
