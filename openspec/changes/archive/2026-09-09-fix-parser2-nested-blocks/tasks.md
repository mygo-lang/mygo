## 1. Parser implementation

- [x] 1.1 Add a committed `\u` escape branch to parser2 ordinary-string parsing;
  consume exactly four hexadecimal digits, decode one Unicode scalar rune, and
  reject surrogate values
- [x] 1.2 Regenerate `internal/mygo/parser2/zz_parser.gen.go` from the updated
  MyGo source and verify the generated package builds

## 2. Regression coverage

- [x] 2.1 Retain parser2 tests for nested `switch` in `case ... then`, plus
  nested `if` and `while` boundaries, as independent block-boundary coverage
- [x] 2.2 Retain the missing-nested-terminator test as independent invalid-block
  coverage
- [x] 2.3 Add focused parser2 tests for valid ASCII and non-ASCII `\uXXXX`
  escapes, including the complete former `stripReceiverArg` nesting shape
- [x] 2.4 Add focused parser2 tests for incomplete, non-hexadecimal, and
  surrogate `\uXXXX` escapes, plus compatibility coverage for `\0`, raw, and
  multiline strings

## 3. Validation

- [x] 3.1 Run `GOCACHE=/tmp/mygo-gocache go test ./internal/mygo/parser2` and
  verify all parser2 tests pass after the Unicode change
- [x] 3.2 Run `GOCACHE=/tmp/mygo-gocache go run ./cmd/mygo --bootstrap sync
  internal/mygo/typeinference2` and then compile/test the generated package
- [x] 3.3 Run OpenSpec validation for the updated change and verify no artifact
  or spec errors remain
