## Purpose

Defines the canonical codegen3 generated-Go layouts for MyGO algebraic data types, generics, and the boundary adaptations that keep FFI and dynamic dispatch correct. Codegen2 continues to emit its legacy interface-based ADT ABI and is not governed by this matrix.

## ADDED Requirements

### Requirement: Value representation is canonical and compiler-versioned

For a given codegen3 type declaration and compiler version, the compiler SHALL produce the same generated-Go representation in every package compilation. A codegen3 build SHALL use one compiler version for all MyGO packages; generated output produced by a different compiler version SHALL NOT be mixed into the same build.

#### Scenario: Packages in one build agree
- **WHEN** two packages are compiled in the same build with the same compiler version and one uses a type declared by the other
- **THEN** both packages use the same Go layout for that type

#### Scenario: Mixed compiler versions are rejected
- **WHEN** a build would combine generated output from a different compiler version
- **THEN** compilation fails with a compatibility diagnostic instead of mixing layouts

#### Scenario: Representation version participates in cache keys
- **WHEN** a cached generated package is considered for reuse
- **THEN** its representation version and adapter ABI must match the current compiler; otherwise the entry is ignored and the package is regenerated

### Requirement: Option uses a value representation without interface assertions

In codegen3, a general `Option[A]` SHALL be represented as a generated Go value carrying a tag and variant payloads. `Option[Ref[T]]` SHALL be represented as `*T`, with `nil` denoting `None`. Constructing, inspecting, or matching an `Option` SHALL NOT require an interface type assertion. Codegen2 keeps its established interface `Option` ABI.

#### Scenario: Some and None round-trip
- **WHEN** a value is wrapped in `Some` and then matched
- **THEN** the original payload and the presence flag are preserved without an interface assertion

#### Scenario: Option of a reference uses nil
- **WHEN** an `Option[Ref[T]]` is `None`
- **THEN** its generated Go value is `nil`, and `Some(ref)` preserves the non-nil reference

### Requirement: Result uses a value representation with FFI adaptation

In codegen3, a general `Result[A, E]` SHALL be represented as a generated Go tagged value carrying both variant payloads. At a Go FFI boundary, a `Result[T, error]` SHALL be adapted to and from Go's `(T, error)` shape without changing which variant is observed. Codegen2 keeps its established interface `Result` ABI.

#### Scenario: Ok and Err preserve their payloads
- **WHEN** a `Result` is constructed as `Ok(v)` or `Err(e)` and then matched
- **THEN** the matching arm observes the same payload and variant

#### Scenario: FFI error is adapted to Err
- **WHEN** a Go FFI function returns a non-nil error
- **THEN** the adapted MyGO `Result` is `Err` carrying that error

### Requirement: List is a linked structure with non-allocating tail access

`List[A]` SHALL be represented as a linked structure terminated by `nil`. Obtaining the tail of a non-empty list SHALL NOT allocate a new wrapper or presence value, and copying a list value SHALL share the underlying chain.

#### Scenario: Tail of a cons cell
- **WHEN** `Tail` is called on a non-empty list
- **THEN** the result denotes the next node without allocating a new presence wrapper

#### Scenario: List sharing is preserved
- **WHEN** a list value is copied and one copy's chain is traversed
- **THEN** both copies observe the same underlying nodes

### Requirement: Enum representation has finite size

In codegen3, non-recursive enums SHALL use a tagged value representation. Enums whose variants directly or mutually reference the enum SHALL introduce indirection so that every generated Go type has finite size. A non-recursive enum whose variant directly contains another declared value type SHALL indirect that variant payload to avoid repeated large-value copies. Matching SHALL dispatch on the tag rather than on Go interface type assertions.

#### Scenario: Recursive enum compiles
- **WHEN** an enum variant directly contains a value of its own enum type
- **THEN** the generated Go represents that field through indirection and the package compiles

#### Scenario: Tag dispatch
- **WHEN** a value of a non-recursive enum is matched
- **THEN** the generated Go selects the arm from the enum tag

#### Scenario: Large non-recursive payload
- **WHEN** a non-recursive enum variant directly contains another declared value type
- **THEN** codegen3 stores that variant payload indirectly while preserving the source-level constructor and match types

### Requirement: First-order generics preserve Go type parameters

For first-order generic types and functions, the generated Go SHALL preserve the MyGO type parameter as a Go type parameter rather than erasing it to `any`. Higher-kinded or dynamically boxed values MAY remain erased when Go cannot express them directly.

#### Scenario: Generic Option keeps its element type
- **WHEN** a generic function declares a parameter of type `Option[T]`
- **THEN** the generated Go signature uses the type parameter `T` rather than `any` for the payload

#### Scenario: HKT remains expressible
- **WHEN** a type uses higher-kinded application that Go cannot express directly
- **THEN** the compiler uses its existing erased encoding without breaking compilation

### Requirement: Boundaries adapt representations without semantic drift

FFI calls, dynamic-typeclass dispatch, and inline Go boundaries SHALL convert between canonical MyGO representations and the external Go shapes they require. If an inline Go snippet or FFI signature cannot be adapted without ambiguity, the compiler SHALL reject it with a diagnostic rather than emit a silently incorrect conversion.

#### Scenario: Dynamic typeclass value round-trip
- **WHEN** a canonical enum or `Option` value is boxed into a dynamic typeclass value and dispatched
- **THEN** the observed payload and method behavior match the unboxed value

#### Scenario: Incompatible inline Go shape is rejected
- **WHEN** an inline Go snippet requires the previous interface-based enum shape
- **THEN** compilation fails with a diagnostic identifying the incompatible boundary
