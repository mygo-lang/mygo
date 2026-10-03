# Spec Delta

## ADDED Requirements

### Requirement: Named Go types retain imported package identity across FFI boundaries
Inference SHALL treat references to the same named Go type from an imported Go package as the same type across Go signatures, MyGO package signatures, and callback values, while keeping distinct named Go types distinct.

#### Scenario: Callback passes a named Go type to a MyGO package method
- **WHEN** a callback parameter uses `Ref[mail.Message]` from a `go:net/mail` import and passes its dereferenced value to a MyGO package method expecting `mail.Message` from the same import
- **THEN** inference SHALL accept the call because both signatures refer to the same Go named type

#### Scenario: Distinct Go named types remain incompatible
- **WHEN** a value of one imported named Go type is passed where a different named Go type is required, even if both have identical underlying representations
- **THEN** inference SHALL report a type mismatch
