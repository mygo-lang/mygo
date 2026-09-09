## 1. Parser implementation

- [ ] 1.1 Update parser2 block-body parsing to recurse through nested `switch`, `if`, and `while` constructs while preserving the enclosing terminator; verify with a focused nested-switch parse test
- [ ] 1.2 Regenerate `internal/mygo/parser2/zz_parser.gen.go` from the updated MyGo source and verify the generated package builds

## 2. Regression coverage

- [ ] 2.1 Add parser2 tests for nested `switch` in `case ... then`, plus nested `if` and `while` boundaries; verify valid input parses successfully
- [ ] 2.2 Add a missing-nested-terminator test and verify the parser reports an appropriate missing `end` error

## 3. Validation

- [ ] 3.1 Run `GOCACHE=/tmp/mygo-gocache go test ./internal/mygo/parser2` and verify all parser2 tests pass
- [ ] 3.2 Run OpenSpec validation for the completed change and verify no artifact or spec errors remain
