## ADDED Requirements

### Requirement: Bootstrap imports exported package-level Go variables
When a bootstrap-compiled program selects an exported package-level `var`
through a `go:`-imported package (for example `gorm.ErrRecordNotFound` from
`go:gorm.io/gorm`, or `os.ErrNotExist` from `go:os`), bootstrap SHALL treat the
selector as a value of the variable's declared Go type, exactly as it already
does for an exported package-level `const`. A package that exports error
values as `var`s rather than `const`s SHALL be fully usable at this boundary.

#### Scenario: Selecting a GORM sentinel error variable
- **WHEN** a function evaluates `gorm.ErrRecordNotFound` where `gorm` is a
  `go:gorm.io/gorm` import
- **THEN** inference SHALL accept the selector and bind the expression to the
  variable's declared type (`error`), and the generated Go SHALL reference
  `gorm.ErrRecordNotFound` directly (no `unknown Go package member` error)

#### Scenario: Variable and constant selectors agree
- **WHEN** a function selects an exported package-level `var` and an exported
  package-level `const` from the same `go:`-imported package
- **THEN** both SHALL type-check as values of their respective declared types,
  with no behavioral difference between the two at this boundary

### Requirement: Bootstrap resolves fields promoted from embedded Go structs
When a MyGO struct declares an embedded Go type via the `embed` marker (for
example `embed gorm.Model`), bootstrap inference SHALL resolve an exported
field of the embedded type when that field is selected on a value of the
embedding MyGO struct, and SHALL report the field at its own declared Go type.
The generated Go SHALL read the promoted field as a direct selector so the Go
compiler applies its own promotion rules.

#### Scenario: Reading a field promoted from an embedded GORM model
- **WHEN** a function with a `User` parameter, where `struct User` contains
  `embed gorm.Model`, evaluates `u.ID` or `u.CreatedAt`
- **THEN** inference SHALL accept the selector and bind it to the embedded
  type's field type (an integer identifier and a timestamp respectively)
  rather than reporting `unknown field User.ID`, and the generated Go SHALL
  contain the direct selector expression `u.ID` / `u.CreatedAt`

#### Scenario: Own fields remain unaffected
- **WHEN** a function selects a field declared directly on the MyGO struct
  that also embeds a Go type
- **THEN** that field SHALL continue to resolve as before, and the generated
  Go SHALL contain the same direct selector it did prior to embedding
