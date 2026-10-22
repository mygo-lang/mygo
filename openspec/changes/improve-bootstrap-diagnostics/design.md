## Context

See proposal.md for motivation. `ast2` already contains SourcePos, SourceSpan, File.DeclPositions and component spans. Parser2 formats syntax locations, while inference and lowering commonly return String errors. `common2.ErrorAtExpr` tests for any colon before adding a position; WithExpressionSourceName only replaces `<input>`. Function wrappers add body positions. Predicate and BootstrapImport currently omit provenance. Go rendering returns text errors and the CLI forwards Go build stderr.

This is a cross-module diagnostic model change, so a design is required. Existing parser and language-specific located-diagnostic requirements remain in force.

## Goals / Non-Goals

**Goals:** carry machine-readable provenance until presentation; retain specific origins; cover every failure exit through a classified coverage inventory; make self-hosting migration reproducible.

**Non-Goals:** language semantic changes, collecting every possible error in one run, a new JSON CLI mode, legacy migration, LSP protocol redesign, or localization of all existing error explanations.

## Decisions

### 1. Shared diagnostic value with explicit location availability

Define diagnostics in a leaf layer usable by parser, inference, generation and orchestration, using ast2 source spans without importing compiler consumers. The value includes stable code, stage, message, optional primary span, related labeled spans, context frames, and an explicit category for source, operation, internal, or generated-support failure. An unlocated source diagnostic is a provenance defect, not an opportunity to choose an arbitrary file.

Use constructors and enrichment helpers that preserve an existing primary and append deduplicated context. Remove colon detection and filename substitution as provenance mechanisms. Alternatives considered: regular expressions over formatted messages remain ambiguous and cannot retain related origins; adding positions only in CLI has already lost the information needed.

### 2. Internal typed paths with compatibility adapters

Migrate actual bootstrap call paths to Result values carrying diagnostics. Keep public string-returning parser/inference/codegen APIs where consumers need compatibility by implementing thin rendering adapters over the typed core. Preserve public Go compiler signatures by wrapping the diagnostic in an error implementation at the Go boundary. Audit all callers so bootstrap never routes through a rendering adapter and then attempts to recover structured data from text.

Rendering happens once: `file:line:column: error [code]: message`, then related notes and stage/function/import context. Category and code remain separate from existing detailed explanations. Keep expected/actual types and unification causes intact. No new public JSON format is required.

### 3. Carry provenance with semantic work

Pair each declaration source with its declaration positions when building PkgDeclSource and typed declaration indexes. Carry parameter, field, return annotation and variant spans to their checks. Attach originating use spans to deferred constraints; preserve separate origins for distinct uses when schemes instantiate rather than reusing a definition position for all calls. Record original declarations for duplicate/conflict notes.

Retain source spans in typed expression reconstruction, desugaring, tail-call transformation and synthetic bindings. Synthetic nodes inherit a labeled originating construct; support nodes without a direct equivalent have an explicit generated-support origin. Import records retain the declaration position and import chain, including FFI signature loading errors. Do not overwrite dependency errors with importer coordinates.

### 4. Byte-based CLI columns

Use one-based UTF-8 byte columns, consistent with Go tool diagnostics; tabs count as one source byte, CRLF forms one line break, and EOF uses the end position. Verify lexer positions against this rule and normalize at the source-position boundary if necessary. LSP conversion remains separate. Named in-memory source keeps its supplied identity; unnamed input uses `<input>` deliberately. This avoids mixing display width, Unicode scalar counts, and Go byte columns.

### 5. Persist mappings for emitted Go

Extend the Go AST/rendering boundary to associate source-backed emitted nodes with origin spans. Persist a versioned sidecar map alongside each emitted file, recording generated ranges, original spans and whether an origin is exact or construct-level. Generate it from final rendered output positions so go/format cannot invalidate offsets. Inline Go retains source-block provenance and maps its content precisely where available; injected scaffolding maps to its triggering construct rather than fabricated byte-level precision.

The CLI captures Go diagnostics, maps positions in project-generated files through this metadata, and emits related raw Go locations and causes. Pass through dependency/tool output that is not a mapped generated-file diagnostic, classifying failures without source appropriately. `//line` alone was considered but does not cover rendering errors or all support-code provenance; sidecar mapping is authoritative and avoids changing runtime stack traces. Rendering errors carry current node provenance before any text exists.

Sidecars include a content fingerprint of generated output, use temporary-file/rename writes, and are rejected if stale. A missing or stale map for compiler-produced source is reported as a provenance/internal failure with the raw generated location; it does not silently claim source coverage. Fresh normal builds regenerate valid maps. Regeneration and cleanup include maps for dynamic dispatch, metadata and other support files.

### 6. Audit classification rather than universal fake positions

Inventory Result Err constructors, error conversions, panics, rendering exits, import/FFI loading, source reads, output writes, and Go subprocess handling. Each exit has a diagnostic category, provenance provider and validation case. Internal invariant failures retain nearest known source context but are clearly internal; no-source filesystem/tool errors explicitly state location unavailability. This separates truthful exceptional cases from missing provenance in ordinary source errors.

## Risks / Trade-offs

- Broad self-hosted API migration → introduce shared types first, retain adapters, regenerate each changed compiler package before consumers using a known working compiler.
- Positions lost in expression rebuilding or declaration indexing → assert provenance preservation across transformations and exercise multi-file package inference, external tests and prelude imports.
- Formatting changes invalidate generated ranges → derive maps from final rendering and verify fingerprints; test multiline inline Go and generated scaffolding.
- Mapping adds output artifacts and memory → store compact per-node ranges, avoid retaining complete source strings in diagnostics, and measure compiler-package bootstrap behavior.
- Existing tests assert exact messages → preserve explanation content while updating explicit formatting expectations and verifying primary versus related positions separately.

## Migration Plan

1. Establish failure-exit inventory and focused fixtures before migration.
2. Add shared diagnostic types and adapters, then preserve provenance in AST/package indexes and parsing.
3. Migrate inference, constraints, generation and bootstrap orchestration; update consumers without allowing string round trips in the active pipeline.
4. Add render provenance, persistent maps and Go build translation; verify output cleanup and stale-map behavior.
5. Bootstrap-sync changed packages in dependency order using a known working compiler, build a fresh CLI, and run focused plus full relevant tests with a writable task-local GOCACHE. Bound high-risk runs with timeout and ulimit.
6. Rollback by reverting the coherent source and regenerated artifact set and regenerating output/maps; do not leave mixed diagnostic APIs or stale maps active.
