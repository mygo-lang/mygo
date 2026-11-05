## 1. Syntax and AST

- [x] 1.1 Add `raw:` parsing to parser2's inline-Go body, including a
  dedicated CST child that covers the full `raw: <type>` range. Verify a
  lossless test asserts the parsed CST text equals the source and the lowered
  AST has the expected raw type.
- [x] 1.2 Add the optional raw type to
  `ast2.ExprKind.InlineGoExpr` and update every constructor and pattern match
  in parser2 and ast2. Verify `go build ./...` succeeds and a parser2
  lowering test reads the raw type back from the AST.
- [x] 1.3 Preserve `raw:` through formatting. Verify `mygo fmt` on a block
  with `raw:` is idempotent, keeps comments and field order, and reparses to
  the same AST.
- [x] 1.4 Make the legacy parser reject a `raw:` block with a targeted
  unsupported-feature diagnostic instead of a generic parse failure. Verify a
  `--legacy sync` probe reports that diagnostic and existing no-`raw:`
  legacy sources still parse.

## 2. Inference and intermediate representation

- [x] 2.1 Carry the raw type through typeinference2 operand substitution,
  typed-AST reconstruction, and substitution application without changing the
  inferred MyGO result type. Verify a generic `go[Option[Ref[T]]]` block with
  `raw: Ref[T]` infers `Option[Ref[T]]` for an instantiated `T`.
- [x] 2.2 Carry the raw type on IR3's foreign expression and preserve it
  through lowering/printing. Verify an IR3 dump or lowering test contains the
  declared raw shape.
- [x] 2.3 Keep rich-error `Result` behavior intact in inference: a
  `go[Result[T, Diagnostic]]` block with no `raw:` must still infer and
  lower exactly as before. Verify the existing diagnostic/panic-boundary tests
  pass unchanged.

## 3. Codegen2 boundary plan

- [x] 3.1 Implement the structural `(target, raw)` plan with Identity,
  OptionRef, OptionTuple, ResultTuple, ResultLoneError, and Unsupported
  outcomes. Verify table-driven plan tests cover every supported pair, the
  identity case, and an unsupported pair.
- [x] 3.2 Implement the `Option[Ref[U]]` + `raw: Ref[U]` nil-check
  adaptation. Verify generated Go evaluates the snippet once, emits
  `Some[*U]` for a non-nil pointer and `None[*U]()` for nil, and a runtime
  probe observes both branches.
- [x] 3.3 Implement the `Option[U]` + `raw: (U, Bool)` comma-ok
  adaptation. Verify generated Go branches on the boolean and a runtime probe
  observes `Some` and `None`.
- [x] 3.4 Implement the `Result[U, Error|String]` + `raw: (U, Error)`
  adaptation. Verify the `Error` target passes the error through, the
  `String` target uses `err.Error()`, and success/failure runtime probes
  observe `Ok`/`Err`.
- [x] 3.5 Implement the `Result[(), Error|String]` + `raw: Error`
  lone-error adaptation. Verify nil becomes `Ok(())`, non-nil becomes
  `Err`, and the `String` target carries the error message.
- [x] 3.6 Route adapted inline Go through the value path in expression,
  let-binding, and return positions, while statement position keeps the raw
  Go statement and discards the value. Verify each position with a codegen
  test.
- [x] 3.7 Emit a diagnostic for an unsupported `(target, raw)` pair, naming
  both types and the source location. Verify
  `go[Option[Int]] { raw: String; ... }` and
  `go[Result[Data, Diagnostic]] { raw: (Data, Error); ... }` both fail with
  that diagnostic.
- [x] 3.8 Add the codegen3 ABI-aware `(target, raw)` plan and constructor
  selection using `GoTypeString` and `preludeCtorName`. Verify codegen2 and
  codegen3 select the same plan kind for every supported pair.
- [x] 3.9 Implement codegen3 `Option[Ref[U]]` + `raw: Ref[U]`: canonical
  identity (the pointer is used directly and nil is observed as `None`) and
  legacy nil-check `Some` / `None`. Verify both ABI modes with
  generated-code and runtime probes.
- [x] 3.10 Implement codegen3 `Option[U]` + `raw: (U, Bool)`,
  `Result[U, Error|String]` + `raw: (U, Error)`, and
  `Result[(), Error|String]` + `raw: Error`, using canonical
  `Option__Some` / `Option__None` / `Result__Ok` / `Result__Err` or
  legacy `Some` / `None` / `Ok` / `Err`. Verify constructor names under
  both ABIs and `err.Error()` for `String`.
- [x] 3.11 Reject statement-form `raw:`, statement-position `raw:`, and
  unsupported `(target, raw)` pairs in codegen2 and codegen3 with a located
  diagnostic. Verify statement-form, statement-position, unsupported-pair,
  and codegen3 diagnostic tests.

## 4. Compatibility and end-to-end behavior

- [x] 4.1 Add regression coverage for every existing inline-Go shape without
  `raw:`: native values (`Int`, `String`, `()`), direct
  `Some`/`None` construction, direct `Result` construction with a rich
  error, `go[(U, Bool)]`, and `go[(U, Error)]`. Verify the generated output
  is unchanged in behavior and all tests pass.
- [x] 4.2 Add an end-to-end probe shaped like the RSA key conversion from
  commune-backend: `go[Option[Ref[rsa.PublicKey]]] { raw: Ref[rsa.PublicKey];
  code: "..."; in key = key }`. Verify a successful assertion yields
  `Some`, a failed assertion yields `None`, and the package builds.
- [x] 4.3 Run the prelude tests and a prelude compile probe to confirm the
  existing `go[Option[T]]` / `go[Option[Ref[T]]]` helper-lambda blocks are
  untouched. Verify the prelude suite passes.

## 5. Documentation and validation

- [x] 5.1 Update `docs/compiler/inline-go.md` with the `raw:` syntax, the
  supported conversion table, the identity rule, the rich-error limitation,
  the expression-form restriction, and codegen3's canonical/legacy ABI
  behavior. Verify the examples match the implemented behavior.
- [x] 5.2 Regenerate self-hosted `zz_*.gen.go` files in dependency order:
  parser2, rebuild CLI, typeinference2, rebuild CLI, IR3, codegen2 and
  codegen3, compiler packages, rebuild CLI. Verify each sync with a no-test
  build and confirm no generated file was hand-edited.
- [x] 5.3 Run focused tests for parser2, formatter, typeinference2, codegen2,
  codegen3, IR3, and compiler. Verify they pass with a task-local writable
  `GOCACHE`.
- [x] 5.4 Run the full `go test ./...` suite. Verify it passes and report any
  environment-only failures separately.
- [x] 5.5 Validate the change with
  `openspec validate inline-go-raw-boundary --strict`. Verify it reports no
  errors.
