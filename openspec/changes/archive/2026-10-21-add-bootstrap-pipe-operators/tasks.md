## 1. Parser2 pipe associativity

- [x] 1.1 Split `parsePipeExpr` into a right-associative `<|` level above a left-associative `|>` level in `internal/mygo/parser2/syntax_parse_expressions.mygo`, reusing the existing binary CST range builder; verify the parser2 package test suite still passes.
- [x] 1.2 Add parser2 syntax tests for `f <| g <| x`, `value |> f <| g`, `a <| b |> c`, nested parentheses, operator spans, and multiline `<|` continuations; verify each lowers to the documented grouping with no diagnostics.
- [x] 1.3 Update `syntaxBinaryLevel` in `internal/mygo/formatter/syntax_binary.mygo` so `<|` is level 0, `|>` is level 1, and later operators shift accordingly; verify formatter tests still pass and formatting remains idempotent for mixed pipe chains.

## 2. Typeinference2 pipe typing

- [x] 2.1 Add a pipe branch to `inferBinary` in `internal/mygo/typeinference2/arrays.mygo` and implement forward/backward pipe unification (`right : A -> R` for `|>`, `left : A -> R` for `<|`) in `infer.mygo`, preserving the typed `BinaryExpr` operator and reporting located direction-specific mismatch errors; verify a focused inference test accepts both directions.
- [x] 2.2 Add inference tests for named functions, function-typed parameters, generic functions, left-associative `|>` chains, right-associative `<|` chains, mixed chains, nested parentheses, and non-function/incompatible-argument failures; verify all expected successes and diagnostics.
- [x] 2.3 Regenerate typeinference2 generated Go with `mygo --bootstrap sync internal/mygo/typeinference2`, then compile the generated Go and run the focused plus full typeinference2 tests using a task-local `GOCACHE` and a `ulimit -v`/timeout guard.

## 3. Codegen2 pipe translation

- [x] 3.1 Translate each pipe operand with its own inferred type rather than the pipe result type in `internal/mygo/codegen2/translate_ast.mygo`; verify a case such as `1 |> f` where `f: Int -> String` in a `String` context emits an `Int` argument and a `String` result.
- [x] 3.2 Preserve source-order evaluation for `|>` when the right operand is effectful by hoisting the left operand into a fresh expression temp after its pre-statements; verify generated Go evaluates the left effect before the right effect exactly once.
- [x] 3.3 Add codegen2 and end-to-end bootstrap tests covering `1 |> double |> inc`, `inc <| double <| 2`, `parse <| input`, mixed chains, nested parentheses, function parameters, generic functions and invalid operands; verify generated Go compiles and the focused/full codegen2 tests pass.
- [x] 3.4 Regenerate codegen2 generated Go with `mygo --bootstrap sync internal/mygo/codegen2` and then rebuild the compiler, verifying the regenerated `zz_translate_ast.gen.go` compiles and the affected consumer packages still pass.

## 4. Integration, documentation and validation

- [x] 4.1 Update `docs/spec.md`, `docs/compiler/semantics.md` and `docs/compiler2/differences.md` with pipe associativity, precedence, typing, evaluation order and the intentional legacy divergence; verify the examples match the parser and compiler behavior.
- [x] 4.2 Run the default `mygo sync`/build on a temporary package containing both pipe directions and verify the resulting Go program prints the expected values; confirm no `zz_*.gen.go` file was hand-edited.
- [x] 4.3 Run parser2, formatter, typeinference2 and codegen2 tests plus the repository differential gate under the resource guard, and verify the differential corpus stays green with chained `<|` confined to test string literals.
- [x] 4.4 Run `openspec validate add-bootstrap-pipe-operators --strict` and verify the change has no validation errors.

