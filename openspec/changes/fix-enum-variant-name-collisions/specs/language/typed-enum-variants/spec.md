## MODIFIED Requirements

### Requirement: Named-struct variant construction
Code SHALL construct a named-struct variant value with
`EnumName.VariantName { field: expr, ... }`, where every specified field name
SHALL exist on the variant and SHALL be specified at most once. Omitting a
field SHALL be allowed unless a construction omits a required field. The
qualified enum and variant names SHALL determine construction even when another
enum in the same package declares a variant with the same unqualified name.

#### Scenario: Constructing a named-struct variant with all fields
- **WHEN** source contains `let c = Shape.Circle { radius: 5.0 }`
- **THEN** `c` SHALL have type `Shape` and represent a circle with radius `5.0`

#### Scenario: Unknown field in a named-struct variant literal
- **WHEN** a construction names a field that does not exist on the variant
- **THEN** compilation SHALL fail with a source-location diagnostic

#### Scenario: Another enum reuses the variant name
- **WHEN** source constructs `AgentState.Running { Loop: ctx }` and a second
  enum in the package declares `Running`
- **THEN** the construction SHALL use `AgentState.Running` rather than the
  second enum's variant
