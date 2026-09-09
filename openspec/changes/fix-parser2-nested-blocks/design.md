## Context

`blockUntil` currently loops over statements while checking only a caller-provided stop token. Nested block statements must finish parsing before that loop checks the outer stop token.

## Goals / Non-Goals

**Goals:**

- Make block parsing depth-aware through the existing statement/expression parser composition.
- Preserve current `case =>`, `if`, `while`, and top-level block behavior.
- Cover nested-block regressions with focused parser2 tests.

**Non-Goals:**

- No syntax changes or new AST nodes.
- No changes to legacy parser or type inference.

## Decisions

Use recursive parsing of recognized nested block constructs inside `blockItems`, so a nested construct consumes its complete body and matching `end` before control returns to the enclosing loop. Keep terminator ownership in each construct rather than counting raw keyword tokens, avoiding false nesting from identifiers, strings, or comments.

Regenerate `zz_parser.gen.go` from the MyGo source after editing `parser.mygo`; generated output is derived and must not be hand-authored.

## Risks / Trade-offs

- [Risk] A generalized recursion change could alter existing block boundary behavior. -> Mitigation: retain focused tests for arrow cases, if/while blocks, and nested switch cases, then run the full parser2 package tests.
- [Risk] Generated Go may compile while hiding source-level generation errors. -> Mitigation: run the concrete parser2 generation command and compile/test the generated package with a writable cache.
