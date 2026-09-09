## Why

parser2 incorrectly treats the first `end` inside a nested `switch`, `if`, or `while` as the terminator of an enclosing `case ... then` body. Valid nested control-flow source is therefore truncated and reported as missing an `end`.

## What Changes

- Make parser2 block-body parsing depth-aware for nested block constructs.
- Ensure each nested `switch`, `if`, and `while` consumes its own complete body and terminator before the enclosing block resumes.
- Add parser2 regression coverage for nested `switch` inside a `case ... then` body and related nested-block combinations.

## Capabilities

### New Capabilities

- `parser2-nested-blocks`: Correct parsing of nested control-flow blocks in parser2.

### Modified Capabilities

<!-- No existing spec requirement is being modified; this adds parser2 behavior coverage. -->

## Impact

- Affects `internal/mygo/parser2/parser.mygo` and its generated Go output.
- Adds parser2 tests; no public API or syntax changes.
- Downstream formatter and bootstrap pipelines benefit from accepting valid nested control-flow source.
