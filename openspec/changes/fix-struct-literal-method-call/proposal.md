## Why

A struct literal used directly as a method receiver (for example
`Point { x: 1, y: 2 }.Sum()`) is not parsed as one postfix chain. The parser
freezes the struct literal and stops, so the trailing `.Sum()` is either
reported as an unrelated error at argument positions (`unknown identifier .`)
or silently dropped on an assignment right-hand side, producing a program that
compiles but never calls the method.

## What Changes

- Parser2 continues postfix parsing after a frozen struct literal, so
  `TypeName { ... }.method(...)`, `TypeName { ... }.field`, and the generic form
  `TypeName[T] { ... }.method(...)` parse as a single expression whose receiver
  is the struct literal.
- The trailing postfix is no longer dropped or reported as an unrelated
  identifier error.
- Regression coverage is added for method calls, field selection, and generic
  struct-literal receivers.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `parser2-lossless-cst`: a struct literal used as the head of a postfix chain
  must keep that chain as part of the same expression instead of terminating the
  expression at the literal.

## Impact

- `internal/mygo/parser2/syntax_parse_expressions.mygo` (postfix chain
  continuation) and its regenerated `zz_*.gen.go` output.
- Type inference and code generation are unchanged: they already accept a
  parenthesized struct literal as a receiver, since
  `(Point { x: 1, y: 2 }).Sum()` compiles correctly today.
- Out of scope: the legacy `--legacy` parser, which reports a syntax error for
  this form and is not the default pipeline.
