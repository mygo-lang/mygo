## ADDED Requirements

### Requirement: Struct literal receivers retain their postfix chain
Parser2 SHALL continue parsing a postfix chain after a struct literal,
including a generic struct literal carrying explicit type arguments. When a
struct literal is followed by a field selection, method call, bracket suffix,
call, or cast, the result SHALL be one expression whose base is that struct
literal. The semantic AST and generated Go SHALL apply the suffix to the
constructed value, and syntax parsing SHALL NOT discard a trailing postfix
suffix or report it as an unrelated identifier error.

#### Scenario: Method call in an argument position
- **WHEN** a program calls a method on a struct literal directly inside a call
  argument, such as `fmt.Println(Point { x: 1, y: 2 }.Sum())`
- **THEN** compilation succeeds and the generated Go invokes the method
  implementation with the constructed value

#### Scenario: Method call on an assignment right-hand side
- **WHEN** a program binds the result of a method call on a struct literal,
  such as `let s = Point { x: 3, y: 4 }.Sum()`
- **THEN** the generated Go stores the method result rather than the bare
  struct literal

#### Scenario: Generic struct literal receiver
- **WHEN** a program calls a method on a generic struct literal with explicit
  type arguments, such as `Box[Int] { value: 42 }.Value()`
- **THEN** compilation succeeds and the generated Go invokes the generic
  method implementation with the constructed value

#### Scenario: Field selection after a struct literal
- **WHEN** a program selects a field from a struct literal, such as
  `Point { x: 1, y: 2 }.x`
- **THEN** the selection resolves to the field of the constructed value

#### Scenario: Postfix chain is retained losslessly
- **WHEN** a struct literal followed by a postfix suffix is syntax-parsed
- **THEN** every input byte is retained exactly once and the expression node
  spans both the literal and its postfix suffix
