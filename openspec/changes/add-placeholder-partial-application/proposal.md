## Why

MyGO has first-class function values, closures, and functions that return
functions, but a call must always supply the callee's full argument list. The
only way to specialise an existing function is to write a wrapper closure by
hand, which repeats the parameter list and obscures the argument that is being
fixed. A call-site placeholder gives the language a local, explicit form for
partial application without changing the meaning of any existing call.

## What Changes

- Recognise a bare `_` in a direct call argument as a placeholder:
  `f(a, _, c)` evaluates to a function that supplies the remaining arguments
  in source order, so the example is equivalent to
  `func(b) f(a, b, c) end`.
- Treat the whole call expression as the generated function body. Already
  supplied arguments are re-evaluated on every invocation of the resulting
  function, matching Scala's placeholder semantics. A caller that needs a
  single evaluation binds the value first.
- Allow any mix and any number of placeholders in one call, including
  `f(_, _)` and `f(_)`. `f(_)` is an eta-expansion of `f`. A call with no
  placeholder keeps its current evaluation and typing behavior.
- Resolve placeholders from the callee's already-instantiated parameter types.
  Generic functions and function values therefore produce a monomorphic
  remaining-argument function at the call site. Placeholder argument order
  follows source order rather than parameter names.
- Support placeholders for ordinary functions, generic functions, function
  values, closures, inherent and interface impl methods, static methods, enum
  variant constructors, and Go FFI calls with a fixed arity. Go FFI methods,
  package functions, and values of named Go function types are included; their
  existing Result/Option boundary wrapping applies to the generated function's
  result.
- Reject a placeholder in a Go variadic call, including forwarding shapes
  such as `fmt.Println(_)` and `fmt.Sprintf(format, _)`. Variadic calls
  without placeholders keep their existing semantics and diagnostics.
- Keep placeholders local to direct call arguments. `_` in a let binding,
  tuple destructuring, switch pattern, or lambda parameter keeps its current
  discard/wildcard meaning. Occurrences such as `1 + _`, `_.Field`,
  `_(x)`, or `Ref.new(_, x)` are rejected with a dedicated diagnostic
  instead of being silently re-scoped.
- Keep the lossless CST and formatter spelling unchanged: `_` remains a
  source token and the formatter does not rewrite it into a lambda.
- Update inference, code generation, diagnostics, editor analysis, tests, and
  language documentation. The legacy `--legacy sync` pipeline is not
  extended; it continues to reject the new form.

## Capabilities

### New Capabilities

- `language/placeholder-partial-application`: placeholder syntax and scope,
  Scala-style re-evaluation semantics, result typing and eta-expansion,
  method and enum-constructor support, Go FFI fixed-arity support, variadic
  and intrinsic rejection, and diagnostics.

### Modified Capabilities

- None. Existing function-call behavior and the existing meaning of `_` in
  binding and pattern positions do not change.

## Impact

- Parser2 lowering: no new CST node; expression-position `_` continues to
  lower as an identifier, and the call form is recognised downstream.
- Type inference: `inferCall`/`inferArgs` in
  `internal/mygo/typeinference2/infer.mygo` gain a placeholder-aware path
  that assigns each hole a fresh result parameter type, preserves the callee's
  instantiated parameter types, and builds `func(remaining) -> result`.
- Code generation: `internal/mygo/codegen2/translate_ast.mygo` and
  `translate_expr.mygo` generate a Go function literal for a placeholder
  call, reusing the existing call, impl-helper, enum-constructor, and FFI
  boundary lowering inside the literal body.
- Diagnostics: new messages for non-argument placeholders, variadic
  placeholders, placeholders in callee position, and compiler-intrinsic calls.
- Editor analysis: `_` in a call argument must not be reported as an unknown
  identifier; inferred parameter types should remain available to hover and
  diagnostics.
- Tests and docs: inference and codegen fixtures covering ordinary,
  generic, method, constructor, and Go FFI calls; formatter round-trips;
  negative cases; and a language-documentation section on partial application.
- Performance: each evaluation of a placeholder call allocates a closure that
  captures nothing by itself, because supplied arguments are re-evaluated
  inside the body. This cost is explicit in the source and does not affect
  calls without placeholders.
