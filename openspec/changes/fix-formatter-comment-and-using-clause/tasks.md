## 1. Separators and Comments in Separated Structures

- [x] 1.1 Differentiate between heading comments, valid content, and trailing comments in the item area: comments are rendered on separate lines and indented by the item; separators are only appended after valid content; only trivia areas do not have separators appended; verified using regression tests of "wrapped lists no longer growing after three consecutive formattings following comments" and "items after comments maintain indentation."

- [x] 1.2 Cover use cases of comments between struct literal field comments and item comments, asserting byte-by-byte output and second-formatted bytes are equal, and intentionally updating existing golden structures that change as a result.

## 2. Rendering with using constraint clauses

- [x] 2.1 In formatter rendering, calculate the header trivia separator based on the previous valid token of the parent, ensuring that `using ToString[Int]` retains spaces; use golden verification with three forms of signatures (func, impl method, interface) and multiple constraints (`using A[Int], Show`).

- [x] 2.2 Assert that after re-parsing the output of a single `FormatSyntaxBlocks`, `Constraints` nodes still exist and `using` keywords are not merged, preventing the recurrence of `usingToString` class corruption.

## 3. Structure Preservation Validation

- [x] 3.1 Extend "AST equivalence before and after formatting (ignore position)" from fixed fixtures to the repository scope: scan all `.mygo` files, printing a list of files that do not meet the requirement instead of silently skipping them; verify using scan output.

- [x] 3.2 After fixing the issue, remove the non-idempotent exceptions for `internal/mygo/typeinference2/env.mygo`, confirming that repository-level idempotent scans no longer have exceptions.

## 4. Integration and Delivery Verification

- [x] 4.1 Synchronize `internal/mygo/formatter` to generate Go in bootstrap order, using a task-specific writable GOCACHE for building; run `go test ./internal/mygo/parser2 ./internal/mygo/formatter`, confirming that existing and new regressions pass.

- [x] 4.2 Run a full `go test ./...`, and perform two formatted byte equality scans and a single AST equivalence scan on all `.mygo` files in the repository, recording and reporting any remaining exceptions.

- [x] 4.3 Update formatting instructions (comment separators and constraint clause rendering), run `openspec validate fix-formatter-comment-and-using-clause --strict` and `git diff --check` to verify.