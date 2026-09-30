## 1. Track Receiver-Call Dictionary Requirements

- [ ] 1.1 Add a type-inference regression case showing receiver-style calls retain every resolved `using` dictionary requirement, and verify the typed call metadata identifies the correct interface, type arguments, and order.
- [ ] 1.2 Update bootstrap receiver-method lowering to append each matching instance dictionary after explicit arguments, and verify a generated `Set[String].Contains` call supplies the `String` Eq helper.
- [ ] 1.3 Cover receiver methods with multiple `using` constraints and verify generated helper calls pass dictionaries in declared order with the matching concrete types.

## 2. Verify Generated Programs

- [ ] 2.1 Add a regression fixture for a constrained receiver implementation that accepts but ignores its dictionary, and verify generated Go compiles while preserving the constraint argument.
- [ ] 2.2 Run the focused type inference and code generation regression suites, then compile the mailfilter-style fixture and verify its generated helper call has the required arity.
