## ADDED Requirements

### Requirement: Pipes apply placeholder functions on both bootstrap backends

Both codegen2 and codegen3 SHALL accept a well-typed single-placeholder call as the function operand of either pipe, including when the other operand is a multiline function literal. The generated program SHALL execute the same call and return the same result as the corresponding direct call, preserving the existing source-order pipe evaluation and placeholder re-evaluation rules.

#### Scenario: Backward pipe supplies a callback
- **WHEN** `WithSession` takes two integers and a `func() -> ()` callback and returns `Result[(), Error]`, and source evaluates `WithSession(1, 2, _) <| func() -> ()` with a multiline body ending in `()` and `end`
- **THEN** both backends SHALL compile and run the program, invoke the callback as the direct call does, and return the same result

#### Scenario: Forward pipe supplies a callback
- **WHEN** a compatible callback value is piped as `callback |> WithSession(1, 2, _)`
- **THEN** both backends SHALL compile and run the equivalent direct call with the same observable behavior

#### Scenario: Pipe and partial application effects preserve their respective order
- **WHEN** both pipe operands and the supplied arguments or callee of a placeholder call have observable effects
- **THEN** pipe operands SHALL be evaluated left to right, and the placeholder callee and supplied arguments SHALL be evaluated only upon invocation in the order specified for partial application

#### Scenario: Incompatible callback is rejected
- **WHEN** a piped value is incompatible with the sole remaining placeholder parameter
- **THEN** both backends SHALL reject it with a source-located type diagnostic without emitting invalid Go
