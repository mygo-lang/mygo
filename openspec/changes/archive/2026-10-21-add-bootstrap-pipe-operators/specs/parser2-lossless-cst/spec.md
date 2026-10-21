## MODIFIED Requirements

### Requirement: Leading pipe operators continue expressions across lines

Parser2 SHALL treat a line-leading `|>` or `<|` as continuation of the preceding expression regardless of indentation. The pipe levels SHALL parse `<|` as right-associative and looser than the left-associative `|>`, with both looser than `||`; simple `f <| x` and `x |> f` expressions SHALL keep their existing grouping. Intervening whitespace, blank lines and comments SHALL be preserved losslessly. Continuation SHALL NOT cross an explicit statement separator, declaration, branch or block boundary. Other line-leading operators SHALL retain their existing statement-boundary behavior.

#### Scenario: Both pipe operators may start continuation lines
- **WHEN** an expression is followed on the next line by `|>` or `<|` and a valid right operand, with equal or greater indentation
- **THEN** it parses as one expression equivalent to the corresponding single-line expression

#### Scenario: Mixed pipe chain retains grouping
- **WHEN** a multiline chain combines `|>`, `<|` and higher-precedence expressions
- **THEN** its AST has the same precedence and associativity as the same tokens written on one line, with chained `|>` grouping to the left and chained `<|` grouping to the right

#### Scenario: Chained backward pipe groups to the right
- **WHEN** source contains `f <| g <| x`
- **THEN** it lowers as `f <| (g <| x)` rather than `(f <| g) <| x`

#### Scenario: Backward pipe binds looser than forward pipe
- **WHEN** source contains `value |> f <| g`
- **THEN** it lowers as `(value |> f) <| g`, preserving both operator tokens and their source spans

#### Scenario: Comments and blank lines within a pipe chain
- **WHEN** whitespace, blank lines or comments separate an operand from a following line-leading pipe
- **THEN** the chain remains one expression and the CST reproduces all input bytes and source ranges

#### Scenario: Ordinary statements remain separate
- **WHEN** an expression is followed by an ordinary statement or a separate unary negative expression instead of a line-leading pipe
- **THEN** existing statement boundaries remain unchanged

#### Scenario: Continuation respects enclosing boundaries
- **WHEN** a pipe would need to cross a declaration, branch, block terminator or explicit statement separator to obtain its left operand
- **THEN** it does not attach to the expression before that boundary and malformed syntax is diagnosed

#### Scenario: Pipe without an operand is rejected
- **WHEN** a pipe has no left or right operand within its expression region
- **THEN** syntax parsing returns a located diagnostic while retaining malformed bytes, and semantic parsing rejects the source

#### Scenario: Existing trailing operator continuation
- **WHEN** a supported binary operator ends a line and its right operand follows on the next line
- **THEN** existing continuation behavior remains supported

