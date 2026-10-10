## Purpose

Define consistent MyGO workspace analysis for editor features, so unsaved changes, package dependencies and incomplete source text produce current and accurately located results.

## ADDED Requirements

### Requirement: Package analysis with document overlays
The server SHALL discover MyGO source packages in initialized workspace folders, include unopened same-package files and imported workspace MyGO packages, and resolve imports and Prelude consistently with the compiler. Open buffers MUST override disk text. Generated Go files MUST NOT be indexed as MyGO source. External Go packages SHALL supply available type metadata; external Go source navigation is outside this capability.

#### Scenario: Unopened sibling definition
- **WHEN** an open file uses a declaration from an unopened file in the same package
- **THEN** analysis exposes that declaration and its source location

#### Scenario: Unsaved imported package change
- **WHEN** an open imported workspace file changes a declaration signature without saving
- **THEN** subsequent requests in dependent packages reflect the new signature

### Requirement: Current analysis results
The server SHALL invalidate affected package and dependent analysis when overlays or relevant disk files change, including file creation and deletion. Results and diagnostics MUST correspond to the current document version; obsolete analysis MUST NOT overwrite newer results.

#### Scenario: Disk change in an unopened dependency
- **WHEN** an unopened dependency changes on disk before the next request
- **THEN** that request uses refreshed analysis, whether the client supplied a file notification or the server detected the change itself

### Requirement: Source diagnostics
The server SHALL publish syntax diagnostics on open and change using standard publishDiagnostics notifications. For syntactically valid packages it SHALL additionally report available type and import errors at the originating source location. Corrected errors MUST be removed; strings and comments MUST NOT trigger keyword deprecation warnings.

#### Scenario: Error correction
- **WHEN** a document changes from a syntax or type error to valid source
- **THEN** the server publishes updated diagnostics that remove the corrected error

### Requirement: Incomplete input degradation
The server SHALL remain responsive for incomplete or invalid source, preserve recoverable syntax features, and omit unavailable semantic information rather than returning stale or invented types and targets. Unaffected packages MUST remain queryable.

#### Scenario: Unfinished function
- **WHEN** a user leaves a function header or expression incomplete
- **THEN** syntax diagnostics and recoverable document symbols remain available, and unavailable semantic requests return empty or null results without terminating the server
