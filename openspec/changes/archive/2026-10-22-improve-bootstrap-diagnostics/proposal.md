## Why

Bootstrap errors use inconsistent string wrapping across parsing, inference, imports, and generation, losing source locations or pointing first to a function body instead of the failing construct. A pipeline-wide provenance contract is needed so pipeline failures retain exact source coordinates and final compiler-emitted failures retain their originating MyGO file and line.

## What Changes

- Introduce structured diagnostics with consistent stages, error codes, primary locations, related locations, and context, rendered at public API and CLI boundaries.
- Preserve provenance through declarations, expressions, constraints, imports, FFI, synthetic nodes, and package boundaries, retaining the most specific primary location.
- Cover generated Go rendering failures with exact structured locations and final Go build failures with generated-code `//line` directives.
- Require exact file, line and UTF-8 byte column coordinates before final Go compilation, while allowing final build column precision to follow generated formatting for compiler-emitted code.
- Preserve operation targets and causes for operational failures, attaching known source triggers as related locations and explicitly identifying unavailable source locations without fabricating coordinates.
- Add a complete diagnostic coverage matrix and end-to-end location regression checks while preserving existing Go API signatures and failure exit behavior.

## Capabilities

### New Capabilities

- `bootstrap-diagnostics`: A pipeline-wide contract for structured bootstrap diagnostics, source provenance, generated-code line-directive mappings, and operational failures without source locations.

### Modified Capabilities

None. Existing language-feature requirements for located diagnostics remain in force; this change adds a unified contract across stages.

## Impact

- Affects `ast2`, `parser2`, `common2`, `typeinference2`, `codegen2/goast`, bootstrap orchestration, and `cmd/mygo`.
- Internal diagnostic interfaces and generated artifacts require a consistent migration and bootstrap regeneration in dependency order; generated Go must not be edited manually.
- Diagnostic text layout changes require updates to tests that assert complete messages; public Go compilation entry-point return types remain compatible.
- No persistent source-map sidecars, new JSON format, language semantics changes, legacy pipeline migration, LSP protocol redesign, or new external service dependencies.
