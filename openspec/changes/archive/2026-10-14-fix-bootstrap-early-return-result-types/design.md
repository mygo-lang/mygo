## Context

See proposal.md for motivation. The current function declaration is placed in the return context using `arrayExpectedEnv`, `inferTypedReturnWith` reads it via `inferArrayReturnValue`, but `inferExprExpectedInner` reverts to normal inference for types like `TApp`, not unifying the return value with the declared type. Block inference uses the type of the last statement as the block result, and the two branches of conditional statements will not unify the results; unresolved generics returned early may therefore have no subsequent constraints.

The generated code uses `ctx.retType` for normal returns, while `translateAstBlockStatement` explicitly returns `TUnit` at the end of the block, subsequently restoring `expected` from the expression's own type; unresolved type variables are converted to Go `any`. The marker method of the prelude `Result[A,E]` does not contain generic parameters, allowing erroneous instances to be assigned via Go interfaces, while the exact variant of `IsErr` leaves `false` after an assertion failure.

These are gaps identified through source code review; the first step still requires reproducing and locating the actual triggering path using the current bootstrap compiler. The design spans inference and generation, therefore this design document is retained.

## Goals / Non-Goals

**Goals:**

- Treat return type constraints as function-level semantics, reliably passed to each explicitly returned typed AST.

- Use existing return generation entry points, covering existing rules for normal returns, tuples, multiple return values, and units.

- Validate generated code and actual execution results within the existing bootstrap testing framework.

**Non-Goals:**

- Do not modify the Go representation of Prelude enums, marker methods, or the runtime implementation of `IsErr`.

- Do not completely refactor control flow or introduce new bottom/never types.

- Do not modify legacy inference/generators, and do not handle independent switch non-exhaustive fallback behavior.

## Decisions

### 1. Establish Return Constraints at Function Entry Points, with Closures Overriding Independently

Establish a return type context for each function declaration and function literal; declared types directly use their MonoType, while undeclared types use the function's independent new type variable. Reuse and generalize the existing return context mechanism in the environment, using explicit function return purpose naming; all function entry points must override the parent context, and closures are prohibited from inheriting outer return types. Array expected inference continues to use the same context, preserving accurate length checks.

Compared to adding a second set of contexts applicable only to `Result`, this approach preserves the single responsibility entry point and can similarly constrain `Option` and ordinary return values. Compared to globally mutable return states, environment scope naturally isolates nested functions.

### 2. Explicit Returns Must Unify Types and Preserve Substitution

At the explicit return inference entry point, first infer the value as expected (preserving special handling for arrays and numbers), then unify the inference type and return context, and apply substitution to the return expression, callee, and its sub-expressions. Do not simply change the top-level `Type` of the expression to the declared type. The synthesized substitution propagates along with existing block inference, allowing early return values ​​of undeclared functions and tail values ​​to jointly constrain function results; branches of conditional statements do not require existing rules with the same value type.

Type errors are located in the return value expression, and a return type mismatch diagnosis is provided. Simply forcibly adding generics in the codegen would mask erroneous payloads and fail to satisfy this check; therefore, inference repair is indispensable.

### 3. Reuse Return Generation Path for All Explicit Returns

The `ReturnWithStmt` within the branch block is routed to the existing `translateAstReturnExpr`/return value path, using the `ctx.retType` of the enclosing function to uniformly handle pre-statements, units, tuples, Go FFI, and existing tail call rules. Other explicit return branches are checked to avoid residual entries that directly generate return values ​​with `TUnit`. Closure generation retains its own child context return type.

No special string replacement for `Err` is added, nor is runtime lenient matching changed; precise generic variant matching remains the correct behavior.

### 4. Acceptance Testing: Check Generated Instances and Runtime Semantics

Reuse the bootstrap compilation temporary module framework of `internal/mygo/compiler`. For critical reproduction, the generated `Err[struct{}, string]` must be checked, and `IsErr`, `IsOk`, and `UnwrapErr` must be executed to avoid Go compilation success being mistakenly considered correctness. The test matrix corresponds to the delta spec, adding negative examples of erroneous payloads/closure signatures, and retaining relevant regressions for array, unit, tuple, and FFI returns.

## Risks / Trade-offs

- [Context Generalization Violates Array Diagnostics] → Retain array expected inference and positional wrapping, and run existing array regressions.

- [Closure Inheritance from Outer Context] → Function literal entries, regardless of whether they are annotated, are given an independent context, covering nested closure test cases.

- [Substitution of Undeclared Functions Not Passed to All Return Paths] → Check for complete typed AST replacements and verify with early Err/tail Ok joint inference. - [Unified return generation changes tuple, unit, FFI, or tail call behavior] → Reuse existing entry points and run these existing regressions instead of adding parallel lowering.

- [Generated files are out of sync with MyGO source code] → Synchronize and rebuild from the modified source code in bootstrap dependency order for verification; manually modifying generated files is prohibited.

## Migration Plan

No user data or API migration is required. After source code repair, synchronize typeinference2 first, then synchronize its dependent codegen2, build a new CLI, and verify bootstrapping and regressions. The new CLI can directly compile early returns that do not explicitly provide generics; existing explicit generic code remains valid. During rollback, the entire source code and corresponding generated artifacts are restored.