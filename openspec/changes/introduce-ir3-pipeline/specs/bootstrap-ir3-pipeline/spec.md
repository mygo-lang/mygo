## Purpose

Defines the compiler's package-level intermediate representation and the guarantees that lowering, optimization, and verification preserve observable MyGO behavior, source provenance, and deterministic output.

## ADDED Requirements

### Requirement: Package-level IR units preserve declaration provenance

The compiler SHALL lower each typed package into one IR unit containing all package declarations and imported declarations as signatures. Every IR construct capable of producing a diagnostic, a line directive, or an ordering-sensitive optimization SHALL retain its originating source path and position.

#### Scenario: Cross-file call keeps its source position
- **WHEN** a function in file A calls a function declared in file B of the same package
- **THEN** the IR node for that call retains file A's source path and position

#### Scenario: Imported declarations do not require bodies
- **WHEN** a package imports another MyGO package
- **THEN** the importing IR unit contains the imported declarations' signatures and type information without requiring their function bodies

### Requirement: Control flow and calls are explicit in the IR

The IR SHALL classify calls as direct, closure, method, dictionary, or foreign; SHALL mark calls in tail position; and SHALL represent join points and pattern-match failure with explicit continuation constructs rather than implicit flags.

#### Scenario: Tail call in a switch arm is marked tail
- **WHEN** a function's final expression is a switch whose selected arm calls the same or a mutually recursive function
- **THEN** that call is represented as a tail call in the IR

#### Scenario: Guard failure targets the next arm
- **WHEN** a switch arm has a guard that evaluates to false
- **THEN** the IR expresses the transfer as an explicit failure continuation rather than a mutable matched flag

### Requirement: Effects guard optimization

The IR SHALL annotate operations with effects sufficient to prevent dead-code elimination, common-subexpression elimination, reordering, and hoisting from changing observable behavior. Unknown Go FFI and inline Go operations SHALL be treated as arbitrary or foreign effects.

#### Scenario: Foreign call is not moved or removed
- **WHEN** an unused call to a Go FFI function appears between two observable operations
- **THEN** optimization SHALL NOT delete or reorder that call

#### Scenario: Pure local can be eliminated
- **WHEN** a local value is produced by a pure operation and is never observed
- **THEN** optimization MAY eliminate the operation without changing behavior

### Requirement: Every pass preserves observable semantics

Each IR lowering and optimization pass SHALL preserve the observable behavior of the source program, including evaluation order, panics and their ordering, mutation, reference identity, Go FFI effects, dynamic dispatch, and result values.

#### Scenario: Evaluation order across side effects
- **WHEN** two operands have observable side effects
- **THEN** the generated Go evaluates them in source order

#### Scenario: Optimization does not suppress a panic
- **WHEN** a source expression must panic before a later side effect
- **THEN** the optimized program still panics before performing that side effect

### Requirement: IR construction and output are deterministic

For identical inputs and compiler version, the compiler SHALL produce identical IR dumps and generated Go files. Any traversal backed by an unordered collection SHALL impose a stable order before it affects output or optimization decisions.

#### Scenario: Repeated compilation is byte-identical
- **WHEN** the same package is compiled twice with the same compiler and options
- **THEN** the generated files and IR dumps are byte-identical

### Requirement: IR is verified before code generation

The compiler SHALL verify IR structure before code generation. Verification SHALL reject malformed control flow, missing or invalid required types, inconsistent value representations, and effect information that would make a required optimization unsound.

#### Scenario: Invalid jump target is rejected
- **WHEN** a jump refers to a continuation that is not in scope
- **THEN** compilation fails with a diagnostic identifying the invalid control-flow edge

#### Scenario: Representation mismatch is rejected
- **WHEN** a value's declared representation disagrees with its use site
- **THEN** verification fails before any Go source is emitted

### Requirement: The IR pipeline has a correct non-optimizing path

The compiler SHALL provide an optimization level that performs no optimizing transformation while preserving correct behavior, so that lowering and backend defects can be isolated from optimizer defects.

#### Scenario: Non-optimizing pipeline remains correct
- **WHEN** the IR pipeline runs with optimization disabled
- **THEN** the generated Go is behaviorally equivalent to the source without relying on optimizing passes
