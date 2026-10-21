## Context

See `proposal.md` for motivation. The bootstrap pipeline is
`parser2 -> ast2 -> typeinference2 -> codegen2`; the legacy pipeline remains a
separate, frozen implementation.

Current parser2 parses both pipe operators in one left-associative level
(`parseBinaryLevel` with `["|>", "<|"]`). The lossless CST and formatter
already retain both operators. Typeinference2 dispatches arithmetic,
comparison and logical operators only, so pipes reach the `unknown binary
operator` fallback. Codegen2 already emits a call for each pipe direction, but
translates both operands with the pipe expression's result type, and `|>` can
evaluate its right operand before its left operand because the right operand
becomes the Go callee expression.

## Goals / Non-Goals

**Goals:**

- Compile both pipe directions through the default CLI with correct typing and
  Go output.
- Make `<|` right-associative and looser than the left-associative `|>` in
  parser2 while keeping the CST lossless.
- Preserve source-order evaluation of pipe operands and type each operand from
  its own inferred type.
- Cover chains, nesting, generic functions and diagnostics with tests.

**Non-Goals:**

- Changing the legacy `parser`, `typeinference` or `codegen` packages, including
  their nested-parenthesis pipe-state behavior.
- Changing `|>` associativity or the precedence of pipes relative to
  arithmetic, comparison and logical operators.
- Reintroducing the legacy codegen behavior that flattens `x |> f(y)` into
  `f(y, x)`.

## Decisions

### Split the pipe precedence into two parser2 levels

Replace the single `parsePipeExpr` level with:

```
parsePipeExpr        -> parseBackwardPipeExpr
parseBackwardPipeExpr: left = parseForwardPipeExpr
                        if next is "<|" then
                          right = parseBackwardPipeExpr   (recursive, right-assoc)
                          build BinaryExpr("<|", left, right)
parseForwardPipeExpr:  left = parseOrExpr
                        repeat while next is "|>":
                          right = parseOrExpr
                          left = BinaryExpr("|>", left, right)
```

This yields the intended grouping:

```
a |> f |> g        =>  (a |> f) |> g
f <| g <| x        =>  f <| (g <| x)
value |> f <| g    =>  (value |> f) <| g
a <| b |> c        =>  a <| (b |> c)
```

Reuse the existing `syntaxBinaryResult` range builder so the operator token and
trivia between operands stay owned exactly once. `parseForwardPipeExpr` can
delegate to the existing `parseBinaryLevel` with only `|>`; the backward level
needs a small right-recursive tail instead of `parseBinaryLevel`, which is
left-associative by construction.

Alternative considered: keep one level and make only `<|` right-recursive. That
cannot express `a |> b <| c` and `a <| b |> c` consistently because both
operators would share one associativity rule.

### Type pipes by unifying the function operand with `T -> R`

Add a pipe branch before `inferBinaryOrdinary` in typeinference2:

- `left |> right`: infer left, then right; unify the right type with
  `TFunc([leftType], freshResult)`; the expression type is `freshResult`.
- `left <| right`: infer both; unify the left type with
  `TFunc([rightType], freshResult)`.

Use the existing substitution composition and `withFreshID` conventions, and
emit the typed AST through `mkTypedBinary` with the original operator. Keeping
the `BinaryExpr` node lets codegen2 retain its existing pipe branches.

Alternative considered: desugar pipes to `CallExpr` during parser2 lowering.
That would bypass the inference gap but discard the operator from the typed AST,
change formatter/lowering parity expectations, and leave codegen2's pipe
branches unreachable.

### A pipe operand is a function-valued expression

`x |> f(y)` and `f(y) <| x` are typed as applying the function value produced
by the call, not as appending `x` to the call's arguments. This matches the
language contract's function-application wording and codegen2's call shape. The
legacy codegen flattening is not a working contract because legacy inference
rejects the same arity mismatch; it stays untouched.

### Translate pipe operands with their own types

Codegen2 currently passes the pipe result type to both operands. For pipe nodes,
translate each operand through `translateExprAst`, which falls back to the
operand's inferred type, instead of `translateExprAstExpected(..., expected)`.
Arithmetic and comparison keep their existing contextual typing.

### Preserve source-order evaluation for `|>`

`<|` naturally evaluates its callee (left) before its argument (right). `|>`
becomes `right(left)` in Go, where Go evaluates the callee before the argument.
When the right operand is an identifier or a pure field-selector chain, reading
it has no observable effect and the direct call is kept. Otherwise, hoist the
left operand into a fresh `ctxFreshExprTemp` variable before translating the
right operand's pre-statements, then pass that variable to the call. Append the
temporary declaration/assignment after the left operand's own pre-statements so
no earlier side effect is reordered.

### Keep the formatter precedence map aligned

`syntaxBinaryLevel` assigns both pipes level 0. Change it to `<|` = 0, `|>` = 1
and shift the remaining levels by one. Wrapping still only inserts breaks after
existing operators, so this changes preferred break boundaries without changing
token order or grouping.

### Keep legacy differential tests off chained `<|` source

The parser2 differential gate uses the frozen legacy parser as an oracle over
every `.mygo` file in the repository. New parser2 associativity tests should
embed chained `<|` programs in string literals (as existing parser tests do),
so the corpus still contains only source both parsers group identically. If a
future executable fixture needs chained `<|`, the gate needs an explicit,
documented exemption rather than silently weakening the comparison.

## Risks / Trade-offs

- **Existing mixed pipe expressions change grouping** -> They currently fail in
  the default pipeline, and the change is documented as a parser2 AST break
  with dedicated scenarios and tests.
- **Generic function values may need type arguments that a plain call does not
  infer** -> Cover a generic `|>`/`<|` case end-to-end; if Go cannot infer the
  type arguments from the single argument, add explicit type-argument support
  in the pipe lowering rather than weakening the type rule.
- **Temp hoisting can reorder pre-statements** -> Build the hoist from the left
  operand's translated result and insert it between the left and right
  pre-statement runs; add a side-effect order test.
- **Differential corpus false failures** -> Keep chained `<|` only inside test
  string literals, and document the intentional legacy divergence in
  `docs/compiler2/differences.md`.
- **Generated files drift from `.mygo` sources** -> Re-run the ordered bootstrap
  sync for typeinference2 and codegen2, then compile and test the generated Go;
  never hand-edit `zz_*.gen.go`.

## Migration Plan

No data or deployment migration. Existing single-direction pipe programs start
compiling; chained `<|` programs adopt right-associative grouping. Rollback is
reverting the change set and regenerating the affected `zz_*.gen.go` files.

