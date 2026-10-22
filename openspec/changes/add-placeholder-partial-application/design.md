## Context

See `proposal.md` for motivation and the delta spec for the behavior
contract.

The bootstrap pipeline already has every runtime primitive this change needs.
`let f = add` binds a first-class function value, `adder(1)(2)` compiles,
and a hand-written `func(b) add(1, b) end` wrapper works. What is missing is
a call-site form that names the missing arguments.

Two current properties shape the implementation:

- In `inferOrdinaryCall`, a call is checked by unifying the callee type with
  `TFunc(argumentTypes, freshResult)`. `TFunc` unification requires equal
  parameter-list lengths, so a short argument list fails with
  `function arity mismatch` in `typeinference2/unify.mygo`.
- Parser2 lowers a bare `_` in expression position to
  `ast2.ExprKind.IdentExpr("_")`. It reaches inference unchanged and fails
  with `unknown identifier _`. The lossless CST and the formatter never need
  to know that `_` has a new meaning.

`_` already means "discard" in let bindings, tuple and struct destructuring,
switch patterns, and lambda parameter lists. This change must not widen any of
those meanings.

## Goals / Non-Goals

**Goals:**

- A placeholder-aware call path that produces a residual function with the
  callee's instantiated parameter types and result type.
- One shared closure-generation path in codegen that reuses the existing
  ordinary, impl-helper, enum-constructor, and Go FFI call lowering.
- Scala-compatible evaluation: supplied arguments and the callee are
  re-evaluated each time the residual function runs.
- No behavior change for calls without placeholders.
- A dedicated diagnostic for every unsupported placeholder position.

**Non-Goals:**

- Automatic partial application of short calls without `_`.
- Placeholders in arbitrary expressions (`1 + _`, `_.Field`), in callee
  position, or in compiler intrinsics such as `Ref.new`.
- Placeholders in Go variadic calls.
- Placeholders in dynamic typeclass dispatch (`MygoDynamic...Call...`).
- Extending the legacy `--legacy sync` pipeline.
- Capturing supplied arguments once at expression creation.

## Decisions

### Recognise holes in inference instead of adding an AST variant

A new `PlaceholderExpr` variant would force every `ExprKind` match in
inference, codegen, tail-call analysis, and editor analysis to handle a new
shape. Parser-level desugaring into `FuncLitExpr` is not viable because
parser2 rejects lambda parameters without a type annotation, and the type of a
hole is only known after the callee is instantiated.

The chosen approach keeps `IdentExpr("_")` in the parsed AST. `inferArgs`
accepts a new `allowPlaceholder` flag. When it is true and the current
argument is a bare `IdentExpr("_")`, inference allocates a fresh type
variable, attaches it to that argument as its inferred type, and lets the
normal `TFunc` unification bind it to the corresponding parameter type. After
the call's substitution is composed, the hole types are concrete.

This keeps argument order, contextual literal inference, generic
instantiation, and Go assignability checks on the existing code path.

Alternative considered: filter holes out of `inferArgs` and unify each
remaining argument directly. That duplicates the substitution threading and
the Go-assignability and dynamic-box paths in `inferArgsHead`.

### Report the residual type from the existing call result

`inferOrdinaryCall` already has the final result type after unifying the
callee with the synthetic `TFunc`. When at least one argument is a hole, the
inferred type of the whole `CallExpr` becomes:

```
TFunc([applySubst(subst, holeType0), ...], applySubst(subst, result))
```

The typed call keeps the hole expressions in argument order with their
substituted types. Codegen uses that order to build the residual parameter
list. This rule is applied to the ordinary `TFunc` branch, the
`TQualifiedName`-wrapped Go function branch, and the `TApp`-wrapped branch.

The variadic branch rejects a call containing a hole before argument
inference. It also checks `TQualifiedName` wrappers so a qualified Go
variadic function cannot slip through.

The residual function must be monomorphic when it reaches code generation.
MyGO already lets `let f = identity` reach Go codegen as an uninstantiated
generic function value, and the Go compiler rejects that output. This change
does not repair that separate limitation. It instead requires every
placeholder type to be concrete before codegen and reports a dedicated
diagnostic when a type variable remains, so a placeholder cannot add a new
source of invalid generic Go.

### Generate one Go function literal around the existing call lowering

`translateCallAstExpected` is the shared entry point for ordinary calls,
impl-method calls, static methods, enum constructors, and FFI calls. A new
guard at its start detects holes. When present it calls
`translatePlaceholderCall`, which:

1. Builds one fresh Go parameter name per hole and a Go type expression from
   the hole's inferred `MonoType`.
