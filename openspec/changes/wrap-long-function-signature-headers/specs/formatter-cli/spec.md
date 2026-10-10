# Spec Delta

## MODIFIED Requirements

### Requirement: Deterministic source formatting
The formatter SHALL accept valid MyGO source text and produce deterministic formatted source text without changing program semantics.

#### Scenario: Formatting valid source
- **WHEN** valid `.mygo` source is passed to the formatter
- **THEN** it returns formatted source using one stable layout for the same input

The formatter SHALL prefer lines no wider than 100 columns and SHALL choose a multiline representation when a compact expression would exceed that width.

#### Scenario: Long expression
- **WHEN** a declaration or control-flow expression exceeds the preferred line width
- **THEN** the formatter emits a stable multiline layout rather than an excessively wide single line

#### Scenario: Long function declaration header
- **WHEN** a function declaration or implementation method header, including its name, parameters, and return type, exceeds the preferred line width
- **THEN** the formatter places the opening parenthesis after the function name, each parameter on its own line indented by two spaces, the closing parenthesis at the header indentation, and the return type after the closing parenthesis

#### Scenario: Short function declaration header
- **WHEN** a function declaration or implementation method header fits within the preferred line width
- **THEN** the formatter keeps the header on one line

#### Scenario: Formatting is idempotent
- **WHEN** formatted output is passed to the formatter again
- **THEN** the second output is byte-for-byte equal to the first output
