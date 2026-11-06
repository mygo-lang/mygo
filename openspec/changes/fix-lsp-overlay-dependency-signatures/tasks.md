## 1. Dependency Source Selection

- [x] 1.1 In internal/mygo/compiler/bootstrap.mygo, change compileUncachedBootstrapDir to load package inputs through bootstrapLoadCachedSourcesDiagnostic(dir, state); verify the source-selection diff is limited to that call and the disk-only loader remains available for callers without a populated source snapshot.
- [x] 1.2 Rebuild the bootstrap CLI with GOCACHE=/tmp/gocache-lsp-overlay go build -buildvcs=false -o /tmp/mygo-lsp-overlay ./cmd/mygo, then run /tmp/mygo-lsp-overlay --bootstrap sync internal/mygo/compiler; verify internal/mygo/compiler/zz_bootstrap.gen.go calls bootstrapLoadCachedSourcesDiagnostic inside compileUncachedBootstrapDir.

## 2. Focused Regressions

- [x] 2.1 Run GOCACHE=/tmp/gocache-lsp-overlay go test ./lsp -run 'TestDependencyEditInvalidatesDependent|TestImportedAliasNavigationAndOverlay|TestRealProtocolSemanticSession' -count=1 -v; verify all three tests pass.
- [x] 2.2 Run GOCACHE=/tmp/gocache-lsp-overlay go test ./lsp -count=1 -timeout 180s; verify the complete LSP package passes.
- [x] 2.3 Run GOCACHE=/tmp/gocache-lsp-overlay go test ./internal/mygo/compiler -run 'TestExportedPackageBindingsCodegen2|TestExportedPackageBindingsCodegen3|TestImportedLetAssignmentIsRejected|TestUnannotatedBindingKeepsDeclarationOrderBoundary' -count=1; verify the inferred exported binding behavior still passes.

## 3. Broader Validation

- [x] 3.1 Run GOCACHE=/tmp/gocache-lsp-overlay go test ./internal/mygo/compiler -count=1 -timeout 600s; verify the compiler package has no new failures.
- [x] 3.2 Run GOCACHE=/tmp/gocache-lsp-overlay go test ./... -count=1 -timeout 600s; verify the repository suite passes, or record each remaining failure as a pre-existing baseline failure with its package and test name.
- [x] 3.3 Review git diff --stat and git diff to confirm the implementation is limited to the compiler source load, generated zz_bootstrap.gen.go, and the OpenSpec change; verify there are no test-only workarounds or unrelated generated files.
- [x] 3.4 Run openspec validate "fix-lsp-overlay-dependency-signatures" --strict; verify the change artifacts validate without errors.
