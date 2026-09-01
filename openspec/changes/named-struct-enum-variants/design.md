## Context

MyGO's parser (`internal/mygo/parser/parser.y`) currently accepts only
tuple-style enum variant declarations (`Name(Type, ...)`) plus zero-argument
variants. The AST already stores fields as `[]ast.Field` (which has `Name`
and `Type`), so declaration syntax can be extended without an AST shape
change. Variants compile to Go structs whose fields are always `F0`, `F1`,
etc.

The self-hosted bootstrap compiler (`parser2`, `typeinference2`, `codegen2`)
is a separate implementation generated from MyGO sources. Any language
extension must be implemented in both compilers. The production compiler is
the reference implementation; the bootstrap compiler mirrors it.

## Goals / Non-Goals

**Goals:**

- Add named-struct enum variant declaration syntax.
- Add construction syntax `Enum.Variant { field: expr }`.
- Add struct-style switch patterns with field binding, shorthand, and `_`
  discard.
- Support mixing tuple and named-struct variants within one enum.
- Reject invalid or duplicate field names at compile time.
- Keep Go codegen output source-compatible for tuple variants.

**Non-Goals:**

- Removing or deprecating tuple variants.
- Adding default field values, optional fields, or builder APIs.
- Implementing this in the legacy parser only; both compilers must be updated.

## Decisions

### Reuse `ast.Field` for named enum variant fields

`ast.EnumVariant.Fields` is already `[]ast.Field`, matching `ast.StructDecl`.
No new AST node is needed for declarations, and existing type-inference code
that iterates `variant.Fields` works unchanged. Parser rules fill in `Name`
for named-struct variants and leave it empty for tuple variants.

### Add a new pattern variant instead of overloading `VariantPattern`

`VariantPattern` binds fields positionally via `Args []string`, which does not
model named fields. A new `StructVariantPattern` (with `Name string` and
`Fields []StructPatternField`) is added to the AST. This keeps validation,
type inference, and codegen explicit about which fields are being bound.

**Alternative considered:** Extend `VariantPattern` with optional field names.
Rejected because it makes invariant checking (tuple vs struct pattern) harder
to reason about at every use site.

### Generate named Go field names for named-struct variants

Current codegen emits `F0 Foo`, `F1 Bar` for every variant. For named-struct
variants, emit the declared field name (e.g. `radius float64`) so generated Go
is readable and direct field access is possible. Backends that already use
`variant.Fields[i]` order are unaffected.

### Struct literal construction reuses `translateStructLit`

Parsing `Enum.Variant { ... }` already produces `*ast.StructLitExpr` with
`TypeName == "Enum.Variant"`. The translator will recognize this compound name
as an enum-qualified variant, look up the variant's named fields, reorder
provided values into declaration order, and emit the variant's Go struct
literal. Unknown fields and duplicate fields are rejected with source
locations.

### Pattern matching lowers to the variant's generated Go struct

For a struct variant pattern `case Variant { a, b: c, _ }`, the generated if is:

```go
if v, ok := target.(Shape__Circle); ok {
    a := v.Radius      // shorthand: `a` binds field `a`
    c := v.B;          // explicit: `b: c`
    // body
}
```

`_` fields are not bound. Field lookup is by Go field name derived from the
MyGO field name.

## Risks / Trade-offs

- **Enum variant field names leak into generated Go structs.** A
  named-struct variant's field name is exported (uppercase) in generated Go.
  This is already the case with `F0`; the new code changes naming to be more
  descriptive but retains exact field-order semantics for tuple variants.

  → Keep tuple variant field names as `F0`, `F1` to preserve compatibility.

- **Two parallel implementations increase review surface.** The production
  and bootstrap compilers have duplicated logic. The parser changes are the
  same concept in both, but the AST and generator types differ.

  → Land production first, then mirror the same semantics in bootstrap,
  keeping each commit buildable.
