## Context

The default bootstrap pipeline parses inline Go with parser2 into
`ast2.ExprKind.InlineGoExpr`, infers the declared result type, and lowers it
through codegen2. Codegen2 already distinguishes two boundary cases:

- `go[(U, Bool)]` in an `Option[U]` context is lowered to a comma-ok
  `Some` / `None` choice.
- `go[(U, Error)]` in a `Result[U, E]` context is lowered to `Ok` /
  `Err`.

Everything else uses the declared type as the raw Go result type. That is why
`go[Option[T]]` and `go[Result[T, E]]` currently require the snippet to
construct the prelude ADT directly. The legacy `--legacy` code generator has
additional textual heuristics, but the default backend does not.

Two existing constraints shape the design:

1. Direct ADT construction is still important. Prelude blocks such as
   `go[Option[T]]` pass `someT` / `noneT` helper lambdas, and compiler
   blocks such as `go[Result[T, Diagnostic]]` call Go helpers that return a
   rich-error `Result`. These blocks must keep their current behavior.
2. An `error` cannot be converted back into an arbitrary MyGO payload such as
   `ast2.Diagnostic`. The FFI error conversion only supports `Error`
   (pass the Go error through) and `String` (`err.Error()`).

Codegen3 uses the same structural target but a different ABI. Under the
canonical prelude, `Option[Ref[U]]` is already the Go type `*U`; under a
legacy prelude it is the interface value `Option[*U]`. Canonical
`Option[U]` and `Result[U, E]` are tagged values constructed through the
`Option__Some` / `Option__None` / `Result__Ok` / `Result__Err` helpers,
while the legacy prelude uses `Some` / `None` / `Ok` / `Err`.
Codegen3's current inline Go path only checks compatibility and emits the raw
snippet, so the new plan must select the correct representation before
emitting.

## Goals / Non-Goals

**Goals:**

- Make the raw Go result shape explicit and separate from the MyGO result type.
- Support nilable-ref, comma-ok, error-tuple, and lone-error adaptations in
  codegen2 and codegen3.
- Emit codegen3's selected canonical or legacy ABI representation using the
  same constructor-selection rules as its existing FFI and generic boundaries.
- Keep both syntax and generated behavior unchanged when `raw:` is omitted.
- Reject unsupported pairs instead of producing incompatible Go or guessing a
  conversion from source text.

**Non-Goals:**

- No heuristic or type-directed detection of whether a snippet constructs an
  ADT. `raw:` is the explicit contract.
- No unboxing or ABI change for `Option` / `Result` in either backend;
  codegen3 uses its existing canonical or legacy representation.
- No conversion for rich error payloads from a Go `error`.
- No new `--legacy` adaptation. Existing no-`raw:` blocks continue to work
  under `--legacy`; a `raw:` block that requires adaptation is rejected.
- No statement-form `raw:`. Only a single Go expression can declare
  `raw:`.
- No change to Go FFI signatures or their existing boundary semantics.

## Decisions

### 1. `raw:` is a go-block field, not a keyword

The block grammar already distinguishes fields by a leading identifier and a
colon. `raw` SHALL be recognized only while parsing a go-block body, so it
remains an ordinary identifier everywhere else. Parser2 gains an
`InlineGoRawType` CST node (or an equivalent typed child) whose children cover
the full `raw: <type>` byte range. Lossless CST tests shall include a block
with `raw:`.

Alternative considered: a new lexer keyword. Rejected because it would reserve
`raw` globally and require changes in every parser and syntax highlighter for
a local field name.

### 2. AST carries an optional raw type

`ast2.ExprKind.InlineGoExpr` gains `RawType: Option[ast2.TypeExpr]` (the
exact field position is an implementation choice, but the tuple-arity change
must be applied consistently). Every pattern match on `InlineGoExpr` must be
updated, including inference, operand substitution, typed-AST reconstruction,
codegen2, IR3 lowering, and tests.

Alternative considered: a reserved `type raw = ...` operand. Rejected because
it collides with a user-visible type operand named `raw`, is not reflected in
the AST as a result shape, and would make the feature depend on operand
substitution order.

### 3. Adaptation is selected by a structural `(target, raw)` plan

The compiler computes a small boundary plan from the declared result type
`T` and the optional raw type `R`:

