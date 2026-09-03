## 1. Old Compiler: Exhaustive Check Implementation

- [x] 1.1 In `validateSwitch` of `internal/mygo/compiler/validate.go`, use the existing `enumDecl` information to calculate missing variants, implement the `collectMissingVariants` helper function, and verify that `switch` covers all enum variants (or has wildcard `_` / raw binding); add `TestValidateSwitchMissingVariant` to test coverage of missing variants and report `non-exhaustive switch: missing variant(s) Blue`.
- [x] 1.2 Add exhaustive nested pattern checks to the old compiler: recursively process `TuplePattern`, extract subtypes from the tuple type of `targetType` and repeat the check for each subpattern; add the `TestValidateSwitchNestedTupleMissingVariant` test to cover scenarios where `a` in `switch (a, b)` is an enumeration but not all variants are covered.
- [x] 1.3 Apply the same checks to `switch` statements (non-tailed expressions) to ensure that they are not skipped due to `expected == ""`; add tests to cover non-exhaustive errors in statement statements.

## 2. Old Compiler: Testing and Existing Migration

- [x] 2.1 Scan the old tests and examples under `internal/mygo/`, add missing cases or explicit `_` for `switch` statements that do not meet exhaustiveness, and run `go test ./internal/mygo/compiler/... ./internal/mygo/codegen/...` to verify that it passes.
- [x] 2.2 Add `internal/mygo/compiler/switch_exhaustiveness_test.go`, covering: all variants pass, missing variants report errors, `_` is present pass, nested tuples report errors, `_` escapes within tuples pass, non-enumerations (Int/String) are not checked, and run the test file to verify that it passes.

## 3. Bootstrap Compiler: Exhaustive Check Implementation

- [x] 3.1 Implement `checkSwitchExhaustive(targetType, cases, state)` in `internal/mygo/typeinference2/infer.mygo`, reuse `lookupEnumVariantDecl` / `enumNameForMonoType` to get enumeration variants, recursively process `TuplePattern`, call and return `Result` error at the entry point of `inferTypedSwitchCases`; regenerate `zz_*.gen.go` using `go generate ./...`.
- [x] 3.2 Add test cases from `internal/mygo/codegen2/codegen2_test.go` to the bootstrap compiler: complete variant coverage succeeds, missing variants return `non-exhaustive switch`, underscores pass, statement-based non-exhaustive errors occur, nested tuple recursion is checked, and run `go test ./internal/mygo/codegen2/...` to verify.
- [x] 3.3 Integrate the bootstrap compiler path into the existing type inference, ensuring that public APIs such as `GenerateSource` correctly return compilation errors instead of panics; run `go test ./internal/mygo/typeinference2/... ./internal/mygo/codegen2/...` to verify.

## 4. Bootstrap Compiler: Bootstrap Verification and Migration

- [x] 4.1 Compile the current codebase using `go run ./cmd/mygo --bootstrap ...`, fix all `switch` statements in the bootstrap source code (`.mygo`) that are reported as non-exhaustive due to new checks, and add case statements or explicit `_` statements.
- [x] 4.2 Rebuild all affected `zz_*.gen.go` files and ensure that `go build ./...` and `go test ./...` pass (at least run tests for the codegen2, compiler, and typeinference2 packages).

## 5. Documentation and Finishing Up

- [x] 5.1 Add exhaustive rule descriptions to the Pattern Matching section of `docs/compiler/semantics.md` (enumerations must cover variants or provide wildcards, nested behavior, and statement forms also apply), and verify that the generated documentation links are accessible.
- [x] 5.2 Run `openspec validate` and `openspec validate --strict` (if the environment supports it) to ensure that all proposals/specs/designs/tasks of the change pass the validation.
