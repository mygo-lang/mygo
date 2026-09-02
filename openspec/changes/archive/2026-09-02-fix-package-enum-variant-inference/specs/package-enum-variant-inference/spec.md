## Purpose

Ensure package-level bootstrap inference recognizes named enum-variant literals
as values of their enclosing enum so valid collection and struct assignments
compile consistently across single-file and multi-file packages.

## ADDED Requirements

### Requirement: Package inference resolves named enum-variant literals

When inferring a package, the compiler SHALL retain the declarations of the
current package for named enum-variant lookup. A literal written as
`EnumName.VariantName { field: value }` for a declared named variant SHALL have
the enclosing `EnumName` type, including when inference includes external
declarations.

#### Scenario: Named variant in an enum-typed slice field

- **WHEN** package inference processes a struct field of type `Slice[Content]`
  initialized with `[Content.Text { Text: text }]`
- **THEN** inference SHALL accept the field value as `Slice[Content]`

#### Scenario: Bootstrap package inference with external declarations

- **WHEN** bootstrap compilation infers a package together with external
  declarations and the package constructs a named enum variant
- **THEN** inference SHALL resolve the literal to its enclosing enum type
  without reporting a `Content` versus `Content.Text` unification error
