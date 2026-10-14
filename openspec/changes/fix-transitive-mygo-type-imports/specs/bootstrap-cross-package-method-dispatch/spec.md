## ADDED Requirements

### Requirement: Imported API results retain defining package identity
Bootstrap SHALL preserve the defining package identity of nominal MyGO types returned by imported functions and impl methods, including types defined in a third package that the consumer does not directly import. Subsequent generic inference SHALL use that identity without requiring a consumer type annotation.

#### Scenario: A chained Result method consumes a third-package type
- **WHEN** `server` imports only `storage`, a storage repository method returns `Result[auth.Identity, String]`, and server calls that method followed by `Unwrap()` without a type annotation
- **THEN** bootstrap SHALL infer the unwrapped value as the Identity type defined by auth
- **AND** the generated generic helper arguments SHALL refer to that defining package and the generated Go SHALL compile and return the expected Identity value

#### Scenario: An imported function returns nested foreign types
- **WHEN** an imported function returns a type containing a third-package nominal type within a generic application, collection, tuple, or function signature
- **THEN** subsequent inferred generic arguments and generated typed declarations SHALL retain the defining package of each nested nominal type

### Requirement: Emitted inferred foreign types receive file-local imports
Bootstrap SHALL provide an appropriate file-local Go import and qualifier for every foreign MyGO nominal type referenced by emitted inferred generic arguments or typed declarations. It SHALL reuse a usable explicit import for the same path, choose deterministic distinct aliases when paths compete for an alias, and omit unused imports and imports of the current package.

#### Scenario: A defining package is not directly imported by the consumer
- **WHEN** a generated file needs an inferred third-package type but its MyGO source does not import that package
- **THEN** the generated file SHALL import the defining package and use its selected alias for every emitted reference to that type

#### Scenario: An explicit alternate alias already exists
- **WHEN** the source imports the defining package under an alternate alias and an inferred type requires the same package
- **THEN** the generated file SHALL reuse that alias without adding a duplicate import

#### Scenario: Two defining packages compete for an alias
- **WHEN** inferred types from two distinct packages require the same preferred alias, or an explicit import occupies that alias
- **THEN** the generated file SHALL use deterministic distinct aliases and each type reference SHALL select its own defining package

#### Scenario: Sibling files need different imports
- **WHEN** one file emits an inferred third-package type and another file in the same package does not emit any reference to that defining package
- **THEN** only the file that needs the type SHALL receive that inferred import

#### Scenario: A result type belongs to the consuming package
- **WHEN** an imported API returns a nominal type whose defining package is the consuming package in an otherwise valid package dependency graph
- **THEN** the generated reference SHALL use the local type name without importing the current package

### Requirement: Missing foreign type qualification is diagnosed before Go output
If bootstrap cannot resolve a required foreign nominal type's package to a usable Go import and qualifier, it SHALL report a generation diagnostic identifying the type and defining package instead of successfully emitting the type as a bare local name. This requirement SHALL NOT prohibit bare names for current-package types or explicitly dot-imported types.

#### Scenario: Required foreign package mapping cannot be resolved
- **WHEN** generation needs a foreign nominal type and cannot establish a usable mapping for its defining package
- **THEN** generation SHALL fail with a diagnostic identifying the unresolved type and package rather than succeeding with an unqualified foreign reference
