## MODIFIED Requirements

### Requirement: Current analysis results
The server SHALL invalidate affected package and dependent analysis when overlays or relevant disk files change, including file creation and deletion. Results and diagnostics MUST correspond to the current document version; obsolete analysis MUST NOT overwrite newer results. Package analysis identity MUST be order-independent: the same set of source paths and file stamps MUST produce the same identity regardless of discovery order or map iteration order.

#### Scenario: Disk change in an unopened dependency
- **WHEN** an unopened dependency changes on disk before the next request
- **THEN** that request uses refreshed analysis, whether the client supplied a file notification or the server detected the change itself

#### Scenario: Repeated request for an unchanged package
- **WHEN** several diagnostics publications target documents of the same package whose source set and stamps have not changed
- **THEN** at most one package analysis runs for that package and the remaining publications reuse its result

## ADDED Requirements

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
