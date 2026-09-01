# typed-enum-variants Specification

## Purpose

Defines the source-language contract for enum variant payloads in MyGO,
including named-struct variants, their construction, and matching, while
preserving the existing tuple-form behavior.

## Requirements

### Requirement: Enum variants support named-struct payloads
An enum variant declaration SHALL support an optional named-struct payload of
the form `Name { field: Type, ... }` in addition to the existing tuple payload
form `Name(Type, ...)`. Each named field SHALL have a non-empty name and a
type. An enum MAY mix tuple variants and named-struct variants in the same
declaration. A single variant SHALL NOT combine tuple and named-struct payload
forms.

#### Scenario: A named-struct variant declares fields
- **WHEN** an enum is declared with a variant `Circle { radius: Float64 }`
- **THEN** the declaration SHALL compile and the variant SHALL expose a single
  field named `radius` of type `Float64`

#### Scenario: A single enum mixes tuple and named-struct variants
- **WHEN** an enum declares `Shape.Circle { radius: Float64 }` and
  `Shape.Rectangle(Float64, Float64)` in the same `enum Shape` block
- **THEN** compilation SHALL succeed

### Requirement: Named-struct variant construction
Code SHALL construct a named-struct variant value with
`EnumName.VariantName { field: expr, ... }`, where every specified field name
SHALL exist on the variant and SHALL be specified at most once. Omitting a
field SHALL be allowed unless a construction omits a required field.

#### Scenario: Constructing a named-struct variant with all fields
- **WHEN** source contains `let c = Shape.Circle { radius: 5.0 }`
- **THEN** `c` SHALL have type `Shape` and represent a circle with radius `5.0`

#### Scenario: Unknown field in a named-struct variant literal
- **WHEN** a construction names a field that does not exist on the variant
- **THEN** compilation SHALL fail with a source-location diagnostic

### Requirement: Struct patterns for named-struct variants
Pattern matching SHALL support matching a named-struct variant with
`case VariantName { field }`, which binds the field to a local variable of the
same name, or `case VariantName { field: bound }`, which binds the field's
value to `bound`. The special binding `_` SHALL discard a field's value and
SHALL NOT introduce a binding.

#### Scenario: A named-struct variant pattern binds by name
- **WHEN** a switch matches `Shape.Circle` and the pattern is `Circle { r }`
- **THEN** `r` SHALL be bound to the value of the `radius` field

#### Scenario: A named-struct variant pattern uses a different binding name
- **WHEN** a switch matches `Shape.Circle` and the pattern is
  `Circle { radius: r }`
- **THEN** `r` SHALL be bound to the value of the `radius` field

#### Scenario: A named-struct variant pattern ignores selected fields
- **WHEN** a switch matches `Shape.Rectangle` and the pattern is
  `Rectangle { width, _ }`
- **THEN** the pattern SHALL match and bind `width`, and SHALL introduce no
  binding for the discarded field

### Requirement: Partial struct patterns
A named-struct variant pattern SHALL be allowed to mention a subset of that
variant's fields. The pattern SHALL match if the runtime value is an instance
of the variant; unmentioned fields SHALL be ignored.

#### Scenario: A pattern matches a subset of named fields
- **WHEN** a switch matches `Shape.Rectangle` and the pattern is
  `Rectangle { width }`
- **THEN** the pattern SHALL match without specifying `height`

### Requirement: Duplicate named fields are rejected
A variant declaration SHALL NOT declare two fields with the same name, and a
pattern SHALL NOT bind the same name twice.

#### Scenario: Duplicate declaration fields
- **WHEN** an enum variant declares `Point { x: Float64, x: Float64 }`
- **THEN** compilation SHALL fail with a diagnostic naming the duplicate field

#### Scenario: Duplicate pattern bindings
- **WHEN** a pattern binds `Circle { x, x }`
- **THEN** compilation SHALL fail with a diagnostic naming the duplicate
  binding
