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

Use one-based UTF-8 byte columns for every diagnostic produced before final Go compilation, consistent with Go tool diagnostics; tabs count as one source byte, CRLF forms one line break, and EOF uses the end position. Verify lexer positions against this rule and normalize at the source-position boundary if necessary. LSP conversion remains separate. Named in-memory source keeps its supplied identity; unnamed input uses `<input>` deliberately. Final build diagnostics preserve the source file and line through directives, while their columns may follow generated formatting and must not be presented as exact source byte columns.

### 5. Emit line directives for final generated Go

Extend the Go AST/rendering boundary to associate source-backed emitted nodes with origin spans, then emit Go line directives from the final formatted output. Emit a column-one `//line filename:line` before a generated physical line whose tokens share one origin. When one generated line contains tokens from different MyGO lines, emit `/*line filename:line[:column]*/` immediately before the source-backed token whose origin changes. This keeps the file and line accurate; the column may follow the generated token layout and is therefore best-effort only for the final Go compilation stage.

Line directive filenames are normalized relative to the generated file's directory (or represented by the named virtual source identity for input strings) so relative paths resolve to the authored file. Directives are inserted after `go/format` has produced the final layout, so formatting cannot move or invalidate them. Inline Go keeps its source-block provenance. Compiler-created scaffolding emits an explicit directive back to its generated support file rather than claiming a MyGO position.

The Go compiler and `go build` consume these directives directly. The CLI preserves their file/line mapping and the underlying tool explanation, while passing through diagnostics in unmapped dependencies. No `.mygo-map.json` sidecar, fingerprint, stale-map lookup, or map cleanup lifecycle is required: the generated file and its directives are written together and regenerated together. Rendering failures occur before final Go text exists and continue to use structured node provenance with exact source coordinates.

### 6. Audit classification rather than universal fake positions

Inventory Result Err constructors, error conversions, panics, rendering exits, import/FFI loading, source reads, output writes, and Go subprocess handling. Each exit has a diagnostic category, provenance provider and validation case. Internal invariant failures retain nearest known source context but are clearly internal; no-source filesystem/tool errors explicitly state location unavailability. This separates truthful exceptional cases from missing provenance in ordinary source errors.

## Risks / Trade-offs

- Broad self-hosted API migration → introduce shared types first, retain adapters, regenerate each changed compiler package before consumers using a known working compiler.
- Positions lost in expression rebuilding or declaration indexing → assert provenance preservation across transformations and exercise multi-file package inference, external tests and prelude imports.
- Formatting changes invalidate generated ranges → insert line directives only after final formatting; test multiline inline Go and generated scaffolding.
- Line directives can change runtime stack-trace file and line identity → document the generated-code effect and keep support code explicitly attributed to its generated file.
- Per-line directives add generated source lines and inline directives add comment text → verify Go parsing, formatting stability, build diagnostics and output size on compiler packages.
- Existing tests assert exact messages → preserve explanation content while updating explicit formatting expectations and verifying primary versus related positions separately.

## Migration Plan

1. Establish failure-exit inventory and focused fixtures before migration.
2. Add shared diagnostic types and adapters, then preserve provenance in AST/package indexes and parsing.
3. Migrate inference, constraints, generation and bootstrap orchestration; update consumers without allowing string round trips in the active pipeline.
4. Add render provenance and line directives; verify final Go build locations, support attribution and output cleanup without sidecar maps.
5. Bootstrap-sync changed packages in dependency order using a known working compiler, build a fresh CLI, and run focused plus full relevant tests with a writable task-local GOCACHE. Bound high-risk runs with timeout and ulimit.
6. Rollback by reverting the coherent source and regenerated artifact set and regenerating output/directives; do not leave mixed diagnostic APIs or directives active.
