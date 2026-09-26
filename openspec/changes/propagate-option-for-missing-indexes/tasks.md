## 1. Audit and classify

- [ ] 1.1 Inventory every `UnwrapOr` use in MyGO source under `internal/mygo` and classify whether it fabricates a missing indexed value or expresses an intentional default; verify the inventory accounts for all `.mygo` source hits.
- [ ] 1.2 Map affected indexed and recursive helpers to their callers across packages, grouping the work in dependency order; verify each candidate has an explicit intended `None` handling point.

## 2. Propagate absence through MyGO sources

- [ ] 2.1 Refactor shared AST and collection utility helpers that mask missing indexed values to return `Option[T]`, and update callers to handle `None`; verify affected packages compile.
- [ ] 2.2 Refactor parser, formatter, and compiler helpers that mask missing indexed values and their call chains; verify affected packages compile and valid source behavior is preserved.
- [ ] 2.3 Refactor type inference and code generation helpers that mask missing indexed values and their call chains; verify affected packages compile and valid source behavior is preserved.
- [ ] 2.4 Review all remaining `UnwrapOr` occurrences under `internal/mygo` and retain only intentional defaults; verify each retained candidate is not substituting solely for a missing indexed element.

## 3. Regenerate and validate

- [ ] 3.1 Run the canonical MyGO generation workflow for changed source packages and verify generated Go files are refreshed from their `.mygo` sources.
- [ ] 3.2 Run repository compiler/build checks covering `internal/mygo` and verify generated outputs compile with no regressions in relevant parser, formatter, inference, and codegen behavior.
