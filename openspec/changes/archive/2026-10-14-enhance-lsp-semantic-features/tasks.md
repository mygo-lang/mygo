## 1. File Splitting and Protocol Basics

- [x] 1.1 Split the existing server.mygo file according to the design responsibilities, retaining the CLI entry point; confirm the usability of the split package through generation and build, and check that the handwritten file targets 400 lines and has a maximum of 500 lines.
- [x] 1.2 Implement persistent stream reader, precise content-length reading, and stderr logging; verify that messages are not lost or corrupted using continuous frames, segmented input, Unicode text, and text without trailing newlines.
- [x] 1.3 Implement explicit JSON-RPC encoding/decoding and parameter validation; validate integer and string IDs, option boundaries, result: null, mutually exclusive error fields, invalid JSON, and unknown request methods.
- [x] 1.4 Implement request and notification dispatch and initialize the /shutdown/exit state; use protocol sessions to verify that there is no response to notifications, requests are rejected before initialization, and exit codes are correct.

## 2. Documentation and Source Location

- [x] 2.1 Implement full synchronization and versioning overlay for standard didOpen/didChange/didClose; verify standard textDocument.text, empty text, ignoring old versions, and disabling rollback disk.
- [x] 2.2 Independently implement URI to UTF-16/source location conversion and verify compiler column units; verify indentation, CRLF, Chinese characters, supplementary flat characters, whitespace paths, and illegal locations.

## 3. Workspace and Package Analysis

- [x] 3.1 Implement workspaceFolders/rootUri and single-file directory package discovery; use multi-file fixtures to verify that unopened files in the same package are included and that generated Go does not enter the source code index.
- [x] 3.2 Integrate read-only imports, Prelude, Go FFI metadata, and package inference; verify import aliases, workspace MyGO dependencies, and compiler consistency, and confirm that analysis does not generate any project files.
- [x] 3.3 Establish package dependency graphs, overlay priority snapshots, and content fingerprint caching; verify that caller type information is not saved when dependency changes are updated.
- [x] 3.4 Implement file creation, deletion, and modification notifications, and disk refresh when no notification is received; verify that disk changes to unopened dependencies will update the results of the next request, and that independent packages remain queryable.

## 4. Syntax and Semantic Indexing

- [x] 4.1 Integrate recovery CST, strict lowering, and syntax diagnostic mappings; verify that incomplete source code retains diagnostics and recoverable declarations without entering invalid package inference.

- [x] 4.2 Establish scope indexes for declarations, parameters, let/var, pattern bindings, and type parameters; verify the correct symbol identity and precise location for each usage association using masked fixtures.

- [x] 4.3 Build type, member, and cross-file usage indexes from the type tree and package metadata; verify that field, method, enum variant, and import alias resolution does not rely on name substring searches.

- [x] 4.4 Adapt to types and import error sources, adding a minimum compatibility diagnostic interface if necessary; verify that errors fall within the source file and that the behavior of the old string compiler interface remains unchanged.

- [x] 4.5 Implement open document diagnostic release, dependency refresh, and close cleanup; verify that diagnostics disappear after error correction, comment strings do not generate false reports, and old snapshots do not overwrite the current version.

## 5. Completion and Hover

- [x] 5.1 Implement scope completion, Prelude, and package symbol candidates; verify parameter and local variable visibility, occlusion, and deduplication.

- [x] 5.2 Implement type annotation completion, generic parameters, and imported type details; verify the accuracy of type candidates after the colon in parameters and their signatures.

- [x] 5.3 Implement receiver member, package member, enum variant, and Go FFI member completion; verify dotted access, including incomplete dotted access, exclude irrelevant members, and degrade when unresolved.

- [x] 5.4 Implement semantic hover and current built-in document; validate inferred types, function signatures, precise token ranges, markup formats, and null for unknown symbols.

## 6. Cross-file navigation and symbols

- [x] 6.1 Implement scope and symbol identity-driven definitions; validate source code jumps for partial masking, similar names, unopened files in the same package, and aliased workspace imports.

- [x] 6.2 Implement cross-file references and includeDeclaration; validate that multiple uses of each line return all results, excluding strings, comments, partial names, and irrelevant masking declarations.

- [x] 6.3 Implement hierarchical documentSymbol; validate the full scope and selection scope of struct fields, enum variants, interface, and impl methods.

- [x] 6.4 Implement case-insensitive substring lookups for workspace/symbol; validate matching for unopened source code, canonical URIs, and empty lists with no matches.

## 7. Integration Acceptance and Delivery

- [x] 7.1 Use real stdin/stdout sessions to perform initialization, opening, diagnosis, completion, hovering, jumping, referencing, modification, closing, and exiting; cover incomplete input and cross-package overlay updates, and confirm that the current version results are consistent.

- [x] 7.2 Refresh the corresponding generated Go, build cmd/mygo-lsp, and run the relevant directed regressions; if the compiler adaptation interface is modified, simultaneously verify the affected bootstrap and CLI paths.

- [x] 7.3 Update the functional scope and workspace description in the README, and review whether the capability declarations are consistent with the actual implementation; deliver the line count check results for all handwritten files not exceeding 500 lines, and continue to split according to responsibility if necessary.