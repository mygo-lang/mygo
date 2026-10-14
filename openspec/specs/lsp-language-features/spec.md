# lsp-language-features Specification

## Purpose

Define useful MyGO editor language features backed by source syntax and package semantics, including type aware completion and navigation across source files within the workspace.

## Requirements

### Requirement: Scope and type aware completion
The server SHALL complete keywords, visible local bindings, parameters, package declarations, imported aliases and Prelude symbols. Type annotation positions SHALL offer valid types and type parameters. Member access SHALL offer fields, enum variants and applicable methods from the resolved receiver or package, including available Go FFI metadata. Entries MUST include available type or signature details, respect visibility and shadowing, and exclude unrelated members. Automatic imports are not required.

#### Scenario: Type annotation
- **WHEN** completion is requested after a parameter type colon
- **THEN** candidates include visible primitive, generic, user-defined and imported types with type details

#### Scenario: Receiver members
- **WHEN** completion is requested after a dot on a typed variable or imported package alias
- **THEN** only applicable accessible members are offered with their available signatures

#### Scenario: Shadowed local
- **WHEN** a local binding shadows a package declaration
- **THEN** completion presents the visible binding and does not duplicate the hidden declaration as the same candidate

### Requirement: Semantic hover
The server SHALL return the resolved symbol's type or signature and available documentation with a precise token range. Built-in documentation MUST match current language behavior and the declared markup format. Unresolved identifiers SHALL return null.

#### Scenario: Inferred local type
- **WHEN** hover targets a local whose type is inferred in a valid package
- **THEN** the response displays that inferred type over the identifier's exact range

### Requirement: Cross-file definition navigation
The server SHALL resolve local bindings, parameters, declarations, type names and members by symbol identity and lexical scope. Targets SHALL include unopened same-package files and imported workspace MyGO packages, preserve import aliases, and select exact declaration names. Unresolved targets SHALL return null or an empty list.

#### Scenario: Aliased import
- **WHEN** definition is requested on a member used through an imported workspace package alias
- **THEN** the result points to the matching declaration in that package's original MyGO source file

#### Scenario: Similar names and shadowing
- **WHEN** a local foo shadows another foo and a declaration foobar also exists
- **THEN** navigation from the local use selects its local declaration without matching the other declarations

### Requirement: Symbol identity references
The server SHALL return all occurrences of the resolved symbol in analyzed workspace source, including multiple uses on one line, and honor includeDeclaration. Comments, strings, partial-name matches and unrelated shadowed symbols MUST be excluded.

#### Scenario: References across files
- **WHEN** references are requested for a package symbol used twice on one line and in another file with includeDeclaration false
- **THEN** every actual use is returned exactly once and its declaration is excluded

### Requirement: Source document and workspace symbols
The server SHALL expose hierarchical document symbols for functions, structs, enums, interfaces and impl blocks, including applicable fields, variants and methods with full declaration ranges and exact selection ranges. Workspace symbols SHALL search declarations in opened and unopened workspace source by case-insensitive substring, returning an empty list for no matches.

#### Scenario: Nested declarations
- **WHEN** document symbols are requested for a struct and an impl with methods
- **THEN** the response includes corresponding children and ranges spanning complete declarations

#### Scenario: Workspace search
- **WHEN** a query matches a declaration in an unopened workspace MyGO file
- **THEN** workspace symbols returns its name, kind and original source location

