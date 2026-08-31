## Context

See [proposal.md](proposal.md) for motivation and the two capability specs for
the behavioral contract. The production pipeline uses a Go yacc lexer/parser,
Go AST, inference, and code generation. The bootstrap pipeline uses parser2,
ast2, typeinference2, and codegen2 written primarily in MyGO. They therefore
need separate implementation work, but must converge on source behavior.

The current documentation also has an internal raw-string contradiction. Both
parsers already preserve multiline backtick content, and the production parser
has a test that covers it. Triple-quoted strings also retain their enclosed
content in both pipelines; this behavior is required by existing inline Go
code fields.

## Goals / Non-Goals

**Goals:**

- Establish executable behavioral parity for the affected source constructs.
- Keep the implementation changes for production and bootstrap independently
  reviewable and cherry-pickable.
- Make bootstrap workflow behavior match normal CLI expectations where that
  behavior is observable to users and tools.
- Resolve the string-literal contract without changing raw-string preservation.

**Non-Goals:**

- Replacing either compiler architecture or sharing their AST implementations.
- Rewriting unrelated inference, FFI, tail-call, or import-resolution logic.
- Guaranteeing byte-identical generated Go between pipelines.
- Adding loop forms other than the existing `while` construct.

## Decisions

### The specification remains the contract, with one raw-string correction

The shared contract is `docs/spec.md`; bootstrap implementation limitations do
not redefine it. The raw-string same-line restriction is an exception because
it contradicts the observed behavior and tests of both pipelines. The spec
will state that raw strings can span lines and preserve their contents.

Triple-quoted strings will preserve their enclosed content verbatim, matching
the established implementation and preserving inline Go source fragments.
The specification will distinguish them from ordinary double-quoted strings,
which retain escape processing. Decoding triple-quoted escapes was considered
but rejected because it breaks inline Go fragments such as `r == '\\n'` and
would require a broad source migration.

### Add loop control as a language feature in both lanes

`break` and `continue` become part of the source language rather than a
bootstrap-only extension. Each parser will reserve the keywords, construct its
native statement representation, and validate loop nesting before Go lowering.
The generated code will use the target language's nearest-loop semantics.

Rejecting the bootstrap extension was considered but rejected because loop
control is useful, already partially represented in ast2/codegen2, and the
user has selected full production support.

### Preserve patterns end-to-end in bootstrap tuple binding

Bootstrap tuple binding will move from a flat list of names to a recursive
binding-pattern representation. It may reuse ast2's pattern shape only if its
wildcard and binding semantics can be made unambiguous; otherwise a dedicated
binding-pattern enum should mirror the production AST. Parser, expression-ID
assignment, inference, and lowering will migrate together. Lowering must bind
from one temporary value so nested destructuring does not repeat effects.

Keeping the flat name slice and adding ad-hoc parser expansion was considered
but rejected because it cannot faithfully represent nesting or discard slots.

### Share switch branch semantics, not implementation structure

codegen2 will extract or align the pattern-to-branch logic used by value and
statement switch lowering, so all supported pattern forms work in both
contexts. The production code generator already has analogous statement-form
logic and will gain tests rather than be structurally coupled to codegen2.

### Treat bootstrap packages as main and external test units

Bootstrap orchestration will classify source files by their declared package
name before inference. A `*_test.mygo` file declaring `package name` remains
in the main unit and only receives the normal Go-recognized test filename.
Only a file declaring `package name_test` forms a separate inference and
generation unit.

The main unit is inferred first. Its exported declarations are then supplied as
the external environment while inferring the external test unit. The external
unit receives a synthesized Go dot import of the main package through codegen
input metadata. This is deliberately not an AST `ImportDecl`: it is not source
syntax, must not be inspected as a MyGO dependency, and must not trigger Go FFI
signature loading before the main package has been generated.

Treating all source files as a single package was rejected because Go cannot
build mixed package names in one directory and external tests need the main
package's exported surface. Classifying solely from a `*_test.mygo` filename
was rejected because it incorrectly turns internal `package name` tests into
external packages and causes a self-import.

### Make prelude policy explicit and reusable

Bootstrap state/options will carry a no-prelude policy from the CLI/API to
dependency traversal and inference. Convenience source-generation APIs will
use the same default prelude-loading boundary as package compilation, while
retaining an explicit path for prelude source itself. This removes the current
API-only behavior split.

### Use differential fixtures as the parity guard

Add focused fixtures that compile through both pipelines and assert acceptance,
diagnostic category/location, generated Go parseability, and runtime behavior
where appropriate. Pipeline-specific unit tests remain the fastest way to
localize failures; differential tests prevent future drift.

## Risks / Trade-offs

- [Triple-string semantics are less familiar than ordinary strings] → State
  the distinction explicitly in the specification and test verbatim content
  in both normal literals and inline Go fields.
- [ast2 tuple-pattern migration touches generated bootstrap artifacts] → Make
  the AST change atomic within the bootstrap-only commit and regenerate/check
  every affected `.gen.go` file in that commit.
- [External test-package imports depend on module resolution] → Reuse the
  existing resolver and add temporary-module end-to-end tests using a local
  `replace` directive.
- [Loop-control lowering can emit invalid Go outside loops] → Track loop depth
  during validation/inference and test source-position diagnostics before
  invoking Go build.
- [A large mixed commit is hard to cherry-pick] → Enforce the commit sequence
  below and prohibit production-source edits in bootstrap commits.

## Migration Plan

1. Land a production-only commit containing production parser/AST/
   validation/codegen changes and its tests for `break`/`continue`. This commit
   is safe to cherry-pick to the
   default-compiler branch.
2. Land a bootstrap-only commit containing parser2/ast2/typeinference2/
   codegen2/orchestration changes and bootstrap tests. It must not modify
   production compiler sources.
3. Land a shared conformance-and-documentation commit containing differential
   fixtures and updates to `docs/spec.md`, `docs/compiler/semantics.md`, and
   `docs/compiler2/differences.md`. It records the convergence after both
   implementation commits are present.
4. Run the production suite, bootstrap suite, differential suite, and a
   bootstrap regeneration check. Roll back the affected lane's commit if its
   suite fails; no data migration is required.
