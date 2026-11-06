## MODIFIED Requirements

### Requirement: Generated Go imports are selected per output file
Each generated Go file SHALL include imports required by its emitted Go
declarations and expressions, including imported types in signatures and
generated helper references. It SHALL omit imports that are not referenced by
that file's emitted Go. Import selection SHALL preserve explicit aliases and
account for compiler-synthesized imports. An identifier occurrence SHALL count
as a package reference only when it resolves to the imported package; a local
declaration whose name equals a Go import alias SHALL NOT keep an otherwise
unused import in the output.

#### Scenario: Import used only by an erased typeclass signature is omitted
- **WHEN** a source file declares a typeclass signature using a Go type but the typeclass declaration is erased and no emitted Go declaration or expression refers to that package
- **THEN** that generated Go file SHALL omit the package import

#### Scenario: Returned imported value needs no caller import for method use
- **WHEN** one generated file declares a function returning a foreign type and another generated file calls it and invokes a method on the result without otherwise naming that type
- **THEN** the declaration file SHALL import the defining package to express its signature and the caller file SHALL not gain an unused import

#### Scenario: File-local alias is preserved in generated references
- **WHEN** a generated declaration in one source file refers to an imported package through an explicit alias
- **THEN** that generated file SHALL use the corresponding import alias and SHALL not inherit aliases declared only by sibling files

#### Scenario: Local binding does not keep an unused package import
- **WHEN** a generated file contains a local binding named like a Go import alias, and no emitted reference resolves to the imported package
- **THEN** the generated file SHALL omit that package import even though selector text such as token.URI appears in the body

## ADDED Requirements

### Requirement: Dependency-declared Go imports do not seed consumer imports
The Go package set considered when emitting imports for one package SHALL be
limited to Go packages imported by that package's own source files. Go
packages observed in dependency declarations MAY remain available for
dependency signature resolution, but their presence SHALL NOT cause an import
in the consumer's generated files.

#### Scenario: Dependency Go import is absent from the consumer
- **WHEN** package app imports MyGO package dep, dep declares a Go import of go/token, and no app source file declares that import
- **THEN** generated app files SHALL NOT import go/token

#### Scenario: Consumer's own Go import is preserved
- **WHEN** an app source file declares a Go import of go/token and emitted code resolves an identifier to that package
- **THEN** only files whose emitted code uses the package SHALL import go/token, with the declared alias
