## MODIFIED Requirements

### Requirement: Package analysis with document overlays
The server SHALL discover MyGO source packages in initialized workspace folders, include unopened same-package files and imported workspace MyGO packages, and resolve imports and Prelude consistently with the compiler. Open buffers MUST override disk text for the package itself and for every declaration used to infer dependent packages, including inferred package-level bindings. Dependency inference MUST NOT use saved disk text while a current open buffer exists. Closing the buffer MUST restore disk-derived declarations for subsequent analysis. Generated Go files MUST NOT be indexed as MyGO source. External Go packages SHALL supply available type metadata; external Go source navigation is outside this capability.

#### Scenario: Unopened sibling definition
- **WHEN** an open file uses a declaration from an unopened file in the same package
- **THEN** analysis exposes that declaration and its source location

#### Scenario: Unsaved imported package change
- **WHEN** an open imported workspace file changes a declaration signature without saving
- **THEN** subsequent requests in dependent packages reflect the new signature

#### Scenario: Unsaved dependency signature affects dependent analysis
- **WHEN** an open imported workspace file changes a function signature or package-level binding without saving
- **THEN** dependents that consume the changed declaration use the overlay-derived signature, report any resulting type errors, and omit navigation targets that are no longer valid

#### Scenario: Imported overlay close restores disk declarations
- **WHEN** a dependent package was analyzed while an imported workspace file had unsaved signature changes, and that file is closed
- **THEN** subsequent analysis uses the on-disk declarations and restores diagnostics and navigation derived from the saved source
