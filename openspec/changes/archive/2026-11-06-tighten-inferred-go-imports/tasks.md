## 1. Regression Coverage

- [x] 1.1 Add a compiler fixture where package dep declares a Go import of go/token and package app imports dep while using a local binding named token; verify generated app files contain no go/token import
- [x] 1.2 Add a fixture where app itself imports go/token and emits a qualified reference; verify only the file that uses it receives the import with the declared alias
- [x] 1.3 Add a fixture where a dependency API returns a type from a third-party Go package; verify the consumer file still receives that inferred Go import

## 2. Import Candidate Separation

- [x] 2.1 Change generateOneFile to derive inferred Go import candidates from qualified Go type paths referenced by the file's typed declarations; verify the new fixture from 1.1 passes while existing inferred-import tests stay green
- [x] 2.2 Ensure dependency Go packages remain available for signature resolution and alias selection; verify bootstrap sync of internal/mygo/codegen2 still succeeds
- [x] 2.3 Confirm deterministic alias selection when explicit and referenced imports compete; verify by running the existing alias-conflict tests

## 3. Binding-Aware Usage Detection

- [x] 3.1 Make the generated-AST alias scan track lexical bindings; verify generated output still compiles for FFI selectors, foreign types, and prelude helpers
- [x] 3.2 Keep filtering emitted imports by the binding-aware scan; verify the local-binding fixture from 1.1 no longer keeps an unused import
- [x] 3.3 Add unit tests covering parameters, short declarations, range variables, and mixed shadow/package-use files

## 4. Repository Regeneration

- [x] 4.1 Sync the lsp package and verify no lsp/zz_*.gen.go file imports go/token
- [x] 4.2 Run go build ./... and verify it passes after regeneration
- [x] 4.3 Re-run bootstrap sync for affected packages in dependency order and verify generated metadata files are stable

## 5. Validation

- [x] 5.1 Run go test ./internal/mygo/codegen2 ./internal/mygo/ir3 ./internal/mygo/compiler with focused import and metadata tests and verify they pass
- [x] 5.2 Run the codegen2 self-host build and verify it succeeds
- [x] 5.3 Run the codegen3 self-host build (--codegen3 --opt-level 2) and verify it succeeds
