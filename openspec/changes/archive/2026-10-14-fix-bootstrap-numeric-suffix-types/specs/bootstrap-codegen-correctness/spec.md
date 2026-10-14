## ADDED Requirements

### Requirement: Bootstrap preserves explicit numeric suffix types
Bootstrap SHALL preserve the numeric type selected by every supported explicit numeric literal suffix in generated Go expressions: `i8/i16/i32/i64` map to `int8/int16/int32/int64`, `u/u8/u16/u32/u64` map to `uint/uint8/uint16/uint32/uint64`, and `f32/f64` map to `float32/float64`. This guarantee SHALL apply independently of whether the surrounding Go context supplies a type, including local bindings and call arguments.

#### Scenario: Local let retains Int64
- **WHEN** a function binds `let count = 0i64` and passes `count` to a function accepting `Int64`
- **THEN** the generated Go SHALL compile and the local value SHALL have Go type `int64`

#### Scenario: Local var retains each supported suffix type
- **WHEN** local mutable bindings are initialized with valid literals using each supported explicit numeric suffix
- **THEN** each generated local SHALL have the corresponding Go type and preserve the literal value
- **AND** later assignments and calls compatible with that type SHALL compile

#### Scenario: Call arguments preserve numeric type without contextual conversion
- **WHEN** a suffixed literal is passed directly to an unconstrained generic identity function or a Go function accepting `any`
- **THEN** the generic result or observed dynamic value SHALL have the suffix-selected Go type

#### Scenario: Unary and arithmetic expressions retain the selected type
- **WHEN** a local binding uses a valid negated suffixed literal or arithmetic between literals carrying the same suffix
- **THEN** its generated Go SHALL compile and retain the selected numeric type and result value

#### Scenario: Prefixed and separated integer literals preserve value
- **WHEN** a valid hexadecimal, octal, binary, or underscore-separated integer literal carries an explicit integer suffix
- **THEN** the generated Go SHALL preserve both its selected integer type and its numeric value

### Requirement: Numeric suffix preservation remains compatible with existing numeric contexts
Bootstrap SHALL preserve valid explicitly annotated bindings, package-level bindings, typed return values, and collection elements using suffixed literals. Unsuffixed literals SHALL retain their existing inference and contextual behavior.

#### Scenario: Annotated and package bindings retain Int64
- **WHEN** a source declares `let count: Int64 = 0i64` locally or `let count = 0i64` at package scope
- **THEN** the generated binding SHALL compile with Go type `int64` and value zero

#### Scenario: Typed returns and collections remain buildable
- **WHEN** a function returns an `i64` literal as `Int64` or creates a `Slice[Int64]` containing `i64` literals
- **THEN** the generated Go SHALL compile and preserve the specified values and element types

#### Scenario: Unsuffixed defaults and contextual literals remain unchanged
- **WHEN** a source uses local unannotated `0` and `3.14`, or an unsuffixed literal in an already supported typed context
- **THEN** generated Go SHALL retain the existing default `int` and `float64` types or the existing contextual type respectively
