# Spec Delta

## Purpose

Defines how generated Go files import the MyGO prelude when their emitted declarations reference prelude-provided functions, types, or generated helpers.

## ADDED Requirements

### Requirement: Generated files import referenced prelude declarations
The code generator SHALL include the MyGO prelude import in a generated Go file whenever that file references a declaration provided by the prelude, regardless of how that declaration's types are represented in generated Go.

#### Scenario: Direct call to a prelude helper with built-in FFI types
- **WHEN** a MyGO source file calls `Panic` and the prelude metadata represents its parameter using a Go built-in type
- **THEN** the generated Go file imports the prelude package and the call resolves to the prelude declaration

#### Scenario: Prelude reference does not cause unused imports
- **WHEN** a generated Go file does not reference any prelude declaration or generated prelude helper
- **THEN** the generated Go file does not include an otherwise unused prelude import
