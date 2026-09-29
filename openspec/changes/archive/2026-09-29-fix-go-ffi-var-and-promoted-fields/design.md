## Context

See proposal.md - Why.

The Go FFI surface is collected by `bootstrapGoPackageInfoFromTypes` in
`internal/mygo/compiler/go_ffi_import.go`, which walks a `*types.Package` scope
and produces `GoFuncSignature`, `GoTypeSignature`, and `GoConstSignature`
entries for the self-hosted pipeline. Two things it does not emit:

- Package-level `*types.Var` objects. The third pass matches only
  `*types.Const`, so any other scope object falls through.
- Promoted fields. `goTypeFields` iterates `named.Underlying().(*types.Struct)`
  and keeps exported fields; an embedded field is itself exported and IS
  collected, but under the *embedded type's* name (`Model`), not the promoted
  field names it contributes (`ID`, `CreatedAt`).

Investigating the `embed` gap changed its location. The `GoTypeSignature`
field table is not what breaks promotion:

- `gorm.Model` is itself an imported Go type, so `m.ID` on a bare
  `gorm.Model` value already resolves today (verified). The embedded type's
  own fields are reachable.
- The failure is specific to a **MyGO** struct with an `embed` field. The
  symbol table for a MyGO struct is built by `structSymbolsInEnvAt` in
  `internal/mygo/typeinference2/env.mygo`, which registers one `StructField`
  symbol per `ast2.Field`. For an `embed` field the legacy AST stores the
  literal string `"embed"` in the name slot (see `structFieldName` in
  `internal/mygo/parser2/syntax_lower_declarations.mygo`), so the symbol table
  ends up with a `StructField("User", "embed", gorm.Model)` entry and nothing
  for `ID`.
- Field selection on a TCon receiver goes through `findSymbol(typeName, field,
  state.SymbolIndex)` in `internal/mygo/typeinference2/infer.mygo`; the `None`
  arm produces the observed `unknown field User.ID`.

Note also that `envWithStructFields` is a no-op stub (`env`), with the real
work in the sibling `structSymbolsInEnv`. The two functions are adjacent and
the stub looks vestigial, but that is out of scope here.

`goTypeFields` records the embedded field under the embedded type's own name
(`f.Name()` on an anonymous field *is* the type name), so the surface already
contains `Model` as a field of `User`. That entry is not incidental — it is the
second access path described in Goals.

### Two further gaps, found while scoping multi-level promotion

The single-level implementation above was verified to work for the GORM
shape, but three things it does not do became load-bearing once multi-level
promotion was scoped:

1. **The Go FFI side does not record embedded identity.** `goTypeFields`
   flattens every exported field, including anonymous ones, into
   `GoFieldSignature{Name, Type}`. An embedded field is therefore
   indistinguishable from a field that merely happens to be named after a
   type, so there is nothing to recurse *into*. Multi-level promotion over Go
   types cannot be written against this shape.
2. **Nothing is promoted from a Go type embedded in a MyGO struct beyond one
   hop**, because `promoteGoTypeFields` matches a single type name and stops.
3. **Methods are not promoted at all.** `goSymbolsFromTypes` registers
   `GoMethod` under the owning Go type's name only, and `inferOrdinaryField`
   looks methods up by the receiver's own type name. `u.Save()` on a
   `struct User` containing `embed gorm.Model` does not resolve today, even at
   one level. Go promotes methods and fields by the same rules, so leaving
   methods out would make any "Go promotion semantics" claim false.

MyGO-struct-embedded-in-MyGO-struct is likewise unhandled: `myGoStructFieldSymbols`
in `types.mygo` registers declared fields only and never inspects the `embed`
marker, so `struct B / embed A` does not currently expose `A`'s members on `B`
at all.

## Goals / Non-Goals

**Goals:**

- Emit exported package-level `var`s into the FFI surface with a usable type
  string, and mirror the same in the hand-written `typeinference` loader.
- Implement Go's promotion semantics for embedded members, for MyGO and Go
  embedded types, at any depth: transitive promotion of exported fields and
  methods, shallowest-wins, equal-depth ambiguity reported, and the embedded
  field also addressable by its type name (`b.A` / `u.Model`).

**Non-Goals:**

- No new FFI syntax. `embed T` parsing and its codegen (`codegen2/decls.mygo`
  already emits an anonymous field) are working and untouched.
- No change to `Ref[T]`, to interface method dispatch, or to the `(T, error)` /
  lone-`error` `Result` boundary rules.
- No promotion of unexported members, and no Go embeddability/type-set rules
  beyond what the collector already exposes. Promotion is restricted to
  members Go would also promote.
- No change to the Go FFI collector's *named* field collection: named fields
  keep flowing through `GoFieldSignature` unchanged. Embedded identity is
  added alongside them, not instead of them.

## Decisions

### Package-level `var` reuses the existing constant channel

