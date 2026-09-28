## 1. Reproduce and locate the boundary loss

- [x] 1.1 Add a formatter regression fixture for the `fieldsForStructInEnvAt` case body, assert the `let` binding and following recursive call render on separate lines, and verify the fixture fails against the current formatter.
- [x] 1.2 Inspect the fixture's parser2 CST and renderer path, identify where the statement boundary is lost, and record the owning node or rendering rule in the implementation change.

## 2. Correct case-body formatting

- [x] 2.1 Fix the generic CST or branch-body rendering behavior at the identified boundary so each statement in a multiline case body renders on its own line; verify the regression fixture passes without function-specific matching.
- [x] 2.2 Verify the formatted fixture reparses and is idempotent, and run the formatter package's existing test suite to check neighboring case layouts.
- [x] 2.3 Regenerate checked-in formatter Go output from the MyGO source and verify generated files contain no unrelated drift.
