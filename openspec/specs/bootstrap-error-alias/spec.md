# bootstrap-error-alias Specification

## Purpose

Defines the builtin-error alias contract for self-hosted MyGO compilation so
both accepted spellings infer as the same type and generated Go signatures use
Go's builtin `error` type.

## Requirements

### Requirement: Go error spellings are one builtin type
The self-hosted compiler SHALL treat `Error` and `error` as two source
spellings of the same Go builtin error type during inference. Neither spelling
SHALL be rejected solely because the other spelling was used at an equivalent
source or imported-boundary position.

#### Scenario: A value crosses differently spelled error annotations
- **WHEN** one function declares an `error` parameter and a caller passes a
  value inferred as `Error`
- **THEN** self-hosted inference SHALL accept the call without reporting that
  the two constructor names cannot unify

#### Scenario: A return annotation crosses differently spelled error types
- **WHEN** an expression inferred as `Error` is returned from a function whose
  return type is annotated as `error`
- **THEN** self-hosted inference SHALL accept that function as type-correct

### Requirement: Generated declarations lower the error builtin
The self-hosted generator SHALL emit Go's `error` type for both accepted
source spellings in declaration signatures. This SHALL apply to ordinary
parameters, generic-free return values, and each component of a tuple return.

#### Scenario: An Error parameter and return are lowered
- **WHEN** a MyGO declaration has an `Error` parameter and an `Error` result
- **THEN** the generated Go signature SHALL contain `error` in both positions
  and SHALL NOT contain a bare `Error` type identifier from that builtin alias

#### Scenario: A tuple return contains the builtin alias
- **WHEN** a MyGO function declares a tuple return that includes an `Error`
  component
- **THEN** the corresponding Go result component SHALL be `error`

#### Scenario: A lowercase error annotation is lowered
- **WHEN** a MyGO declaration spells a builtin parameter or result type as
  `error`
- **THEN** the corresponding generated Go signature SHALL use `error`

### Requirement: Explicit type parameters shadow the builtin alias
If a generic declaration names a type parameter `Error`, its explicit parameter
identity SHALL take precedence when lowering that declaration's type
expressions. The builtin alias lowering SHALL NOT rewrite such a type parameter
to Go's `error` builtin.

#### Scenario: A generic declaration names its parameter Error
- **WHEN** a MyGO function declares the type parameter `Error` and uses that
  name as a parameter and result type
- **THEN** the generated Go signature SHALL preserve that declared type
  parameter rather than emitting `error` in those positions
