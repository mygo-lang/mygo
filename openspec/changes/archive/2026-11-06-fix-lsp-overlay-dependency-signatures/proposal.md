## Why

Unsaved edits to an imported workspace package are parsed by the editor overlay path, but dependency signature collection can still re-read the package from disk. As a result, LSP diagnostics, navigation, completion, hover, and references can use the previous saved signature until the buffer is closed or the file is saved. The existing lsp-analysis contract already promises overlay behavior, and three LSP regression tests fail at HEAD.

## What Changes

- Make on-demand dependency inference consume the current editor source snapshot, including open overlays, instead of unconditionally reloading dependency files from disk.
- Preserve the inferred package-level binding metadata introduced for exported bindings while ensuring that inferred dependency signatures come from the same overlay-aware source set as the importing package.
- Add regression coverage for imported dependency signature changes, stale definition removal, and restoration after overlay close.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- lsp-analysis: Clarify that dependency signature collection and inferred package-level bindings must use the current overlay source snapshot for open imported workspace files, and that closing the overlay must restore disk-derived analysis.

## Impact

- internal/mygo/compiler/bootstrap.mygo: dependency source loading for on-demand inference.
- lsp analysis and semantic tests already covering imported overlays: lsp/parse_cache_test.mygo, lsp/semantic_index_test.mygo, and lsp/session_test.mygo.
- No public LSP API or protocol change; behavior is corrected to the existing lsp-analysis contract.
