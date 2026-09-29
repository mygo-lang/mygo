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

An additional observation from the same investigation: `goTypeFields` records
the embedded field under the embedded type's own name, so the surface does
already contain an entry for `gorm.Model` as a field of `User`. The design
below therefore expands promotion at the *MyGO struct symbol table*, and does
not change the Go FFI collector's field collection.

## Goals / Non-Goals

**Goals:**

- Emit exported package-level `var`s into the FFI surface with a usable type
  string, and mirror the same in the hand-written `typeinference` loader.
- Make a field promoted from a Go type embedded in a MyGO struct resolve, at
  the embedded type's field type, and lower to a plain Go selector.

**Non-Goals:**

- No new FFI syntax. `embed T` parsing and its codegen (`codegen2/decls.mygo`
  already emits an anonymous field) are working and untouched.
- No change to `Ref[T]`, to interface method dispatch, or to the `(T, error)` /
  lone-`error` `Result` boundary rules.
- No multi-level promotion depth guarantees beyond what the chosen mechanism
  gives (see Decisions).
- No change to the Go FFI collector's struct-field collection.

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

### Promotion is resolved when building a MyGO struct's symbol table

**Decision:** Extend `structSymbolsInEnvAt` in `internal/mygo/typeinference2/env.mygo`
so that when it encounters a field whose name is the `embed` marker and whose
type resolves to an imported Go named struct, it also registers a
`StructField` for each exported field of that Go type - registered on the
embedding MyGO type, at the embedded type's field types.

This is the narrowest point that fixes it. The alternative - expanding
promotion inside `goTypeFields` - does not work, because the lookup that fails
is for a *MyGO* type name (`User`), whose symbols come from the MyGO struct
declaration, not from the Go package's type table. `goTypeFields` is only
consulted for `gorm.Model`-as-a-type lookups, which already work.

The embedded type's field list is available at this point: the loader already
collected it into the `GoTypeSignature` for `gorm.Model`, and
`goFieldsFromSigsInto` in `types.mygo` turns those entries into symbols. The
implementation reuses that lookup rather than re-walking `go/types`.

*Alternative considered:* resolve promotion lazily in the `findSymbol` miss
arm in `infer.mygo`, by walking the receiver's `embed` fields on demand.
Rejected as strictly more work per lookup and harder to keep consistent with
the eager symbol table; the table is built once per struct declaration.

### Only exported fields of the embedded type are registered

`structSymbolsInEnvAt` registers promoted entries for exported fields only,
mirroring Go's own promotion rules. Unexported embedded fields are invisible
across the package boundary anyway, and registering them would let MyGO code
reference something the generated Go cannot name.

*Known limitation:* this registers one level of promotion. A field promoted
from a type that is itself embedded two levels down is not covered. GORM's
actual shape (`User` embeds `gorm.Model` directly) does not need it, and the
recursive form is a natural follow-up if a real package requires it. This is
recorded rather than solved because no verified case demands it and recursion
through the same helper would also need cycle handling.

### Name collisions between own and promoted fields

If a MyGO struct declares `Id: Int` and also embeds a Go type that contributes
a promoted `Id`, both register a `StructField` for the same name. Go's own
rule is that the outer declaration wins. The symbol table is an association
list, so a later registration can shadow an earlier one on lookup; the
implementation must ensure own fields are registered after promoted ones (or
explicitly win the collision) so `findSymbol` sees the outer field first.
This is called out because it is a correctness detail that is easy to lose.

## Risks / Trade-offs

- [Symbol table growth] Registering every exported field of every embedded Go
  type enlarges the per-struct symbol list, and `findSymbol` scans it
  linearly. -> GORM's `gorm.Model` contributes 4 fields, so the growth is
  small in practice; revisit only if a pathological package shows up.
- [Collision ordering is implicit] Correctness of own-vs-promoted precedence
  depends on registration order in `structSymbolsInEnvAt` rather than on an
  explicit rule. -> Pin it with a regression test that embeds a Go type whose
  field name collides with a declared field.
- [Single-level promotion] Deeper embedding is not resolved. -> Documented as
  a known limitation in this design; GORM does not need it.
- [Loader parity drift] The bootstrap and hand-written loaders are separate
  code. -> Apply the same `var` change to both in the same change, so neither
  pipeline accepts a program the other rejects.
- [Silent widening of the FFI surface] More imported names could change
  inference for a program that previously relied on a name being unresolved.
  -> Low risk: the added names are real exported objects, and the existing
  `bootstrap-ffi-boundary` scenarios must continue to pass unchanged.

## Migration Plan

Purely additive compiler change. No data migration, no config change.
Rollback is reverting the two edits; previously compiling programs are
unaffected either way since the change only adds members that were dropped.

## Open Questions

- Whether `structSymbolsInEnv` is reachable from more than the one call site
  found, and whether registering promoted symbols there affects the
  `envWithStructFields` stub path. Neither changes the specs or the approach;
  resolve during implementation.
