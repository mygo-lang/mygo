## Context

See proposal.md for motivation. The existing `lsp/server.mygo` centralizes protocols, storage, string scanning, and all processors, exceeding a thousand lines. `readOneMessage` creates a reader each time and reads the text on a newline basis; `Main` outputs startup logs to stdout. The protocol structure is directly embedded in the Option, requiring explicit JSON boundaries.

parser2's `ParseSyntaxAt` preserves the original text, CST, location, and recovery diagnostics; strict AST lowering rejects source code with diagnostics. typeinference2's `PackageInfo` provides environment, members, import information, and typed declarations preserved by source file, but type errors are returned as strings, and there is no readily available complete editor symbol index.

## Goals / Non-Goals

**Goals:**

- Service analysis reuses compiler semantics and import rules to avoid discrepancies between the editor and compilation results.

- All processors read the same version of the analysis snapshot; package analysis is independent of code generation and cannot write to the generation file.

- Limit file responsibilities and length to allow subsequent features to evolve independently.

**Non-Goals:**

- Do not build a second type inferrer; do not implement each request through full compilation or writing temporary source files.

- Do not automatically add imports, do not navigate external Go source code, and do not extend language semantics.

- Use full document synchronization and controlled-order request processing in the first round; do not require incremental CST or parallel analysis.

## Decisions

### 1. Define Protocol Boundaries

Use a persistent reader and precise byte reading; all logs are output to stderr. Internally, the service still uses the MyGO data type. Boundaries are explicitly converted using independent JSON adaptation to optional, null, missing fields, and request IDs, avoiding the assumption that Options automatically have protocol encoding. IDs retain their original integer or string values. Routes distinguish between requests, notifications, and illegal messages; lifecycle states cover uninitialized, running, and shut down; connections are terminated due to unrecoverable transmission errors such as invalid frame length; JSON errors within a complete frame return a protocol error and continue.

An alternative is to directly marshal the current unified message structure. The drawback is that it cannot accurately express the mutual exclusion relationships between notifications, errors, and `result: null` fields.

### 2. Documents and Coordinates as Independent Base Layers

Document snapshots save URIs, paths, versions, source code, and row indices. All locations are centrally converted to source code offsets and UTF-16 columns; after verifying the column counting units of parser2 and ast2, they are converted via adapters; directly subtracting one cannot be used as an LSP column. URIs use standard parsing and escaping. Changes accept strictly incremental versions; full synchronization only replaces the complete text; after closing, overlays are undoed, empty diagnostics are published, and dependencies are invalidated.

An alternative is for each processor to independently slice strings, which easily leads to repeated Unicode, indentation, and URI errors.

### 3. Workspace Package Graph and Read-Only Analysis Snapshots

Packages are discovered from workspaceFolders, rootUri, or a single open file directory; dependencies are located by module root and normalized paths; existing import, Prelude, and Go metadata loading rules of the compiler are reused. The package graph records forward and reverse dependencies. When reading `.mygo`, overlay is prioritized. The cache key includes the source content fingerprint, document version, and dependency snapshot identifier. File notifications are handled, and disk input is checked before analysis requests to ensure updates even without client notifications. Initial correctness is prioritized to avoid rescanning the entire working directory for each request.

Input snapshots are frozen for each request, analysis is performed in dependency order, and code generation is not performed. If existing import loading and generation are coupled, the minimum read-only adapter interface is extracted without changing the compilation CLI behavior. Invalid dependencies render the semantic results of the relevant package unavailable but do not block independent packages. Non-file documents without a working directory support syntax features.

An alternative is to only index open files, which cannot satisfy jumps to unopened files; running command-line compilation has file writing and latency issues.

### 4. CST, Typed AST, and Semantic Index Hierarchy

A recovery CST is always built for each document, providing syntax diagnostics and document notation; only packages without syntax errors undergo strict lowering and inference. The system traverses typed declarations and expressions to establish indices for scope, symbol identity, definition location, usage location, and type information. Parameters, let/var symbols, pattern bindings, type parameters, and members are handled separately. Identity is determined by the package, file, and declaration location; it cannot rely solely on the name. The scope tree handles masking, and package resolution handles import aliases.

Completion gathers candidates from the visible environment, type parameters, and import metadata. Point access is filtered based on receiver type fields, enum variants, and available methods. Incomplete point accesses prioritize resolving the receiver using the restored CST and the current valid declaration context; if the type cannot be determined, it degenerates into a syntax candidate, and types from older snapshots cannot be referenced. Reference queries use the identity index, and working area symbols are searched only for source declarations.

An alternative is to upgrade regular expression search, which still cannot identify masking, type members, and precise usage locations.

### 5. Diagnosis and Failure Handling

Syntax errors are directly mapped to CST diagnostics. Type errors are first adapted to existing error formats carrying location information; paths that cannot be reliably located are adapted with minimal structured diagnostics, while maintaining compatibility with the compiler's existing string error interface. The initial type inference round allows returning the first reliable error, but does not guarantee the collection of all type errors at once. Diagnostics are only published to open documents and carry the applicable version; affected open documents are refreshed to prevent dependency errors from remaining in old locations. Only syntactically valid packages output semantic snapshots; old types are not reused after failures.

### 6. Split MyGO Files by Responsibility

| File | Responsibility |

| --- | --- |

| server.mygo | State, main loop, and lifecycle |

| protocol_types.mygo / protocol_codec.mygo / transport.mygo | Protocol structure, JSON boundaries, stream framing |

| documents.mygo / positions.mygo / uri.mygo | Overlay, coordinates, and URI |

| workspace.mygo / package_loader.mygo / analysis_cache.mygo | Package discovery, loading, dependencies, and caching |

| syntax_analysis.mygo / semantic_analysis.mygo | CST and type analysis adaptation |

| scope_index.mygo / symbol_index.mygo | Scope, symbol identity, and usage index |

| diagnostics.mygo / completion.mygo / hover.mygo | Function handlers |

| definition.mygo / references.mygo / document_symbols.mygo / workspace_symbols.mygo | Navigation and Symbol Processor |

| router.mygo | Parameter Validation and Request Dispatch |

Each handwritten file target is no more than 400 lines, with a hard cap of 500 lines; complex processors are further split by candidate collection, member resolution, or index traversal, without creating a helpers file containing all logic. This constraint applies to newly added or modified handwritten adapters and test files; generated Go files are maintained by the generator and are not subject to line count limits. Specific naming can be adjusted, but responsibility and length constraints must be preserved.

## Risks / Trade-offs

- [Package inference lacks local indexes required by the editor] → Build an index on top of the type tree; add a minimum adapter when source information is missing, do not guess the target.

- [Input is frequently incomplete] → Always retain recovery CST; resolve completion context separately, provide explicit degradation when semantics are unavailable.

- [Slow Go metadata loading and working space scanning] → Cache and reuse import information by package, only refresh affected dependencies.

- [Prelude merges polluted user symbol positions] → Records source files; user document symbols are only output for declarations within the current file.

- [Compiler interface adjustments causing bootstrap differences] → Targeted verification and generated file synchronization; adaptation layers should remain in the LSP as much as possible.

## Migration Plan

1. First, split existing source files and establish the protocol, documentation, and coordinate foundation; then add the working area and analysis layer; finally, connect to each processor.

2. Verify each stage using real stdin/stdout frames and multi-file fixtures; after completion, refresh the corresponding generated Go and build `cmd/mygo-lsp`.

3. Update the README to support range, full synchronization, and working area limitations. During rollback, undo source files and their generated files in groups, maintaining consistent CLI entry points.