## Why

The default self-hosted pipeline rejects every expression that uses the pipe
operators. Parser2, the lossless CST, the formatter and codegen2 already model
`|>` and `<|`, but typeinference2 has no case for either operator and fails
with `unknown binary operator`. In addition, `<|` is parsed left-associatively,
so a backward chain such as `inc <| double <| 2` groups as
`(inc <| double) <| 2` and cannot type-check as function application.

## What Changes

- Add pipe typing to typeinference2: `left |> right` applies the function
  produced by `right` to `left`, and `left <| right` applies the function
  produced by `left` to `right`. Both directions report a located type error
  when the function operand does not accept the argument operand.
- Make `<|` right-associative in parser2 and give it lower precedence than the
  left-associative `|>`, so `inc <| double <| 2` lowers as
  `inc <| (double <| 2)` and `value |> f <| g` keeps the intended grouping.
- **BREAKING** (parser2 AST): chained `<|` expressions and mixed `|>` / `<|`
  chains lower with different grouping than before. Simple `f <| x` and
  `x |> f` expressions are unchanged.
- Fix codegen2 pipe translation so each operand is translated with its own
  inferred type instead of the pipe result type, while preserving left-to-right
  evaluation order and emitting a single call.
- Keep the legacy `parser` / `typeinference` / `codegen` pipeline unchanged,
  including its existing nested-parenthesis pipe state behavior.
- Add parser, inference, codegen and end-to-end bootstrap tests for both
  directions, mixed chains, nested parentheses, generic functions and failure
  diagnostics.

## Capabilities

### New Capabilities

- `bootstrap-pipe-operators`: typing, code generation and diagnostics for `|>`
  and `<|` in the self-hosted bootstrap pipeline.

### Modified Capabilities

- `parser2-lossless-cst`: the pipe-continuation requirement currently requires
  unchanged associativity; it must instead specify right-associative `<|` with
  lower precedence than left-associative `|>` while remaining lossless.

## Impact

- `internal/mygo/parser2/syntax_parse_expressions.mygo` and parser2 syntax tests.
- `internal/mygo/typeinference2/arrays.mygo`, `infer.mygo`, `utils.mygo` and the
  regenerated `zz_infer.gen.go`.
- `internal/mygo/codegen2/translate_ast.mygo` and the generated codegen output.
- `internal/mygo/formatter/syntax_binary.mygo` precedence levels used for wrap
  selection.
- Language documentation in `docs/spec.md`, `docs/compiler/semantics.md` and
  `docs/compiler2/differences.md`.

