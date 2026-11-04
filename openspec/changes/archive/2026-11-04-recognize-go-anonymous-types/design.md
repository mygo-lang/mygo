## Context

See `proposal.md` for the observable failure. The current FFI path serializes
`go/types` signatures into strings, parses those strings back into `MonoType`
values, and uses a Go-side `types.AssignableTo` query only after ordinary
unification fails. Named imported types have a canonical package path and a
package-scope name, so that query can resolve both sides. Anonymous types have
neither, which is why the current `interface{...}` spelling is split like a
`pkg.Type` name.

The Go loader already retains complete `*types.Package` graphs in the
assignability registry. The design should reuse those type objects rather than
duplicating Go's interface method-set and embedding rules in MyGO.

## Goals / Non-Goals

**Goals:**

- Give every anonymous type that appears in an FFI signature a deterministic,
  collision-resistant internal identity tied to its real `go/types.Type`.
- Resolve anonymous syntax before package-qualified-name splitting, including
  when it is nested inside another signature type.
- Use Go assignability for anonymous interface arguments and reject
  incompatible method sets.
- Keep anonymous identities out of user-visible MyGO syntax and generated Go
  type declarations.

**Non-Goals:**

- Adding MyGO syntax for writing anonymous Go types.
- Supporting field selection, struct literals, or direct method dispatch on
  anonymous structs/interfaces beyond what is explicitly required by the
  specs.
- Replacing `go/types` assignability with a MyGO reimplementation of Go's
  interface method-set rules.
- Changing named Go type resolution, generated ABI shapes, or ordinary MyGO
  unification.

## Decisions

### Use opaque Go type identities for anonymous signature types

The loader will register anonymous types from parameter, result, field, embed,
and nested composite positions in a process-wide registry. Each entry retains
the actual `types.Type` and a deterministic key derived from a full-path
rendering of the type. The serialized FFI spelling carries that key in a
reserved internal namespace, with a human-readable spelling retained for
diagnostics. The parser recognizes the namespace and produces an opaque type
identity instead of splitting the spelling at a dot.

This keeps `MonoType` and codegen from having to model every anonymous Go
shape. It also avoids implementing Go interface identity rules twice. The
alternative of adding a complete structural anonymous-interface variant was
rejected because it would require a broad audit of unification, substitution,
codegen, metadata, and self-hosted generated code for little additional value.

### Reuse the existing Go assignability query

`goBoundaryArgumentAssignable` will recognize the opaque anonymous source
identity and call the Go service with the source registry key and the target
named type. The service will resolve the source `types.Type` from the registry
and the target from the existing package registry, then call
`types.AssignableTo`. The entry must be registered before inference starts, and
the target package graph must be registered as it is today. The FFI loader
must request dependency type information so transitively imported target
packages are complete; registering only root packages is insufficient for an
anonymous interface that embeds a type from another package.

This preserves embedded interface methods, unexported method identity, pointer
method sets, and generic instantiations according to the Go toolchain rather
than a hand-written approximation.

### Recognize balanced anonymous syntax before qualified-name resolution

The signature parser will add an early branch for `interface{` and
`struct{` with balanced-brace scanning. Existing branches for arrays, slices,
maps, pointers, channels, functions, and `interface{}` remain in place. Nested
anonymous types are handled by the existing recursive descent: an anonymous
interface inside `[]T`, `map[K]V`, a function signature, or another anonymous
type reaches the same branch with the inner spelling.

The parser must not use the first dot as a package separator until all
anonymous syntactic forms have been eliminated. Unsupported anonymous shapes
produce a diagnostic naming the Go type rather than a synthetic package path.

### Keep anonymous identities inference-only

Anonymous identities are not valid MyGO type names. Codegen must continue to
derive concrete Go types from expressions and must not emit the internal key in
a generated type position. If an operation would require naming the anonymous
type explicitly and no existing expression-based path supplies it, the compiler
fails with a clear diagnostic. This preserves the existing generated Go ABI.

## Risks / Trade-offs

- **Key stability across compiler processes** -> derive keys from deterministic
  full-path `types.TypeString` output and keep the registry populated from the
  same package load that produces the signatures.
- **Key collisions between package-local anonymous types** -> include full
  import paths and the complete structural spelling in the key, especially for
  unexported interface methods and struct tags.
- **Registry growth in LSP or long-lived processes** -> reuse the existing
  package registry lifecycle and overwrite equivalent keys deterministically;
  keys are bounded by loaded Go type graphs.
- **Anonymous type identity leaking into generated Go** -> audit codegen paths
  that render `MonoType` and reject internal anonymous identities in type
  positions that cannot be expressed in Go.
- **Nested parsing regressions** -> add fixtures for anonymous interfaces
  nested in slices, maps, pointers, channels, and function signatures, and
  retain the existing named-type and `interface{}` tests.
- **Over-broad assignability** -> keep the Go query behind the FFI call-boundary
  check; ordinary MyGO unification remains unchanged.

## Migration Plan

No data migration is required. The change is internal to the bootstrap FFI
pipeline and can be rolled back by disabling anonymous-type registration and
recognition while retaining the existing named-type path. Existing generated
files are regenerated only after the compiler tests pass; consumer code such as
`commune-backend/internal/content/rich.mygo` should compile unchanged.
