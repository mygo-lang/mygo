## 1. Regression Coverage

- [ ] 1.1 Add a compiler fixture where package dep declares a Go import of go/token and package app imports dep while using a local binding named token; verify generated app files contain no go/token import
- [ ] 1.2 Add a fixture where app itself imports go/token and emits a qualified reference; verify only the file that uses it receives the import with the declared alias
- [ ] 1.3 Add a fixture where a dependency API returns a type from a third-party Go package; verify the consumer file still receives that inferred Go import

## 2. Import Candidate Separation

- [ ] 2.1 Change generateOneFile to derive inferred Go import candidates from qualified Go type paths referenced by the file's typed declarations; verify the new fixture from 1.1 passes while existing inferred-import tests stay green
- [ ] 2.2 Ensure dependency Go packages remain available for signature resolution and alias selection; verify bootstrap sync of internal/mygo/codegen2 still succeeds
- [ ] 2.3 Confirm deterministic alias selection when explicit and referenced imports compete; verify by running the existing alias-conflict tests

## 3. Binding-Aware Usage Detection

- [ ] 3.1 Record the import alias at each generator site that emits a package-qualified reference; verify generated output still compiles for FFI selectors, foreign types, and prelude helpers
- [ ] 3.2 Filter emitted imports by the recorded alias set instead of the textual selector scan; verify the local-binding fixture from 1.1 no longer keeps an unused import
- [ ] 3.3 Retain a rendered-source consistency check so a missed recording site fails generation with a diagnostic; verify by temporarily forcing a mismatch in a test fixture

## 4. Repository Regeneration

- [ ] 4.1 Sync the lsp package and verify no lsp/zz_*.gen.go file imports go/token
- [ ] 4.2 Run go build ./... and verify it passes after regeneration
- [ ] 4.3 Re-run bootstrap sync for affected packages in dependency order and verify generated metadata files are stable

## 5. Validation

- [ ] 5.1 Run go test ./internal/mygo/codegen2 ./internal/mygo/ir3 ./internal/mygo/compiler with focused import and metadata tests and verify they pass
- [ ] 5.2 Run the codegen2 self-host build and verify it succeeds
- [ ] 5.3 Run the codegen3 self-host build (--codegen3 --opt-level 2) and verify it succeeds