2. Rebuilds the argument list with each hole replaced by a typed
   `IdentExpr` for its generated parameter.
3. Calls the existing `translateCallAstExpected` logic with the residual
   result type as the expected result.
4. Wraps the resulting call, including every pre-statement it produced, in a
   Go function literal.

Because the call lowering runs inside the literal body, supplied arguments,
the receiver, and the callee keep their existing left-to-right evaluation
order and are re-evaluated on every invocation. No value is hoisted out of the
literal.

The residual result type comes from `CallExpr.Type`'s `TFunc` result, so
Go FFI boundary conversions (`(T, error)` to `Result[T, error]`,
`(T, bool)` to `Option[T]`, lone error to `Result[(), error]`) are
unchanged.

Alternative considered: rewrite the typed call into `FuncLitExpr` during
inference. That requires converting `MonoType` back to `TypeExpr` and
constructing `ast2.Param` values, which risks losing qualified Go type
identity and package aliases that codegen already handles from `MonoType`.

### Keep statement-position discarding on the expression path

`translateDiscardBindingValue` normally bypasses the Result/Option wrapper
for an FFI call used as a statement. A placeholder call produces a function
value, so the discard optimisation does not apply. The guard for holes runs
before that optimisation; a discarded placeholder call is lowered as an
ordinary expression.

### Exclude placeholder calls from tail-call lowering

The tail-call analysis matches `CallExpr` shapes in
`codegen2/tailcall.mygo` and in the parallel tail-call branches of
`translate_ast.mygo`. A placeholder call is not a tail call: it returns a
function. Every candidate check SHALL first reject a call whose argument list
contains a hole. The shared predicate lives beside the codegen hole helper so
ordinary and tail-call paths agree.

### Preserve source and editor behavior

No CST node or formatter rule is added. Formatting sees the original `_`
token and round-trips it. The language service gets its type from the typed
argument's `Expr.Type`, which is already keyed by `Expr.ID` in the typed
information, so hover can show the residual parameter type. The unknown
identifier diagnostic disappears because `inferArgs` consumes the hole
before the identifier lookup.

### Unsupported positions fail early with dedicated messages

`inferArgs` only enables placeholder acceptance for direct call arguments.
The following positions keep their existing paths and receive explicit
diagnostics:

- A hole nested in an argument expression, used as a field receiver, or used
  as a callee is diagnosed by the existing expression inference with a
  placeholder-specific message rather than `unknown identifier _`.
- `Ref.new` and `ref.value` are compiler intrinsics; `inferRefCall`
  rejects holes before constructing the intrinsic call.
- A variadic callee rejects holes before `inferGoVariadicCall`.

## Risks / Trade-offs

- [A typed `IdentExpr("_")` leaks into a codegen path that does not expect a
  hole and emits Go's blank identifier] -> Consume holes only in
  `translateCallAstExpected`; add an internal diagnostic if a bare `_`
  reaches ordinary identifier translation.
- [Tail-call lowering has a parallel implementation and misses the new
  predicate] -> Extract one hole predicate and use it in both the ordinary and
  tail-call paths; add a regression that would otherwise emit a trampoline
  for a placeholder call.
- [Generic instantiation assigns the wrong type to a hole] -> Let the existing
  `inferArgsHead` unification bind the hole's fresh variable, then apply the
  composed substitution before reading hole types.
- [A hole remains an unbound type variable and codegen emits a generic
  function value that Go rejects] -> Check hole types for remaining
  `TVar`/`TParam` nodes before emitting the literal and require an annotation
  or immediate application.
- [Go FFI variadic signatures hide behind `TQualifiedName` or a named
  function type] -> Check the effective unwrapped callee type for
  `TVariadic` before accepting a hole.
- [Receiver or supplied arguments are evaluated repeatedly and surprise
  callers] -> This is the specified Scala behavior; document it with a
  side-effecting example and keep the generated body structurally identical
  to the source call.
- [Every invocation allocates a closure] -> The cost is explicit at the call
  site and does not affect calls without `_`; no hidden partial application
  is introduced.
- [`_` becomes visually overloaded across binding, pattern, and argument
  positions] -> Keep the syntactic contexts disjoint, document the rule, and
  provide the hover type so the argument position is unambiguous in editors.

## Migration Plan

This is an additive language change. Existing programs contain no legal
expression-position `_`, so they are unaffected. Rollback is limited to
removing the new inference and codegen branches; no data or artifact migration
is required. The legacy pipeline remains on its current behavior and does not
need a compatibility shim.

## Open Questions

None. The placeholder spelling, re-evaluation semantics, and Go FFI scope were
decided before this design was written.
