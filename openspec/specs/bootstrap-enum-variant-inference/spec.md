# bootstrap-enum-variant-inference Specification

## Purpose

Ensure bootstrap type inference keeps independently created fresh type
variables distinct while evaluating fields of named enum-variant literals.

## Requirements

### Requirement: Named enum-variant fields preserve inference progress
The bootstrap type inference pipeline SHALL use the inference state produced by
each named enum-variant field when evaluating the next field and the enclosing
expression.

#### Scenario: Empty slice follows a named enum-variant literal
- **WHEN** a named enum-variant literal contains a field expression that
  allocates fresh type variables and is followed by an empty slice expression
- **THEN** the empty slice SHALL receive an independent fresh element type and
  SHALL NOT inherit a type binding from the preceding field expression

### Requirement: Independent later expressions retain their declared context
The bootstrap type inference pipeline SHALL allow a later empty slice to unify
with its independently required element type after a named enum-variant literal
has been evaluated.

#### Scenario: Later empty slice is constrained by a result type
- **WHEN** a function returns a tuple containing a named enum-variant literal
  and an empty slice with an annotated element type
- **THEN** inference SHALL accept the function without a type-variable collision
