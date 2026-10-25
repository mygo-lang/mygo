# lsp-analysis Specification

## Purpose

Define consistent MyGO workspace analysis for editor features, so unsaved changes, package dependencies and incomplete source text produce current and accurately located results.

## Requirements

### Requirement: Package analysis with document overlays
The server SHALL discover MyGO source packages in initialized workspace folders, include unopened same-package files and imported workspace MyGO packages, and resolve imports and Prelude consistently with the compiler. Open buffers MUST override disk text. Generated Go files MUST NOT be indexed as MyGO source. External Go packages SHALL supply available type metadata; external Go source navigation is outside this capability.

#### Scenario: Unopened sibling definition
- **WHEN** an open file uses a declaration from an unopened file in the same package
- **THEN** analysis exposes that declaration and its source location

#### Scenario: Unsaved imported package change
- **WHEN** an open imported workspace file changes a declaration signature without saving
- **THEN** subsequent requests in dependent packages reflect the new signature

### Requirement: Current analysis results
The server SHALL invalidate affected package and dependent analysis when overlays or relevant disk files change, including file creation and deletion. Results and diagnostics MUST correspond to the current document version; obsolete analysis MUST NOT overwrite newer results. Package analysis identity MUST be order-independent: the same set of source paths and file stamps MUST produce the same identity regardless of discovery order or map iteration order.

#### Scenario: Disk change in an unopened dependency
- **WHEN** an unopened dependency changes on disk before the next request
- **THEN** that request uses refreshed analysis, whether the client supplied a file notification or the server detected the change itself

#### Scenario: Repeated request for an unchanged package
- **WHEN** several diagnostics publications target documents of the same package whose source set and stamps have not changed
- **THEN** at most one package analysis runs for that package and the remaining publications reuse its result

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

### Requirement: Incremental analysis reuse
The server SHALL reuse previously computed parse results for files whose stamp is unchanged, including the Prelude, imported workspace packages, and unchanged sibling files of the edited package. A file's stamp SHALL be derived from the overlay version when the file is open and from disk size and modification time otherwise. Reuse MUST NOT change observable results: diagnostics, symbols, hover, completion, definitions, and references produced from reused results MUST equal those produced by a full recomputation for the same inputs.

#### Scenario: Edit does not reparse unchanged dependencies
- **WHEN** a document in a package is edited while imported workspace packages and the Prelude remain unchanged
- **THEN** those unchanged packages and the Prelude are not parsed again, and the additional work per edit does not grow with their size

#### Scenario: Edit does not scale with open document count
- **WHEN** several documents of the same package are open and one document changes
- **THEN** the resulting analysis work is one package analysis for that version, not one per open document

#### Scenario: Changed document parsed once per version
- **WHEN** an open document changes and both syntax diagnostics and package analysis are produced for the new version
- **THEN** that document is parsed once and the package analysis reuses the resulting syntax or AST

#### Scenario: Reused results equal a cold analysis
- **WHEN** a package is analyzed, one of its files changes, and it is analyzed again
- **THEN** diagnostics and language-feature results match those of a fresh process analyzing the same current sources

#### Scenario: Overlay close returns to disk content
- **WHEN** an open document with unsaved changes is closed
- **THEN** subsequent analysis uses the on-disk content and does not serve the closed overlay's parse result
