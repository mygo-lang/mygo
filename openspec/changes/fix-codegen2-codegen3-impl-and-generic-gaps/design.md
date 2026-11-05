## Context

See `proposal.md` for motivation and the three delta specs for the behavioral
contract. The current failures all occur in or below the shared bootstrap
front-end used by codegen2 and codegen3:

- User-defined generic types are applied by building a substitution from the
  applied type's bound parameter IDs to the supplied argument types, then
  recursively applying that substitution. Type parameters use positional
  negative IDs, so a supplied caller parameter can coincide with one of the
  applied type's own bound IDs. `Result[U, E]` in a `T, U, E` declaration is
  the smallest reproduction: `U` is resolved through `Result`'s second bound
  parameter and both result slots become `E`.
- Instance-method tables store a receiver head derived from the source impl
  target. For `impl lib.Box` that head is `lib.Box`, while the typed receiver
  at a call site is a `TQualifiedName` whose short member is `Box`. codegen2's
  fallback only resolves Go FFI package paths, and codegen3 strips the
  qualification before an exact string comparison. The helper is emitted, but
  the call is not rewritten.
- Inference already turns `value.Method` and `Type.Method` into receiver-
  adjusted function types through impl-method lookup and receiver stripping.
  The typed expression remains an ordinary field selector, however. Both
  backends only rewrite method selectors when they are direct call callees, so
  method values and method expressions reach Go unchanged.

codegen3 consumes the method table built by `codegen2.BuildInstanceMethodTable`,
then applies its own receiver normalization in `ir3`. Fixes in the shared table
and inference layer therefore affect both backends, while each backend still
needs its own method-value emission step.

## Goals / Non-Goals

**Goals:**

- Make generic type application capture-free for all user-defined generic
  types and aliases while preserving existing unification substitutions.
- Define one canonical receiver identity shared by inference candidate keys,
  codegen2 matching, and codegen3 instance-method matching.
- Lower bound method values and unbound method expressions to buildable Go in
  both codegen2 and codegen3, including generic and cross-package methods.
- Keep direct receiver calls, direct type-qualified calls, field access, helper
  names, and codegen2's default/legacy ABI unchanged.

**Non-Goals:**

- Changing the legacy `internal/mygo/codegen` pipeline.
- Introducing new method-value syntax or a new source keyword.
- Rewriting MyGO impl methods into Go receiver methods. Helpers remain
  top-level functions.
- Changing dynamic-interface dispatch or implementing runtime reflection for
  method values.
- Changing struct-field precedence. A declared field named like a method still
  wins for a non-call selector.

## Decisions

### D1 — Apply user-defined type arguments with a simultaneous substitution

Separate the substitution used for generic type application from general
unification substitution. For a type application, resolve the supplied type
arguments in the caller's context, then replace the applied type's bound
parameters in one traversal. A replacement value is opaque to that application:
it is not recursively resolved through another binding of the same applied
type. This preserves `Result[U, E]` as two distinct caller parameters.

The existing `applySchemeTypeArgs` path remains the single entry point for
alias and generic-type application, but it uses the simultaneous form instead
of feeding negative-ID mappings through the chain-resolving `applySubst`. The
general unification substitution keeps its current behavior; only application
of a type's own parameter list changes.

Alternative considered: rename every applied type's bound parameters to fresh
global IDs before substitution. That also avoids capture, but requires a fresh
ID allocator and a second traversal, and risks changing alias identity and
cache/test output. Simultaneous replacement is smaller and matches the
mathematical meaning of applying type arguments.

Alternative considered: special-case `Result`. Rejected because the defect is
in generic application, not in the Result declaration; `Map`, aliases, and
future multi-parameter types must obey the same rule.

### D2 — Canonicalize receiver identity as package path plus member

Represent an impl receiver with a canonical identity rather than a bare short
name. The identity is `(defining package path, member name)` for nominal types,
with an empty path for the current package or compiler builtins. The helper's
owner remains the package containing the impl declaration; receiver identity
and helper ownership are separate concepts.

In codegen2, derive the receiver identity from the impl target and the known
MyGO package aliases. Add it to instance-method entries/candidates. Matching
uses the canonical identity from the typed receiver, rebuilding it from the
receiver's `TQualifiedName`. The existing Go FFI alias fallback remains for Go
types, but MyGO package paths are resolved with the same package-aware logic.

In codegen3, carry the receiver path and member in `ir3.InstanceMethod` rather
than reducing `TQualifiedName` to a bare member. `instanceReceiverName` or its
replacement derives the same identity from the typed receiver, and
`findInstanceMethod` compares both fields. The table builder converts the
codegen2 entries without discarding the path.

For `impl lib.Box` declared in `app`, the receiver identity is
`(example.com/gap/lib, Box)` and the helper is a local `app` symbol. For an impl
declared inside `lib`, the identity is the same but the helper is `lib`-owned
and emitted with the `lib` alias in the consumer.

