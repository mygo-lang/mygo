# compiler3 canonical value representation

This note freezes the representation rules used by `codegen3`. A representation
is a deterministic function of the MyGO type declaration and the compiler
representation version (`reprVersion`); packages compiled separately with the
same version must agree.

The wide-size estimator includes Go field alignment for emitted variant payload
structs. Generic type parameters use a conservative estimate because their
instantiated size is not bounded by the generic declaration; this may select
compact storage for a small instantiation, while preserving its concrete Go
payload type.

## Matrix

| MyGO type | Generated Go representation | Notes |
| --- | --- | --- |
| `Option[A]` | Tagged struct `{ discriminator uint8; Some struct { F0 A }; None struct {} }` | The discriminator is the one-based source declaration ordinal; zero is invalid. Its generated field name avoids variant field collisions. The compiler emits canonical helper names and the legacy `Some`/`None` aliases. |
| `Option[Ref[T]]` | `*T` | `nil` denotes `None`; `Some(ref)` preserves the non-nil reference. |
| `Result[A, E]` | Tagged struct `{ discriminator uint8; Ok struct { F0 A }; Err struct { F0 E } }` | Same source-derived ordinal scheme as every enum. |
| `Result[T, error]` at FFI boundary | Go `(T, error)` | Adapted at the call boundary; the MyGO value remains the struct form. |
| `List[A]` | `struct { node *listNode[A] }` with `listNode { head A; tail *listNode[A] }` | `nil` node denotes `Nil`; copying shares the chain. |
| Non-recursive enum | Tagged value struct | An unsigned integer field selects the variant by one-based declaration ordinal. Its width is `uint8` through 255 variants, `uint16` through 65,535, then `uint32`. The discriminator field name avoids variant field names. Variant payloads are stored inline unless a variant directly contains another declared value type. |
| Large enum, including recursive enums | Compact tagged struct with one `any` payload slot | Selects compact storage by estimated wide type size. The payload interface breaks recursive Go type cycles; selected concrete payloads retain source-level field types. |
| Small recursive enum | Tagged struct with per-variant payload pointers | Keeps direct and mutual recursive Go types finite without a wider generic payload slot when pointer-node storage is already small. |
| `Ref[T]` | `*T` | Unchanged. |
| `Slice[T]` | `[]T` | Native Go slice. |
| `Map[K, V]` | `map[K]V` | Native Go map. |
| `Set[A]` | `map[A]struct{}` | Native Go set encoding. |
| `T[N]` | `[N]T` | Native Go array. |
| First-order generic `T` | Go type parameter `T` | Do not erase to `any` when Go can express it. |
| HKT application | Existing erased `HKT`/`any` encoding | Go cannot express higher-kinded application directly. |
| Dynamic typeclass value | Existing `MygoDynamic*` wrapper | `any` payload remains; adapters are regenerated per `adapterABI`. |

For non-compact tagged enums, the discriminator is emitted after all variant
payload fields. This keeps a zero-sized variant from occupying the final Go
field, where Go may add padding to distinguish its address. Constructors and
matches use field names, so physical field order does not change source-level
variant order or ordinal values.

## Recursion rule

Build a directed type-reference graph over named types. Ordinary field access
and tuple/slice element references create edges; `Ref[T]`, `Slice[T]`, and
`Map[K, V]` already introduce indirection.

- A type that participates in a direct or mutual cycle must cross an
  indirection. Small recursive enums use per-variant payload pointers; a
  recursive enum selected for compact storage uses its single `any` payload
  slot to break the cycle.
- A non-recursive enum normally remains a tagged value struct.
- Any enum whose estimated wide representation exceeds the compact form may
  use one `any` payload slot, including a recursive enum.
- A non-recursive enum also uses indirect variant payloads when a variant
  directly contains another declared value type. This prevents large unions
  such as `ast2.Decl` from being copied through every match and function call.
- Constructors take source-level value arguments and store them behind the
  selected pointer or interface payload; match bindings retain source types.
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
