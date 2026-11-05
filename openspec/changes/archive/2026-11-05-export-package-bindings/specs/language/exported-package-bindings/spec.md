## Purpose

Defines how a MyGO package exposes package-level `let` and `var` bindings to
importing packages, including naming, type resolution and identity, mutability,
and metadata visibility.

## ADDED Requirements

### Requirement: Exported package bindings are importable

A package-level `let` or `var` whose name starts with an ASCII uppercase
letter SHALL be a member of that package's exported surface. An importing
MyGO package SHALL reference the binding as `alias.Name`, where `alias` is the
package's import alias. A package-level binding whose name starts with a
lowercase letter or a non-letter SHALL remain private to its defining package.

An exported package binding's type SHALL be its explicit type annotation when
one is present, or otherwise the type inferred for its initializer by the
defining package's inference pass. A package-level binding with an uppercase
name SHALL NOT be rejected solely because it lacks a type annotation.

#### Scenario: An importing package reads annotated bindings
- **WHEN** package `dep` declares `let DefaultPort: Int = 8080` and
  `var Active: Bool = false`, and package `app` imports `dep`
- **THEN** `dep.DefaultPort` and `dep.Active` SHALL resolve in `app` with types
  `Int` and `Bool`, respectively

#### Scenario: An unannotated binding exports its inferred type
- **WHEN** package `dep` declares `let DefaultPort = 8080` with no annotation
  and package `app` reads `dep.DefaultPort`
- **THEN** the reference SHALL resolve with type `Int` and SHALL NOT produce an
  annotation-required diagnostic

#### Scenario: An unannotated struct value exports its inferred type
- **WHEN** package `dep` declares `struct Box` plus
  `let Default = Box { Value: 1 }` with no annotation, and package `app` reads
  `dep.Default`
- **THEN** `dep.Default` SHALL have type `dep.Box`

#### Scenario: An importer initializes a binding from an imported binding
- **WHEN** package `app` declares `let Port: Int = dep.DefaultPort`
- **THEN** initializing `app` SHALL observe the value assigned by `dep`'s own
  initialization, not the Go zero value

#### Scenario: A lowercase binding stays private
- **WHEN** package `dep` declares `let defaultPort: Int = 8080` and package
  `app` references `dep.defaultPort`
- **THEN** compilation SHALL fail with an unresolved package member diagnostic

### Requirement: Imported binding types preserve package identity

The type of an imported package binding SHALL be interpreted in the defining
package's context. A named type declared by the defining package SHALL remain
qualified by that package when the binding is used by an importer, even when
the importer declares a type with the same name. Builtin, prelude, and
third-party package types SHALL keep their existing identities.

#### Scenario: A local type with the same name does not capture the binding
- **WHEN** package `dep` declares `struct Box` plus `let Default: Box` and
  package `app` declares its own `struct Box` and reads `dep.Default`
- **THEN** `dep.Default` SHALL have type `dep.Box`, and field access SHALL use
  the fields declared by `dep.Box`

#### Scenario: A collection carries the defining package's element type
- **WHEN** package `dep` declares `let Defaults: List[Box]` and package `app`
  reads an element of `dep.Defaults`
- **THEN** the element SHALL have type `dep.Box`

### Requirement: Exported var is assignable and exported let is immutable

Assigning to an exported package `var` through a package qualifier SHALL be
allowed and SHALL type-check the assigned value against the resolved type.
Assigning to an exported package `let` through a package qualifier SHALL be
rejected with a diagnostic. Reads of both forms SHALL be allowed.

#### Scenario: An importer assigns an exported var
- **WHEN** `dep` declares `var Active: Bool = false` and `app` executes
  `dep.Active = true`
- **THEN** compilation SHALL succeed and a later read of `dep.Active` SHALL
  observe `true`

#### Scenario: An importer assigns an exported var with the wrong type
- **WHEN** `dep` declares `var Count: Int = 0` and `app` assigns a `String` to
  `dep.Count`
- **THEN** compilation SHALL fail with a type mismatch diagnostic

#### Scenario: An importer assigns an exported let
- **WHEN** `dep` declares `let Limit: Int = 10` and `app` executes
  `dep.Limit = 20`
- **THEN** compilation SHALL fail with a diagnostic stating that the imported
  `let` binding is immutable

### Requirement: Exported bindings survive metadata-only loading

Generated package metadata SHALL record every exported package binding with
its name, resolved type (annotation or inferred), and mutability. When a
dependency is loaded from metadata without its MyGO sources, the reconstructed
declarations SHALL make the same exported bindings available to importers as
the source-backed path.

#### Scenario: A dependency is loaded from metadata
- **WHEN** `dep`'s metadata is the only available description of `dep` and
  package `app` references an annotated binding and an unannotated binding
  whose type was inferred
- **THEN** both references SHALL resolve with the recorded types, and the
  recorded mutability SHALL drive the same assignment rules as source loading

#### Scenario: Metadata written before this capability is loaded
- **WHEN** a metadata document has no exported binding entries
- **THEN** loading SHALL succeed and the package SHALL expose no package-level
  value members, preserving the previous behavior

### Requirement: Both code generation paths agree on exported bindings

The codegen2 and codegen3 paths SHALL accept the same exported package binding
fixtures, including unannotated bindings with inferred types, and SHALL produce
the same observable results for reads and for assignment to exported `var`
bindings. Codegen3 SHALL use the inferred type for an unannotated package
value and SHALL NOT reject it solely because no annotation is present.

#### Scenario: The same two-package fixture runs on both paths
- **WHEN** a fixture with annotated and unannotated exported `let` and `var`
  bindings, and reads from a second package, is compiled through codegen2 and
  through codegen3
- **THEN** both paths SHALL compile it and produce equal values and equal
  diagnostic-free runtime behavior

#### Scenario: Codegen3 compiles an unannotated package value
- **WHEN** a package declares `let DefaultPort = 8080` without an annotation
  and the codegen3 path has an inferred type for the initializer
- **THEN** codegen3 SHALL emit the package variable using the inferred type and
  SHALL NOT report an annotation-required diagnostic

### Requirement: Export does not add declaration-order visibility

Exporting an unannotated package binding SHALL NOT make it visible to
declarations that appear earlier in its defining package's source order. An
annotated binding SHALL keep its existing package-wide predeclaration behavior.

#### Scenario: An earlier declaration references a later unannotated binding
- **WHEN** a function declared before `let DefaultPort = 8080` references
  `DefaultPort` inside the same package
- **THEN** inference SHALL report the unresolved identifier as it does today

#### Scenario: An earlier declaration references a later annotated binding
- **WHEN** a function declared before `let DefaultPort: Int = 8080` references
  `DefaultPort` inside the same package
- **THEN** inference SHALL resolve the binding through predeclaration
