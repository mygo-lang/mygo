# Design

## Context

See `proposal.md` for the motivation and `specs/language/dynamic-typeclass-values/spec.md` for the required behavior. Dynamic typeclass interfaces are represented by generated wrapper types with per-implementation constructors. The caller can obtain constructor and implementation metadata from generated package artifacts. Today, type ascription and ordinary Go conversion lowering do not reliably select that wrapper constructor, while type inference has separate information about the source expression and target type.

## Goals / Non-Goals

**Goals:**

- Make explicit conversion and expected-type coercion converge on one validated dynamic boxing decision.
- Preserve typeclass implementation evidence across package boundaries using existing generated metadata.
- Keep the inferred expression type equal to the requested dynamic interface after a successful boxing decision.

**Non-Goals:**

- Implement general covariance or contravariance between MyGO types. Boxing changes runtime representation and carries implementation evidence; it is not ordinary subtype assignment.
- Replace MyGO type inference with Go's type checker. Go type information can answer Go FFI assignability questions, but cannot select MyGO typeclass implementations or dynamic wrapper constructors by itself.
- Infer a dynamic target for expressions without an expected type, including unannotated conditional branch joins.

## Decisions

### Use a typed AST node to carry the boxing decision

Add an internal `DynamicBoxExpr` expression kind. It carries the boxed value, canonical interface package/name, stable implementation ID, and caller-visible constructor symbol. Type inference creates this node after resolving a unique implementation and constructor metadata; it does so for both explicit `as I` and implicit expected-type coercion. AST substitution and traversal preserve the node and its source position. Code generation consumes the recorded constructor symbol and never reruns implementation selection.

Alternative considered: infer only that source and target are compatible and let code generation rediscover the implementation. This duplicates selection logic and risks inference accepting a value for which codegen cannot choose a unique constructor.

Alternative considered: encode the decision only in the expression's inferred type. The type does not identify which implementation evidence was selected, especially when implementations or interface packages share visible names, so an explicit node is needed.

### Carry canonical package identity into inference

Pass each source unit's canonical import path alongside its declarations. Local implementation selection uses that path to derive the same caller-visible constructor symbol as dynamic metadata generation; package names and source filenames are not reliable substitutes for import identity. Imported implementation selection continues to use each dependency's canonical path and declarations.

MyGO performs candidate enumeration, compatibility checks, and uniqueness selection. For source-free dependencies, a small Go FFI helper validates metadata ABI/schema and projects implementation records into simple fields; MyGO filters those records by interface and target and selects the unique candidate. For source-backed imports, after MyGO uniquely selects an AST candidate it may call a Go FFI helper to validate the dependency metadata ABI and retrieve that exact record's stable ID. Neither helper searches for or chooses an implementation. This keeps JSON decoding and metadata schema validation at the Go metadata boundary without adding inline Go or moving MyGO impl resolution into Go. Local metadata is produced later in the codegen pipeline, so local selection keeps its source identity key until codegen associates it with the generated constructor, whose payload already carries the canonical metadata ID.

### Lower through the generated constructor, never a Go conversion

Code generation should consume `DynamicBoxExpr` and emit the constructor associated with its implementation ID, including when the node is a function's direct return value. For imported packages, the constructor is the caller-visible constructor assembled from dependency metadata. This produces the common dynamic wrapper and records the selected implementation ID; emitting `TargetInterface(value)` is invalid for concrete MyGO structs and loses that evidence.

Alternative considered: generate Go interfaces for MyGO typeclasses and rely on Go method sets. Existing MyGO impls lower to package-level helpers/adapters and need not define Go method sets, so Go interface conversion would not model the language's implementation relation.

### Propagate expected dynamic types at supported expression sites

At sites where the target type is already known—initially function/method arguments and explicitly typed bindings, with return and branch contexts where the existing inference structure can pass an expected type—pass that target into expression inference. Only a dynamic typeclass target triggers boxing; ordinary same-type unification and Go FFI assignability keep their existing paths. Do not synthesize a target from sibling expressions in an unannotated join.

Alternative considered: add a global post-inference pass that rewrites every compatible expression. That would have to reconstruct contextual expectations after inference and could alter overload/implementation resolution; local expected-type propagation keeps the decision tied to the source context.

### Keep static dispatch and Go FFI assignability separate

Dynamic boxing is selected only when the expected MyGO type is a generated dynamic interface and a MyGO implementation is resolved. Go FFI assignability continues to use Go's assignability rules for imported Go types. The Go type service is not needed to determine MyGO implementation evidence and must not make otherwise distinct MyGO types unify.

## Risks / Trade-offs

- [Some expression contexts do not currently carry expected types] → Thread expected types only through identified inference sites and list their coverage in tasks; retain explicit `as` for other contexts until they are supported.
- [Dependency metadata may lack a constructor or use an incompatible ABI] → Reuse dynamic metadata validation and emit a targeted compile-time diagnostic before code generation.
- [Implicit coercion may mask which implementation is selected] → Require a unique implementation and preserve its stable implementation identity in the generated constructor call.
- [Generated output can accidentally regress to a Go type conversion] → Add focused inference and codegen regression cases for local and imported implementations, including generated Go compilation.

## Migration Plan

No source migration is required. Existing explicit `as I` expressions remain valid and are lowered through the wrapper constructor. Callers may omit `as I` at supported sites where the expected target is already `I`. Existing generated dependency metadata remains the source of constructor availability; if the implementation needs an incompatible metadata/adapter ABI change, version it and diagnose stale dependency artifacts.