| Target `T` | Raw `R` | Adaptation |
| --- | --- | --- |
| any `T` | absent | Existing behavior (raw shape is `T`) |
| `T` | `T` | Identity |
| `Option[Ref[U]]` | `Ref[U]` | Nil check into `None` / `Some` |
| `Option[U]` | `(U, Bool)` | Comma-ok into `None` / `Some` |
| `Result[U, Error]` or `Result[U, String]` | `(U, Error)` | `Ok` / `Err`; `String` uses `err.Error()` |
| `Result[(), Error]` or `Result[(), String]` | `Error` | `Ok(())` / `Err`; `String` uses `err.Error()` |
| anything else | anything else | Unsupported diagnostic |

The plan is computed after type-parameter substitution, so `raw: Ref[T]`
inside a generic function resolves `T` from the enclosing environment.
`raw:` is resolved in the same type environment as the type in `go[T]`;
block-local `type` operands are code-string substitutions and do not
participate in the plan.

Alternative considered: infer the raw shape from the snippet with `go/types`
or textual constructor detection. Rejected for this change because opaque Go
helpers and rich-error Results cannot be classified without a full generated
package type environment, and because the user explicitly chose an explicit
marker.

### 4. The plan lowers through the existing boundary helpers

`translateInlineGoBoundary` already emits one-time `:= ` bindings and a
branch into `Some` / `None` or `Ok` / `Err`. It gains:

- an `OptionRef` kind that assigns the raw pointer once and branches on
  `== nil`;
- a `ResultLoneError` kind that assigns only the error and synthesizes the
  unit payload;
- an explicit `Identity` path that bypasses adaptation.

Expression, let-binding, and return positions route through the value path when
a plan requires adaptation. Statement position continues to emit the raw Go
statement and discards the value; it does not construct an unused temporary.

The error type conversion mirrors the Go FFI lowering: an `Error` target
passes the `error` value directly, while a `String` target uses
`err.Error()`. No other error payload is synthesized.

Codegen3 uses the same plan with ABI-aware emission. Its inline Go expression
carries the optional raw type through IR3; when `raw:` is absent, the
existing compatibility path remains in charge. When `raw:` is present,
codegen3 computes the plan and bypasses the old inline-Go compatibility check,
because the snippet is now explicitly declared to return the raw shape rather
than the ADT.

For emission, codegen3 uses `ir3.GoTypeString` for the target and raw Go
types and `preludeCtorName` for constructor names. This gives two ABI modes:

- Canonical prelude: `Option[Ref[U]]` renders as `*U`, so
  `raw: Ref[U]` is an identity. Nil is already `None`; non-nil is already
  `Some` for pattern matching and no wrapper is emitted.
- Legacy prelude: `Option[Ref[U]]` renders as `Option[*U]`; a nil-check
  function literal constructs `Some[*U]` or `None[*U]`.

For `Option[U]` with `raw: (U, Bool)`, codegen3 emits a function literal
that binds the value and boolean once and returns the ABI's
`Option__Some[U]` / `Option__None[U]` or `Some[U]` / `None[U]`.
For `Result[U, Error|String]` with `raw: (U, Error)`, it binds the value
and error once, converts a `String` error with `err.Error()`, and returns
the ABI's `Result__Ok` / `Result__Err` or `Ok` / `Err`. The lone-error
case uses the same constructor selection with a unit payload. The existing
`ffi_result.mygo` wrapper only covers `Result[T, Error]`, so the raw
Result adaptation needs its own `String`-aware path.

Codegen3 emits the wrapper through `goast.FuncLitFromStmts`, matching the
existing `optionRefToGeneric` and FFI Result helpers. A raw declaration on a
statement-form inline Go body is rejected; expression-form blocks (including
a function-literal call) are the only supported shape.

### 5. `raw:` takes precedence over existing tuple inference

When `raw:` is present, the explicit plan controls lowering. When it is
absent, the existing `inlineGoBoundaryKind` tuple behavior remains in charge,
so `go[(U, Bool)]` and `go[(U, Error)]` continue to behave exactly as
they do today. This is the compatibility boundary for existing sources.

### 6. Legacy and codegen3 behavior

