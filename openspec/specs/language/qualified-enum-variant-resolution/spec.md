# qualified-enum-variant-resolution Specification

## Purpose

Ensures qualified enum-variant construction remains unambiguous when multiple
enums in a package declare variants with the same spelling and payload shape.

## Requirements

### Requirement: Qualified enum constructors are collision-safe

The compiler SHALL resolve a construction written as
`EnumName.VariantName { field: value }` using both `EnumName` and
`VariantName`. A declaration of `VariantName` by another enum in the same
package SHALL NOT alter the selected constructor, its payload-field validation,
or the resulting enclosing enum type.

#### Scenario: Named payload collides with a zero-payload variant

- **WHEN** `AgentState.Running { Loop: ctx }` is constructed and another enum
  in the same package declares a zero-payload `Running` variant
- **THEN** compilation SHALL select `AgentState.Running`, validate `Loop`, and
  infer the expression as `AgentState`

#### Scenario: Named payload variants share a spelling

- **WHEN** two enums in the same package each declare a named-payload variant
  with the same name but different fields or field types
- **THEN** a qualified construction SHALL validate only against the fields of
  its named enclosing enum