Alternative considered: strip all package qualifiers and match on `Box` alone.
Rejected because a local `Box` and `lib.Box` can both have a method with the
same name; the current helper names differ, but the call table would become
ambiguous.

Alternative considered: reject impls on imported types. This removes the silent
failure but does not provide the requested feature; helper lowering is already
possible, so the design completes it instead.

### D3 — Mark method values in the typed AST

Inference already computes the correct function type by finding an impl-method
symbol and stripping the receiver. Preserve that resolution explicitly in the
typed AST instead of leaving a plain `FieldExpr`. Add a typed method-value form
that records:

- the receiver expression for a bound value, or the nominal type for an
  unbound method expression;
- the method name;
- the receiver-adjusted function type and any explicit type arguments;
- whether the expression came from a field or an impl method, so codegen does
  not re-run ambiguous lookup.

Struct-field lookup retains precedence. If the receiver has a declared field
with that name, inference produces the existing field expression and no
method-value marker. Only an impl-method result creates the marker.

Alternative considered: infer function type from `Expr.Type` and re-run method
candidate matching in codegen. Rejected because a function-typed struct field
can have the same type shape as a method value, and codegen does not currently
carry the symbol index needed to distinguish them reliably.

### D4 — Bound values become one-evaluation closures; unbound values use the helper

For `let fn = value.Method`, codegen evaluates `value` once at method-value
creation, stores it in a temporary when necessary, and emits a closure whose
body calls the mangled helper with that receiver plus later arguments. This
matches Go method-value evaluation order and avoids re-running receiver side
effects on every call.

For `let fn = Type.Method`, codegen emits the helper as a function value. Its
type keeps the receiver as the first parameter. A direct call
`Type.Method(receiver, args...)` is lowered directly to the helper call.

codegen2 performs this in expression translation using the existing helper
candidate and selector-qualification logic. codegen3 lowers the typed marker to
an IR closure or helper variable; `RewriteInstanceMethods` must handle the new
form rather than only `ApplyExpr` callees. Generic helper calls retain the same
explicit type-argument and dictionary selection used by direct calls.

Alternative considered: lower `value.Method` to a raw Go method value. There is
no Go receiver method because MyGO emits top-level helpers, so this produces the
current invalid selector.

### D5 — Tests exercise both backends and observable behavior

Focused inference tests cover substitution and aliases. Cross-package tests
cover consumer-declared impls, imported-package impls, and same-short-name
types. Method-value tests cover storage, higher-order arguments, immediate
calls, receiver side-effect count, fields with colliding names, generic
receivers, and imported methods.

Every accepted generated-Go fixture is built with `go build`; method-value
fixtures also execute and compare results. codegen3 runs use an isolated cache
and temporary module so generated files never enter the checkout. Bootstrap
tests and helpers are written in MyGO where practical, with inline Go only to
bridge to existing compiler test entry points.

## Risks / Trade-offs

- [Simultaneous substitution changes alias expansion] → Distinguish type
  application from unification substitution, and test direct types, aliases,
  nested applications, and multi-parameter types. Do not change substitution
  used by unification itself.
- [Canonical receiver identity changes existing method lookup] → Add explicit
  local-versus-imported same-name tests and verify imported-impl behavior that
  already worked. Keep helper ownership and receiver identity separate.
- [Adding a typed AST form touches many exhaustive switches] → Add it only in
  the typed AST, make every switch update compile-checked, and run the full
  compiler test suite before enabling both backends.
- [Bound method values duplicate receiver side effects] → Evaluate the receiver
  into a temporary at creation and test with a counter-returning receiver
  expression.
- [Generic or typeclass method values need dictionaries/type arguments] →
  Reuse the direct-call candidate and requirement expansion logic; add focused
  generic and `using`-constraint fixtures before declaring parity.
- [Codegen output changes for previously broken programs] → No source
  migration is required. The generated-Go ABI and helper names remain stable;
  only previously invalid or mistyped forms change.

## Migration Plan

1. Land the substitution fix and its inference tests first; this restores
   correct generic types independently of method dispatch.
2. Canonicalize receiver identity in the shared method table and update
   codegen2 and codegen3 matching together. Verify the existing imported-impl
   fixtures still build.
3. Add the typed method-value form and lower it in codegen2 and codegen3.
4. Update `docs/compiler/semantics.md` from the field-only selector contract to
   the new method-value contract.
5. Run focused inference tests, codegen2/codegen3 build fixtures, runtime
   checks, then the full bootstrap suite.

Rollback is a revert of the source and test changes. No persisted metadata,
generated-Go ABI, or migration step changes are required. codegen2 remains the
default backend and codegen3 remains opt-in.
