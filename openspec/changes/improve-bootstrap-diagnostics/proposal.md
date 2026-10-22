## Why

Bootstrap errors use inconsistent string wrapping across parsing, inference, imports, and generation, losing source locations or pointing first to a function body instead of the failing construct. A pipeline-wide provenance contract is needed so every source-related failure can be traced to its real filename, line, and column.

## What Changes

- Introduce structured diagnostics with consistent stages, error codes, primary locations, related locations, and context, rendered at public API and CLI boundaries.
- Preserve provenance through declarations, expressions, constraints, imports, FFI, synthetic nodes, and package boundaries, retaining the most specific primary location.
- Cover generated Go rendering failures and final Go build failures with mappings from generated code to MyGO source.
- Preserve operation targets and causes for operational failures, attaching known source triggers as related locations and explicitly identifying unavailable source locations without fabricating coordinates.
- Add a complete diagnostic coverage matrix and end-to-end location regression checks while preserving existing Go API signatures and failure exit behavior.

## Capabilities

### New Capabilities

- `bootstrap-diagnostics`: A pipeline-wide contract for structured bootstrap diagnostics, source provenance, generated-code mappings, and operational failures without source locations.

### Modified Capabilities

None. Existing language-feature requirements for located diagnostics remain in force; this change adds a unified contract across stages.

## Impact

- Affects `ast2`, `parser2`, `common2`, `typeinference2`, `codegen2/goast`, bootstrap orchestration, and `cmd/mygo`.
- Internal diagnostic interfaces and generated artifacts require a consistent migration and bootstrap regeneration in dependency order; generated Go must not be edited manually.
- Diagnostic text layout changes require updates to tests that assert complete messages; public Go compilation entry-point return types remain compatible.
- No language semantics changes, legacy pipeline migration, LSP protocol redesign, or new external service dependencies.
