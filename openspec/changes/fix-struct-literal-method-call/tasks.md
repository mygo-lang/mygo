## 1. Parser2 postfix continuation

- [ ] 1.1 In `internal/mygo/parser2/syntax_parse_expressions.mygo`, pass the
  chain start index into `parsePostfixStructLiteral`,
  `parsePostfixStructLiteralBody`, `parsePostfixStructLiteralFreeze`, and
  `parsePostfixStructLiteralNode`, and make a frozen struct literal continue
  with `parsePostfixTail` instead of returning directly. Verify with
  `go test ./internal/mygo/parser2/...`.
- [ ] 1.2 Confirm the head-name guard still prevents a non-struct brace group
  from being treated as a literal and that a suffix after the literal cannot
  re-enter struct-literal freezing (no recursion). Verify by reviewing the
  guard path and running the parser2 tests from 1.1.
- [ ] 1.3 Regenerate the parser2 Go output with the current compiler
  (`./mygo --bootstrap sync internal/mygo/parser2`) and confirm the regenerated
  `zz_syntax_parse_expressions.gen.go` reflects the change.

## 2. Regression coverage

- [ ] 2.1 Add a parser2 syntax test asserting that a struct literal followed by
  a method call parses as one expression spanning the literal and its postfix
  suffix. Verify by running that test.
- [ ] 2.2 Add a compile-level test covering the method call in an argument
  position and on an assignment right-hand side, asserting the generated Go
  invokes the method with the constructed value rather than dropping the call.
  Verify by running that test.
- [ ] 2.3 Add a generic struct literal receiver case
  (`Box[Int] { value: 42 }.Value()`) and a field selection case
  (`Point { x: 1, y: 2 }.x`). Verify by running those tests.

## 3. End-to-end verification

- [ ] 3.1 Rebuild the compiler and re-run the original reproduction to confirm
  both previously failing positions now compile and call the method. Verify by
  compiling a fixture with `fmt.Println(Point { x: 1, y: 2 }.Sum())` and
  `let s = Point { x: 3, y: 4 }.Sum()` and inspecting the generated Go.
- [ ] 3.2 Run the broader parser2 and compiler test suites to confirm no
  regressions. Verify with `go test ./internal/mygo/...`.
