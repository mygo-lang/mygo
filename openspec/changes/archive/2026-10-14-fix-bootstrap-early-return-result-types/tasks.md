## 1. Reproducibility and Return Constraints

- [x] 1.1 Add early `Err` minimal reproduction to the bootstrap testing framework, check the generated generics and run `IsErr`/`IsOk`/`UnwrapErr`; verify failures before fixing and record the triggered inference and generation paths. Baseline: `Err[any, string]` and runtime `failure was not recognized`; branch-block ReturnWithStmt used TUnit and inference left its success parameter unconstrained.
- [x] 1.2 Generalize the existing function return context, independently establish or override the return type in the declared function and function literal entry, including new type variables for functions without annotations; verify through closure type isolation and existing array expected inference tests.
- [x] 1.3 Unify explicit return values ​​to the function return context, propagate substitution to the complete typed AST and block inference; verify through concrete/generic `Result`, early `None`, and early Err/tail Ok joint inference tests for functions without annotations.
- [x] 1.4 Diagnose incompatible explicit return types and source locations; verify failure before Go generation using error Err payload and closure return type conflict negative examples.

## 2. Return Generation and Behavior Regression

- [x] 2.1 Reuse existing return generation paths for explicit returns at the end of branch blocks, and check that other return entries consistently use function return contexts; verify that the generated code is compileable and error judgment is correct through early return tests of conditional, switch, loop, and nested blocks.
- [x] 2.2 Complete delta spec scenario regression, including unit/non-unit/generic Result, Option, closure isolation, annotation-free joint inference, and explicit generic compatibility; confirm that the complete instance parameters and running results conform to the specifications.
- [x] 2.3 Run existing array, unit, tuple, Go FFI, and tail call return related tests to confirm that reusing return paths does not change their behavior.

## 3. Synchronization and Integration Verification

- [x] 3.1 Using a task-specific writable GOCACHE, first synchronize typeinference2, then synchronize codegen2 and necessary consumers, and build a new CLI; confirm that the generated files come from the source code and that the relevant packages compile successfully.
- [x] 3.2 Use the updated CLI to verify bootstrap bootstrapping and directed compiler regression; check that the git diff contains only the source code, generated artifacts, tests, and task status required for this change, and execute `openspec validate fix-bootstrap-early-return-result-types --strict`.
