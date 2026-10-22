## 1. Failure inventory and diagnostic foundation

- [x] 1.1 Deliver a failure-exit coverage inventory for parser, AST validation, inference, solver, imports/FFI, generation, writes and Go execution; verify every exit is classified with a provenance provider and regression case.
- [x] 1.2 Add shared structured diagnostic types, stable stage/code constructors and primary/related/context enrichment; verify punctuation independence, deduplication and preservation of the most specific primary span in unit tests.
- [x] 1.3 Add one-time rendering and public compatibility adapters; verify existing Go compilation signatures compile and library/CLI output has one primary prefix and preserves detailed causes.

## 2. Source provenance and parsing

- [x] 2.1 Preserve declaration positions and component spans in package inputs and typed declaration indexes; verify multiple files, ordinary tests, external tests and prelude sources keep distinct original identities.
- [x] 2.2 Establish one-based UTF-8 byte column handling and structured parser errors; verify Chinese text, tabs, CRLF, empty input, EOF and explicitly named source-string compilation with exact coordinates.
- [x] 2.3 Preserve source origins in expression reconstruction, desugaring and synthetic nodes; verify transformed and tail-call constructs retain their source spans and support-only nodes are explicitly classified.

## 3. Inference, declarations and constraints

- [x] 3.1 Migrate declaration checks and conflicts to typed diagnostics with component and related declaration locations; verify duplicate declarations, invalid annotations and cross-file conflicts through package-level inference.
- [x] 3.2 Migrate expression inference and unification propagation, replacing colon heuristics and body-position prefixes; verify nested function errors and colon-bearing messages point to the offending expression and retain expected/actual types.
- [x] 3.3 Carry deferred constraint and instantiation origins through schemes and solver operations; verify two uses of the same constrained function retain their own use sites and delayed failures identify the failing use.

## 4. Bootstrap orchestration and imports

- [x] 4.1 Carry import declaration spans through dependency traversal and Go FFI loading; verify missing imports, cycles, FFI failures and dependency expression errors preserve primary and importing locations.
- [x] 4.2 Migrate bootstrap orchestration to typed diagnostic paths without intermediate formatting; verify ordinary and external-test compilation plus named source-string entry points produce consistent codes and locations.
- [x] 4.3 Classify filesystem, tool, internal invariant and generated-support failures; verify injected read/write/start failures retain causes and targets, state location unavailability when appropriate, and never fabricate coordinates.

## 5. Code generation and line directives

- [x] 5.1 Migrate generation failures and Go AST rendering errors to diagnostics carrying node origins; verify unresolved foreign types and source-backed render failures identify the original construct.
- [x] 5.2 Track final rendered Go lines and emit `//line` or inline `/*line*/` directives that preserve the originating MyGO file and line; verify formatting, multiline expressions, inline Go, tail calls and support code.
- [x] 5.3 Integrate line-directive generation into output writing and regeneration without sidecar files; verify generated support attribution, deterministic output and cleanup of obsolete generated artifacts.
- [x] 5.4 Verify final Go build diagnostics use directives to preserve file, line, tool explanations, build flags, exit behavior and tool-start failures; allow column approximation only for compiler-emitted code while keeping unmapped dependency output unchanged.

## 6. Integrated acceptance and self-hosting

- [x] 6.1 Add a table-driven coverage suite spanning every inventory category and spec scenario; verify exact pre-build file/line/column, final-build line accuracy and permitted column approximation, primary-versus-related positions, deterministic ordering and absence of placeholder locations for source errors.
- [x] 6.2 Bootstrap-sync changed compiler packages individually in dependency order and build a fresh CLI using a task-local writable GOCACHE; verify no generated Go was hand-edited and the rebuilt compiler passes focused package tests with line directives.
- [x] 6.3 Run relevant full Go tests and bounded compiler-package/bootstrap smoke builds with timeout and ulimit; record commands, results, resource observations and any external validation limitations.
- [x] 6.4 Reconcile the failure inventory with implemented diagnostics and document coordinate precision by stage and line-directive lifecycle; verify no unclassified source error exits remain and run `openspec validate improve-bootstrap-diagnostics --strict`.