The legacy parser and code generator are not required to implement `raw:`.
They must not silently misinterpret it: the legacy parser may reject a
`raw:` block with a targeted message, and it must not accept the field and
ignore it. The `--legacy` code generator keeps supporting no-`raw:` blocks
unchanged.

Codegen3 implements the plan from Decision 4 rather than rejecting it. The
canonical-versus-legacy constructor selection must use the same
canonical-prelude predicate as `GoTypeString` and `preludeCtorName`, so an
`Option[Ref[U]]` identity never allocates an interface wrapper under the
canonical ABI. Unsupported pairs still produce the structural diagnostic.

### 7. Raw declarations are expression-form only

`raw:` is a contract about the single value produced by an inline Go
expression. A block whose body is a statement list cannot be wrapped without
reinterpreting its `return` statements, so it is rejected with a diagnostic.
Expression-form blocks, including a function-literal call, are wrapped in an
IIFE. Statement-position inline Go without `raw:` is unchanged; statement
position with `raw:` is rejected by the same expression-form check.

### 8. Formatter and lossless parsing

Inline Go bodies are currently rendered from preserved CST text, so the
formatter needs no field-specific rewriting: a `raw:` field is preserved in
place. The parser must still include the field in the CST so spans, comments,
and lossless assertions remain correct. Add a formatter idempotence test and a
lossless-CST test for `raw:`.

### 9. Bootstrap order

Because parser2, typeinference2, IR3, codegen2, and codegen3 are self-hosted,
regeneration must proceed in dependency order with a writable `GOCACHE`:

1. Update and sync parser2.
2. Rebuild the CLI.
3. Update and sync typeinference2.
4. Rebuild the CLI.
5. Update and sync IR3.
6. Update and sync codegen2 and codegen3 (and any compiler package that
   pattern-matches `InlineGoExpr`).
7. Rebuild the CLI.

Never hand-edit generated `zz_*.gen.go` files. Run a no-test build after
each risky sync, then focused tests, then the full suite.

## Risks / Trade-offs

- **AST arity churn** -> the optional field changes every `InlineGoExpr`
  pattern. Mitigation: make the syntax/AST change first, let the compiler
  enumerate all matches, and add focused parser/inference/codegen tests before
  behavior changes.

- **Existing no-`raw:` behavior regresses** -> the identity path and the
  existing tuple path must remain distinct. Mitigation: regression scenarios
  for direct `Option` construction, rich-error `Result` helpers, `go[Int]`,
  and both tuple forms.

- **Silent backend disagreement** -> a backend might ignore `raw:` and emit
  incompatible Go. Mitigation: codegen2 and codegen3 must adapt; legacy must
  reject; add canonical and legacy codegen3 tests plus a legacy rejection
  test.

- **Codegen3 ABI mismatch** -> canonical and legacy preludes use different
  representations and constructor names. Mitigation: derive both from
  `GoTypeString` and `preludeCtorName`, and test `Option[Ref[U]]`,
  `Option[U]`, and `Result` under both ABI modes.

- **Statement-form raw blocks** -> wrapping a statement list could change
  `return` semantics. Mitigation: reject statement-form `raw:` early with
  an expression-form diagnostic.

- **Lossy CST spans** -> a new field node can break source reconstruction.
  Mitigation: lossless test asserting the parsed CST text equals the source
  and that comments around `raw:` survive formatting.

- **Rich-error Result confusion** -> a user may expect `raw: (T, Error)` to
  produce `Result[T, Diagnostic]`. Mitigation: the structural plan rejects
  that pair and the diagnostic names both types; direct construction remains
  the documented escape.

- **Bootstrap ordering** -> a new codegen2/codegen3 feature cannot compile a
  prelude that uses it until the new CLI exists. Mitigation: the feature is
  additive and no existing prelude source needs to change; sync in the order
  above and keep `raw:` out of bootstrap-critical sources until the new CLI
  is built.

## Migration Plan

`raw:` is additive. Existing sources need no migration and are expected to
generate the same output under codegen2, codegen3, and `--legacy`. New
sources opt in by adding `raw:` when the snippet returns the canonical raw
shape; codegen3 then emits the representation required by its selected
prelude ABI. Rollback is removing the field and the optional AST field; there
is no persistent data or ABI migration.