**Decision:** In the third pass, match `*types.Var` alongside `*types.Const`
and emit it as a `GoConstSignature` entry.

`GoConstSignature` is just `{Name, Type}` - a name and a rendered type string -
and typeinference2 already seeds those into the environment as values. A `var`
carries a fully typed `types.Type` (never untyped), so `types.Default` is a
no-op for it and the existing type-string rendering applies unchanged.

*Alternative considered:* add a distinct `GoVarSignature` list and a second
seeding path. Rejected - it doubles the surface for zero behavioral gain, and
the two are indistinguishable once they are name+type pairs in the env.

*Alternative considered:* a narrower fix that hardcodes GORM's error vars.
Rejected - the gap is general (`os.ErrNotExist`, `io.EOF`-style sentinels);
package-specific handling would not survive the next library.

The hand-written loader in `internal/mygo/typeinference/go_imports.go` has the
same gap in its own `*types.Const`-only loop. It gets the same treatment so the
two pipelines do not diverge on programs that use either.

### [Superseded] Promotion resolved eagerly in the symbol table

The first implementation of this change extended `structSymbolsInEnvAt` in
`internal/mygo/typeinference2/env.mygo` to *prepend* one `StructField` per
exported field of an embedded Go type onto the embedding MyGO type. That fixed
`u.ID` at one level and is what `tasks.md` 3.1-3.3 shipped.

**This decision is superseded.** Eager flattening cannot express the semantics
the spec now requires:

- **Depth is unrepresentable.** Every promoted member lands in the same
  `TypeName::Field` key space as a declared field, so once flattened, depth-1
  and depth-2 candidates are indistinguishable and shallowest-wins cannot be
  evaluated.
- **Ambiguity is unrepresentable.** Go rejects `a.b` as ambiguous when two
  same-depth embeds contribute `b`. `findSymbol` returns the first entry and
  `symbolIndexFromSlice` is first-wins, so the compiler would silently pick one.
- **Only fields.** The flattening is wired to `GoFieldSignature`; methods never
  enter it.
- **One hop.** `promoteGoTypeFields` matches a single type name and stops.
- **MyGO-to-MyGO is absent.** `structSymbolsInEnvAt` is the only embed-aware
  path, and it only recognises embedded *Go* types.

The entry it *did* get right is kept: the embedded field registered under the
embedded type's own name (`Model` as a field of `User`) is exactly what the
qualified path `u.Model` needs, and it is reused rather than removed.

### Promotion is resolved at selector time, with depth and provenance

**Decision:** Keep the symbol table as the source of *declared* members, and
resolve promoted members on a lookup miss by walking the receiver's embedding
chain on demand, collecting candidates annotated with `(depth, originType)`.

`inferOrdinaryField` in `internal/mygo/typeinference2/infer.mygo` already has a
miss arm that produces `unknown field <T>.<f>`. That is the single point every
selector on a TCon receiver flows through, so the promotion resolver is
installed there and applies uniformly to fields and methods, MyGO and Go
receivers.

Resolution:

1. Direct hit in `SymbolIndex` (a declared field or method) wins immediately —
   this is the outer-declaration-wins rule, and it falls out of the existing
   precedence without any extra bookkeeping.
2. Otherwise walk the receiver's embedded types breadth-first, recording each
   candidate's depth and the embedded type it came from. A visited set keyed by
   type name terminates cycles (`struct A` embedding a type that embeds `A`).
3. Keep only the minimum-depth candidates.
4. If exactly one remains, use it. If several remain and do not all share the
   same origin type, report an ambiguous selector naming the member. If they do
   share an origin, they are the same Go member reached twice and it is used.

