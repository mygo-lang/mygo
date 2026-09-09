# parser2-nested-blocks Specification

## Purpose

Define parser2 ordinary-string Unicode escape behavior so source using
`\uXXXX` is parsed correctly and failures identify invalid escape syntax rather
than being misdiagnosed as enclosing block terminator errors.

## Requirements

### Requirement: Ordinary strings support four-digit Unicode escapes

Parser2 SHALL decode `\uXXXX` in an ordinary quoted string, where each `X` is
an ASCII hexadecimal digit and the resulting value is a Unicode scalar value.
The decoded scalar SHALL contribute one rune to the resulting string literal.

#### Scenario: Valid Unicode escape in a nested control-flow expression
- **WHEN** an ordinary string containing `\u0000` appears in an expression
  within nested `switch`, `case ... then`, and `if` blocks
- **THEN** parser2 SHALL accept the complete source and preserve the enclosing
  block terminators for their owning constructs

#### Scenario: Valid non-ASCII Unicode escape
- **WHEN** an ordinary string contains `\u4E2D`
- **THEN** parser2 SHALL decode that escape as the single rune `中`

### Requirement: Invalid Unicode escapes are rejected at the string literal

Parser2 SHALL reject a `\u` escape with fewer than four hexadecimal digits, a
non-hexadecimal digit, or a decoded UTF-16 surrogate value.

#### Scenario: Incomplete Unicode escape
- **WHEN** an ordinary string contains `\u12` before its closing quote or
  end-of-input
- **THEN** parser2 SHALL return a parse error for the string literal

#### Scenario: Non-hexadecimal Unicode escape
- **WHEN** an ordinary string contains `\u12G4`
- **THEN** parser2 SHALL return a parse error for the string literal

#### Scenario: Surrogate Unicode escape
- **WHEN** an ordinary string contains `\uD800`
- **THEN** parser2 SHALL return a parse error for the string literal

### Requirement: Existing string modes retain their current behavior

Parser2 SHALL retain existing ordinary-string escapes and SHALL NOT interpret
Unicode escapes in raw or multiline strings as part of this change.

#### Scenario: Existing null escape remains valid
- **WHEN** an ordinary string contains `\0`
- **THEN** parser2 SHALL continue to decode it as a null rune

#### Scenario: Raw Unicode spelling remains literal
- **WHEN** a raw string contains the characters `\u4E2D`
- **THEN** parser2 SHALL preserve those six characters literally
