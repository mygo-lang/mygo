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

- `POrElse` backtracks only when the left parser failed without consuming input
- `PAttempt` resets a failure to non-consuming, allowing `choice`-style recovery
- `PMany` and `PMany1` preserve consuming failures and stop on a clean non-consuming
  failure
- `PLabel` replaces the expected text on a non-consuming failure
- `PLookAhead` parses without advancing the input state

## Main combinators

- `PPure`, `PFail`
- `PMap`, `PBind`, `PThen`
- `POrElse`, `PChoice`, `PAttempt`
- `PMany`, `PMany1`, `POptional`
- `PBetween`, `PSepBy`, `PSepBy1`
- `PSatisfy`, `PChar`, `PString`, `PEof`
- `PIdentifier`, `PDigit`, `PLetter`, `PAlphaNum`

## Intended use

The package is designed for:

- small grammar recognizers
- config/text format parsers
- token-level parsers that need readable backtracking rules

For large grammars, keep tokenization and high-level syntax parsing separate and
compose the two layers with these combinators.
