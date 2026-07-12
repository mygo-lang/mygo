# `lib/text/parsec`

This package provides a small parser combinator core in MyGO, modeled after
the parts of F# FParsec that are most useful for handwritten grammars.

## Core model

- `Parser[A]` wraps `func(State) -> Reply[A]`
- `State` tracks input, byte index, line, and column
- `Reply[A]` carries:
  - success/failure
  - whether the parser consumed input
  - the parsed value
  - the next state
  - the parse error

## Semantics

- `OrElse` backtracks only when the left parser failed without consuming input
- `Attempt` resets a failure to non-consuming, allowing `choice`-style recovery
- `Many` and `Many1` preserve consuming failures and stop on a clean non-consuming
  failure
- `Label` replaces the expected text on a non-consuming failure
- `LookAhead` parses without advancing the input state

## Main combinators

- `Pure`, `Fail`
- `Map`, `Bind`, `Then`
- `OrElse`, `Choice`, `Attempt`
- `Many`, `Many1`, `Optional`
- `Between`, `SepBy`, `SepBy1`
- `Satisfy`, `Char`, `PString`, `Eof`
- `Identifier`, `Digit`, `Letter`, `AlphaNum`

## Intended use

The package is designed for:

- small grammar recognizers
- config/text format parsers
- token-level parsers that need readable backtracking rules

For large grammars, keep tokenization and high-level syntax parsing separate and
compose the two layers with these combinators.
