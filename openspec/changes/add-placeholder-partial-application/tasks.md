## 1. Inference plumbing

- [x] 1.1 Add an `allowPlaceholder` flag to `inferArgs` and `inferArgsHead`, update every call site, and verify `go build ./internal/mygo/typeinference2` succeeds.
- [x] 1.2 Allocate a fresh type variable for a bare `IdentExpr("_")` when the flag is set, skip environment lookup, and verify `add(1, _)` infers `func(Int) -> Int`.
- [x] 1.3 Apply the composed substitution to hole types and return `TFunc(holeTypes, result)` from the `TFunc`, `TQualifiedName`, and `TApp` branches; verify generic `identity(_)` and a function value `f(_, 2)` both infer the expected residual type.
- [x] 1.4 Reject a placeholder in a variadic call, including a `TQualifiedName`-wrapped callee, before `inferGoVariadicCall`; verify `fmt.Println(_)` and `fmt.Sprintf(format, _)` fail with the variadic placeholder diagnostic.
- [x] 1.5 Reject placeholders in `Ref.new` intrinsic calls; verify the intrinsic diagnostic is reported at the call site.
- [x] 1.6 Add placeholder-specific diagnostics for non-direct arguments and callee position; verify `1 + _`, `_.Field`, and `_(1)` fail with those messages instead of `unknown identifier _`.
- [x] 1.7 Verify nested calls bind to the innermost call: `outer(inner(_), 2)` yields a function whose first argument is `inner`'s residual function.
- [x] 1.8 Run the existing inference suite and verify `let _`, tuple destructuring, struct patterns, and switch wildcards keep their current behavior.
- [x] 1.9 Detect unbound `TVar`/`TParam` hole types before codegen and report the annotation-or-application diagnostic; verify `let g = identity(_)` fails while `let g: func(String) -> String = identity(_)` succeeds.

## 2. Code generation

- [x] 2.1 Add one shared helper that detects placeholder arguments and returns their indexes; verify it with a unit test over zero, one, and several holes.
- [x] 2.2 Generate a Go function literal in `translatePlaceholderCall`, replace each hole with a generated parameter, and keep every call pre-statement inside the literal body; verify the generated Go contains a function literal and the compiled fixture returns the expected value.
- [x] 2.3 Preserve Scala re-evaluation order: a side-effecting callee and supplied argument each run once per invocation; verify with a counter fixture invoked twice.
- [x] 2.4 Route ordinary, generic, and function-value placeholder calls through the shared path; verify each fixture compiles and returns the same value as the fully applied call.
- [x] 2.5 Route impl methods and static methods through the shared path while reusing `translateImplMethodCall`; verify a receiver-side-effect counter runs once per invocation and the helper dispatch matches a direct call.
- [x] 2.6 Support enum variant constructors; verify `Some(_)` applied to `1` produces the same value and type as `Some(1)`.
- [x] 2.7 Support Go FFI fixed-arity package functions, methods, and named function type values; verify `(T, error)`, `(T, bool)`, and lone-error results keep their Result/Option wrapping.
- [x] 2.8 Let a discarded placeholder call use ordinary expression lowering instead of the FFI statement-discard optimisation; verify the generated Go compiles.
- [x] 2.9 Exclude placeholder calls from tail-call lowering in `tailcall.mygo` and the parallel tail-call branches; verify a recursive function using `f(a, _)` generates a closure rather than a trampoline.
- [x] 2.10 Report an internal diagnostic if a bare `_` reaches ordinary identifier translation; verify the guard with a focused codegen test.

## 3. Diagnostics and editor analysis

- [x] 3.1 Give every new rejection a stable message and source position, and verify each message with an inference fixture.
- [x] 3.2 Ensure the language service no longer reports a call-argument `_` as an unknown identifier; verify with an LSP analysis test.
- [x] 3.3 Expose the hole's inferred parameter type to hover; verify hovering `_` in `add(1, _)` reports `Int`.
- [x] 3.4 Verify a residual-function application with the wrong argument type reports a type mismatch at the application site.

## 4. Formatter and documentation

- [x] 4.1 Add a formatter round-trip fixture for `add(1, _)` and verify formatting leaves it byte-identical.
- [x] 4.2 Document placeholder syntax, source-order parameters, per-invocation re-evaluation, supported callees, and unsupported positions in `docs/compiler/semantics.md`.
- [x] 4.3 Add a `HISTORY.md` entry describing the additive language change and its Scala-style evaluation rule.

## 5. Validation

- [x] 5.1 Run the focused inference suite for placeholder typing and diagnostics and confirm all cases pass.
- [x] 5.2 Run the focused codegen suite for ordinary, method, constructor, FFI, and tail-call exclusion cases and confirm all cases pass.
- [x] 5.3 Compile the generated Go for every new fixture with `go build` and confirm no unused-value or invalid-closure output.
- [x] 5.4 Sync the changed bootstrap packages in dependency order with a task-local `GOCACHE` and confirm the self-hosted compiler rebuilds.
- [x] 5.5 Run the full test suite under a memory limit and timeout, then confirm the results match the pre-change baseline except for the new fixtures.
- [x] 5.6 Run `openspec validate add-placeholder-partial-application` and confirm the change is valid.

## 6. Compatibility

- [x] 6.1 Confirm the legacy `--legacy sync` pipeline still rejects expression-position `_` without emitting partial generated code.
- [x] 6.2 Re-run representative existing programs without placeholders and confirm generated output and runtime behavior are unchanged.
