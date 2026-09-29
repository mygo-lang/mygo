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

### Requirement: Bootstrap resolves embedded members with Go promotion semantics
When a struct declares an embedded type via the `embed` marker, bootstrap
inference SHALL make the embedded type's exported members reachable on the
embedding struct exactly as Go does, for both MyGO and `go:`-imported embedded
types, and at any nesting depth.

An embedded type SHALL be reachable by two distinct paths, and both SHALL
type-check and agree on types:

1. **Promoted access** — an exported member of an embedded type is reachable
   directly on the embedding value (`b.F1`), transitively through any number of
   embedding levels, and across MyGO/Go embedding boundaries in either
   combination.
2. **Qualified access** — the embedded field itself is addressable by the
   embedded type's name (`b.A`, and for a Go embed `u.Model`), so its members
   can be reached through the explicit path (`b.A.F1`).

Promotion SHALL cover exported **fields and methods** alike. Only members Go
itself promotes are in scope; unexported members remain invisible across the
package boundary.

When a member name is reachable at more than one depth, bootstrap SHALL adopt
Go's resolution rules:

- The shallowest reachable member wins.
- A member declared directly on the embedding struct wins over any promoted
  member of the same name.
- Ties at the same depth that originate from different embedded types SHALL be
  reported as an ambiguous selector rather than resolved arbitrarily.
- The qualified path (`b.A.F1`) is always unambiguous and never participates
  in ambiguity.

Generated Go SHALL read both paths as direct selectors so the Go compiler
applies its own promotion rules.

#### Scenario: Reading a field promoted from an embedded GORM model
- **WHEN** a function with a `User` parameter, where `struct User` contains
  `embed gorm.Model`, evaluates `u.ID` or `u.CreatedAt`
- **THEN** inference SHALL accept the selector and bind it to the embedded
  type's field type (an integer identifier and a timestamp respectively)
  rather than reporting `unknown field User.ID`, and the generated Go SHALL
  contain the direct selector expression `u.ID` / `u.CreatedAt`

#### Scenario: Reaching an embedded field by its type name
- **WHEN** a function with a `B` parameter, where `struct B` contains
  `embed A` and `struct A` declares `F1: Int`, evaluates `b.A` or `b.A.F1`
- **THEN** inference SHALL bind `b.A` to the embedded type `A` and `b.A.F1` to
  `A`'s declared field type, and the generated Go SHALL contain the direct
  selector expressions `b.A` / `b.A.F1`

#### Scenario: Multi-level promotion across MyGO embeds
- **WHEN** `struct A` declares `F1: Int`, `struct B` contains `embed A`, and
  `struct C` contains `embed B`, so that embedding reaches `F1` at depth two
- **THEN** `c.F1` SHALL type-check at the field's own declared type

#### Scenario: Multi-level promotion across a Go embed
- **WHEN** a MyGO struct embeds a `go:`-imported struct whose own underlying
  struct embeds a further struct, so that a field sits two levels down
- **THEN** the field SHALL be reachable directly on the MyGO struct value at
  its own declared type

#### Scenario: Promotion through a mixed MyGO and Go embed chain
- **WHEN** a MyGO struct embeds another MyGO struct which embeds a
  `go:`-imported struct (or the reverse ordering), so that a member sits two or
  more levels down across the FFI boundary
- **THEN** that member SHALL be reachable directly on the outermost MyGO value
  at its own declared type

#### Scenario: Promoted method call on an embedded Go type
- **WHEN** a function with a `User` parameter, where `struct User` contains
  `embed gorm.Model`, calls a method exported by `gorm.Model` on `u`
- **THEN** inference SHALL resolve the method on the `User` receiver and bind
  its result to the method's declared result type, and the generated Go SHALL
  contain a direct method call on the value

#### Scenario: Shallowest member wins at greater depth
- **WHEN** an outer struct declares a member with the same name as a member
  reachable through its embedded type, and also embeds a second type that
  contributes that name at a shallower depth
- **THEN** the shallower member SHALL be selected, matching Go's
  shallowest-wins rule

#### Scenario: Ambiguous promotion at equal depth
- **WHEN** a struct embeds two types that both contribute a member of the same
  name at the same depth, and the name is not declared on the struct itself
- **THEN** inference SHALL fail with an ambiguous selector error naming the
  member, rather than silently picking one of the two

#### Scenario: Own fields remain unaffected
- **WHEN** a function selects a field declared directly on the MyGO struct
  that also embeds a type contributing the same name
- **THEN** that field SHALL continue to resolve at its own declared type, as it
  did prior to embedding, and the generated Go SHALL contain the same direct
  selector

#### Scenario: Bare imported Go struct members still resolve
- **WHEN** a function selects a field or calls a method directly on a value of
  a `go:`-imported struct type, without any MyGO struct embedding it
- **THEN** that member SHALL continue to resolve as before, confirming the
  promotion work did not change the pre-existing path
