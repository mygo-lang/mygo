## ADDED Requirements

### Requirement: Separators precede trailing line comments

When the formatter wraps a delimited construct and appends separators, it SHALL place each separator after the item's significant content and before any line comment that trails that item. It SHALL NOT append a separator to a region that contains only trivia. Comment text SHALL be preserved verbatim, comments SHALL be emitted on their own lines at the item indentation, and an item that follows a comment SHALL keep that indentation.

#### Scenario: Trailing comment keeps its text and gains no separator
- **WHEN** a wrapped list or struct literal ends with a line comment after its last item
- **THEN** the comment is reproduced verbatim on its own line with no separator appended, and formatting the result again is byte-for-byte identical

#### Scenario: Comment between items preserves the next item's indentation
- **WHEN** a line comment sits between two items of a wrapped delimited construct
- **THEN** the comment occupies its own line at the item indentation and the following item keeps that same indentation

#### Scenario: Separator never follows a line comment
- **WHEN** the formatter emits a separator together with an item that is followed by a line comment
- **THEN** the separator appears before the comment and the comment text is unchanged

#### Scenario: Wrapped struct literal fields
- **WHEN** a wrapped struct literal contains a field comment
- **THEN** the field labels and comment text are preserved and repeated formatting adds no characters

### Requirement: Constraint clause rendering keeps its separators

The formatter SHALL render a declaration's `using` constraint clause inside the declaration header with the `using` keyword separated from each constraint by canonical spacing, for functions and implementation methods in block form as well as interface and implementation signatures. It SHALL NOT emit text whose tokenization drops the `using` keyword or merges it with a constraint name.

#### Scenario: Function constraint clause stays in the header
- **WHEN** a function declaration carries `using ToString[Int]` on its signature line
- **THEN** the header retains the clause as `using ToString[Int]` and the body follows unchanged

#### Scenario: Multiple constraints keep their separators
- **WHEN** a constraint clause holds several constraints such as `using A[Int], Show`
- **THEN** every constraint and separator is reproduced in source order without losing the keyword or the comma

#### Scenario: Implementation member constraint clause
- **WHEN** a block-form implementation method carries a constraint clause
- **THEN** the clause stays in the member header and the method body is unaffected

#### Scenario: Interface signature constraint clause
- **WHEN** an interface member signature carries a constraint clause
- **THEN** the signature keeps the clause with its separating spaces

#### Scenario: Reformatting a constraint clause is stable
- **WHEN** formatted output containing a constraint clause is formatted again
- **THEN** the second output equals the first and still parses with the clause present

### Requirement: Formatted output preserves the parsed structure

A single formatting pass SHALL produce text that re-parses to a structure equivalent to the input, ignoring source positions and comments. The formatter SHALL NOT emit text that changes tokenization, such as dropping a required separator so that two tokens merge. When formatting reaches a fixed point, that output SHALL satisfy the same equivalence. A repository-wide check SHALL compare the lowered structure of every accepted source before and after formatting and SHALL report any remaining exception explicitly.

#### Scenario: One pass preserves structure
- **WHEN** a source containing constraint clauses, comments in delimited constructs, and wrapped expressions is formatted once and the result is parsed
- **THEN** the lowered structure matches the input structure apart from source positions

#### Scenario: Fixed point does not persist a corrupted layout
- **WHEN** a declaration with a constraint clause is formatted to a fixed point
- **THEN** the result still contains the constraint clause and parses to the same structure as the input

#### Scenario: Repository-wide equivalence
- **WHEN** the formatter runs over the repository's MyGO sources
- **THEN** each accepted source lowers to an equivalent structure before and after formatting, and any file that does not is reported rather than silently excluded

