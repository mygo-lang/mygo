# Tasks

## 1. Resolve dynamic boxing consistently

- [x] 1.1 Add a shared boxing-resolution path for explicit and implicit conversions that selects the unique implementation and constructor metadata; verify inference cases for a valid, missing, and ambiguous implementation.
- [x] 1.2 Add an internal `DynamicBoxExpr` carrying the value, canonical interface identity, implementation ID, and constructor symbol; verify AST substitution, ID assignment, and typed-expression handling preserve those fields and source positions.

## 2. Lower explicit dynamic conversions

- [x] 2.1 Update `as` expression code generation to consume the resolved boxing decision and call the dynamic constructor; verify generated Go contains the wrapper constructor and no concrete-to-interface Go conversion.
- [x] 2.2 Add local and imported-package regression cases for explicit boxing, including missing or incompatible constructor metadata; verify diagnostics identify the source conversion.

## 3. Infer boxing from expected types

- [x] 3.1 Thread expected dynamic interface types into supported expression inference sites, starting with function and method arguments and typed bindings; verify concrete arguments can omit `as` and unrelated MyGO types still fail.
- [x] 3.2 Extend expected-type propagation to return and branch contexts where the current inference structure supports it; verify unannotated conditional joins do not invent a dynamic target.
- [x] 3.3 Add inference/codegen regressions for explicit and implicit forms selecting the same implementation; verify the generated constructor identity matches the expected interface and implementation.

## 4. Verify generated Go integration

- [x] 4.1 Add a cross-package fixture that passes a concrete implementation to a dynamic-interface parameter without `as`; verify the caller's generated Go compiles against dependency metadata.
- [x] 4.2 Add an explicit `as` fixture for a concrete struct implementation; verify generated Go compiles and uses the dynamic wrapper constructor rather than a Go interface cast.
