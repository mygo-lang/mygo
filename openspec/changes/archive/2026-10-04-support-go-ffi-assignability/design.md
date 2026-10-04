# Design

## Context

See proposal.md - Why. Bootstrap currently gathers Go package signatures with
`go/types`, converts them to serialized FFI metadata, and performs HM inference
in `typeinference2`. Its argument checker first tries MyGO unification and
currently has a narrow metadata-based fallback for imported named interfaces.
That fallback cannot reliably account for concrete pointer method sets across
packages.

## Goals / Non-Goals

**Goals:**

- Make Go FFI argument acceptance match Go assignability for Go values at Go
  call boundaries.
- Correctly model `Ref[T]` as Go `*T` for pointer method-set checks.
- Keep Go-specific type rules authoritative in Go's type service while keeping
  MyGO inference in `typeinference2`.
- Preserve argument source positions and useful failure diagnostics.

**Non-Goals:**

- Replace or supplement Hindley-Milner inference with a Go type inference
  engine.
- Reimplement all of Go's type system in `typeinference2`.
- Apply Go assignability to ordinary MyGO calls, assignments, or unification.
- Allow conversions that Go does not classify as assignability.

## Decisions

1. **Use a Go type service for boundary assignability.** The service will use
   `go/types` (which already loads imported package type information) to answer
   whether a concrete source Go type is assignable to a target Go type. This
   delegates aliases, named types, interfaces, embedded methods, and pointer
   method sets to the same rules used by Go. The existing serialized interface
   method metadata can remain useful for selector resolution, but should not be
   the authority for assignment decisions.

2. **Keep the service narrow and boundary-scoped.** `typeinference2` should ask
   a focused compatibility question after normal unification fails for a Go
   FFI call argument. It should not invoke Go inference or normalize all types
   through Go. The compiler boundary owns access to loaded Go packages and may
   provide a callback/service or precomputed assignment facts to the bootstrap
   inference call.

3. **Represent source pointer shape explicitly at the query boundary.** A
   MyGO `Ref[T]` argument maps to Go `*T`; compatibility must query that Go
   pointer type, not the element type `T`. Existing FFI type resolution and
   generated argument expressions remain unchanged when Go says the argument
   is assignable.

4. **Preserve strict failure behavior.** A negative Go assignability result
   falls through to the current type mismatch path. The check must not relax
   ordinary HM unification or accept named types based solely on identical
   underlying structure.

5. **Validate the final emitted call with Go.** Regression coverage should
   include a concrete pointer receiver implementing an imported interface (the
   SMTPBackend / smtp.Backend shape), compatible interface values, and negative
   cases. The generated Go should pass `go/types` or a Go build check for those
   fixtures.

6. **Treat an empty tuple pattern as unit when matching a unit value.** The
   parser represents `()` in a pattern as an empty `TuplePattern`, while the
   inference model represents the unit value with `TUnit`. Pattern binding
   should accept that empty pattern for `TUnit` (including as the payload of
   `Ok(())`) and continue to require `TTuple` for non-empty tuple patterns.
   This keeps the surface syntax and the distinct unit type aligned without
   changing tuple destructuring rules.

7. **Filter automatic prelude imports per generated source file.** An
   external test file receives the same automatic prelude import metadata as
   ordinary MyGO files, but generated Go should include that import only when
   the file's emitted code refers to a prelude-provided name. Reuse the existing
   per-file usage analysis where possible so external tests do not get an
   unused dot-import while real helper references remain available.

8. **Dispatch imported static inherent methods through their mangled helpers.**
   A call such as `backends.SendPulseBackend.New(...)` has a package-qualified
   type expression as the base of its final selector. The current static
   inherent-method codegen path recognizes only an unqualified `IdentExpr`
   base, so it falls through to ordinary selector translation and emits
   `backends.SendPulseBackend.New(...)`. Imported MyGO inherent methods are
   emitted as exported package-level helpers (for example,
   `MygoIN16SendPulseBackendM3New`), so the call site must recognize the
   qualified type form and emit `backends.MygoIN16SendPulseBackendM3New(...)`.
   Preserve ordinary Go package members and Go FFI selectors when the
   qualified type/method pair is not an imported MyGO inherent method.

## Risks / Trade-offs

- [Go type information may not be available at a particular inference entry
  point] -> Keep the service optional for non-FFI inference and produce the
  existing diagnostic when the query cannot be resolved; ensure bootstrap
  orchestration threads it through all package/import paths.
- [Treating every parenthesized pattern as unit could break tuple matching] ->
  Apply the unit rule only to an empty `TuplePattern` matched against `TUnit`;
  retain arity checks for `TTuple` and reject non-empty tuple patterns for
  unit.
- [Filtering prelude imports may miss names introduced indirectly by lowering]
  -> Base the decision on the generated file's actual prelude symbol references
  and cover helper calls, enum constructors, and type references in regression
  cases.
- [Reconstructing source types from serialized strings can lose identity or
  pointer shape] -> Prefer retaining type handles or canonical type descriptors
  from package loading; if serialization is required, include import path,
  named-type identity, generic arguments, and pointer/reference shape.
- [A Go service call per argument may add compilation cost] -> Cache assignability
  queries by canonical source and target type identity if profiling shows the
  lookup is material.
- [Qualified type selectors can also name Go package members] -> Restrict the
  static-method rewrite to known imported MyGO types and methods, leaving
  unresolved selectors on the existing ordinary package/FFI path.

## Migration Plan

No source migration is expected. The compiler accepts additional Go-valid FFI
calls; invalid calls continue to fail. Generated Go continues to use the
original argument expression. Rollback consists of removing the boundary
assignability hook and restoring the previous inference behavior.