*Alternative considered:* keep flattening but tag each entry with a depth, and
resolve ties at index-build time. Rejected: it still cannot report ambiguity
(the index is a single-slot `Map[String, Symbol]`; representing a tie needs a
multi-value entry, which changes `findSymbol`'s contract for every caller), and
it eagerly multiplies the symbol list by the embedding chain's breadth.

*Alternative considered:* resolve promotion once per struct after the table is
built, as a separate promotion table consulted on miss. This is closer to eager
and avoids repeated chain walks, but it needs its own index keyed by
`(type, member)` with a depth-bearing value, duplicating `SymbolIndex`. Deferred
unless the on-demand walk shows up in profiles; the resolver is written against
one interface so it can be memoized later without touching callers.

### Embedded identity is recorded explicitly, and named-field collection is unchanged

**Decision:** Add an embedded-type list to `GoTypeSignature`, populated by both
loaders from `f.Anonymous()`, alongside the existing `Fields` list. `goTypeFields`
continues to emit every exported field — including anonymous ones, under their
own names — so `u.Model` keeps working exactly as it does today.

The new list is what the resolver recurses through. It is additive: a struct
with no embedded fields produces an empty list and the resolver behaves as
before, so the bare-Go-struct path (`m.ID` on a `gorm.Model` value) is
untouched.

*Alternative considered:* drop anonymous fields from `Fields` and rely solely
on the new embedded list for the qualified path. Rejected: the qualified path
`u.Model` needs the name registered under the *outer* type, which the embedded
list does not itself provide, and reusing the existing entry avoids a second
mechanism for the same name.

*Consequence:* `GoTypeSignature` is a cross-package struct consumed by
`internal/mygo/compiler`, `internal/mygo/typeinference`, and
`internal/mygo/typeinference2`, and its literal construction appears in tests.
Every construction site must set the new field.

### The qualified path resolves to the embedded type and never competes

`b.A` binds to the embedded type `A`; `b.A.F1` then resolves `F1` against `A`'s
own members, exactly as an explicit Go selector does. Because the first step
lands on the embedded type itself, the second step is an ordinary lookup and
the two access paths cannot diverge in type.

This also means the qualified path needs no resolver work: it is satisfied by
the embedded field entry that already exists. It is explicitly excluded from
the ambiguity rules, matching Go, where naming the embedded type is the
standard disambiguation.

*Consequence:* for MyGO embeds, `struct B / embed A` must register
`StructField("B", "A", A)`. The legacy AST records the literal `"embed"`
marker in the field-name slot, so the embedded type's name must be recovered
from the field's type expression rather than read off the name slot. Today
`structSymbolsInEnvAt` registers the marker name `"embed"` itself, which is a
name MyGO code can never use.

### Only exported members are promoted, and only those Go promotes

Promotion registers exported fields and methods only. Unexported embedded
members are invisible across the package boundary, and registering them would
let MyGO code reference something the generated Go cannot name.

The collector already filters on `f.Exported()`, so the embedded list inherits
that filter. Go's remaining embeddability rules (e.g. embedding a type with
field or method conflicts) are out of scope: the goal is that programs Go
accepts also infer here, not that every Go embeddability error is reproduced.

### Ambiguity and precedence are decided by the resolver, not by registration order

The previous design made own-vs-promoted correctness depend on registration
order in `structSymbolsInEnvAt` — a hidden coupling that the one-level tests
only pinned indirectly. The resolver removes it: declared members live in
`SymbolIndex` and are checked first by construction, so an outer declaration
always shadows a promoted member without any ordering requirement. Depth ties
are then resolved explicitly by the rules above rather than by which entry a
map happened to keep.

## Risks / Trade-offs

- [Promotion cost per lookup] The resolver walks the embedding chain on every
  selector miss, and misses are common (any unknown identifier, any field on a
  non-struct receiver). -> The walk exits immediately when the receiver has no
  embedded types, which is the overwhelming majority of lookups; only a struct
  with embeds pays. Memoize only if this shows up in profiles.
- [Cycle handling] A self-referential embedding (`struct A` embedding a type
  that embeds `A`) would loop without a guard. -> A visited set keyed by type
  name is required, not optional, and a regression test must exercise a cycle.
- [New ambiguous-selector error] Programs that previously failed with
  `unknown field T.X` may now fail with an ambiguous-selector error, and
  programs that previously resolved to one arbitrary embed now fail. -> Both
  are strictly closer to Go; neither is a regression against a program Go
  accepts. Callers matching on the `unknown field` text need review.
- [Cross-package struct change] `GoTypeSignature` gains a field, and its
  literals are constructed in `internal/mygo/compiler`,
  `internal/mygo/typeinference`, and tests. -> The compiler will not catch an
  un-updated keyed literal, so grep every construction site and run both
  pipelines' tests.
- [Method promotion is new surface] Resolving methods on MyGO struct receivers
  can newly succeed where it previously reported an unknown method. -> Low
  risk: the members are real exported Go methods, and the existing
  `bootstrap-ffi-boundary` scenarios must continue to pass unchanged.
- [Loader parity drift] The bootstrap and hand-written loaders are separate
  code. -> Apply the same `var` change to both in the same change, so neither
  pipeline accepts a program the other rejects.
- [Silent widening of the FFI surface] More imported names could change
  inference for a program that previously relied on a name being unresolved.
  -> Low risk: the added names are real exported objects, and the existing
  `bootstrap-ffi-boundary` scenarios must continue to pass unchanged.

## Migration Plan

Purely additive compiler change. No data migration, no config change.
Rollback is reverting the edits; previously compiling programs are unaffected
either way since the change only adds members that were dropped.

One intentional behaviour change: a selector that Go reports as ambiguous now
fails here with an ambiguous-selector error instead of resolving to whichever
embed happened to be found. No program Go accepts changes behaviour.

## Open Questions

- Where the resolver's embedding-chain data comes from for a receiver whose
  type is a MyGO struct declared in the same package (the `ast2.Field` embed
  marker) versus an imported one. Expected to be a lookup in `PkgInfo` for the
  former and in the Go package entry for the latter; to be confirmed against
  `inferOrdinaryField`'s available state during implementation.
