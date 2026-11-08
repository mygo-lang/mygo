## ADDED Requirements

### Requirement: Placeholder applications have bootstrap backend parity

Both codegen2 and codegen3 SHALL implement the existing placeholder requirements for well-typed calls, whether the resulting function is bound, immediately applied, or used as a pipe operand. Calls SHALL preserve placeholder order, per-invocation evaluation, dispatch, instantiated types, return representations and Go FFI boundary conversions. Existing invalid placeholder forms SHALL retain their diagnostics.

#### Scenario: Stored partial application is invoked repeatedly
- **WHEN** a partial application with a side-effecting supplied argument is stored and invoked twice
- **THEN** each backend SHALL evaluate that argument once per invocation and produce the same observable results

#### Scenario: Multiple placeholders retain parameter order
- **WHEN** a call has multiple direct placeholder arguments and its resulting function is applied
- **THEN** both backends SHALL pass those arguments to the original positions in source order

#### Scenario: Dispatch and FFI results are preserved
- **WHEN** a supported method, constructor, dictionary-dispatched call, or fixed-arity Go FFI call is partially applied
- **THEN** both backends SHALL preserve the dispatch and boundary conversion of the equivalent ordinary call

#### Scenario: Existing invalid forms remain invalid
- **WHEN** a placeholder appears in a variadic or intrinsic call, in a nested non-call expression, or as a callee
- **THEN** both backends SHALL reject the source with the existing applicable located diagnostic
