## Context

Bootstrap uses `codegen2` to emit one Go file per MyGO source file. The
generator currently decides whether to add a Prelude dot import from unlowered
declarations. That misses dictionary/helper identifiers introduced during
method-call lowering and can retain an import for declarations that emit no Go
code. Separately, a final `switch` expression enters the generic single-value
return path even when the enclosing function's declared return is a tuple.

See proposal.md and the delta specifications for the externally observable
requirements.

## Goals / Non-Goals

**Goals:**

- Keep top-level tuple-return conventions intact for switch expressions.
- Determine generated-file Prelude imports from emitted Go references or an
  equally complete lowering-aware dependency signal.
- Cover positive and negative import cases with bootstrap generation followed
  by Go compilation.

**Non-Goals:**

- Changing MyGO syntax, tuple value semantics, or the Prelude public API.
- Altering imports explicitly declared by MyGO source.
- Implementing or changing external application behavior.

## Decisions

### Route tail-position switches through tuple-aware return lowering

The return-expression lowering will recognize a tuple-returning function whose
last expression is a `switch` and lower its cases at the function return
boundary. Each case then produces the declared Go result list rather than an
anonymous tuple struct assigned to a temporary.

This is preferred to post-processing generated Go because the typed return
shape and per-case expressions are available in the AST lowering context.
It also preserves the existing distinction between top-level multi-return
functions and tuple-valued function literals.

### Make Prelude import detection lowering-aware and per emitted file

Prelude dependency detection will account for the final emitted declaration
set, including mangled dictionary calls introduced by collection and string
method dispatch. Declarations erased during lowering, such as typeclass
interfaces, will not on their own cause an import.

This is preferred to importing Prelude unconditionally: an unconditional dot
import makes independent generated files fail Go compilation when they do not
use Prelude. It is also preferred to a hard-coded method list because new
Prelude dispatch paths would recreate the omission.

### Test through the bootstrap public path and Go compilation

Focused compiler tests will build temporary MyGO packages with
`CompileDirBootstrap`, inspect the relevant per-file import behavior, and run
`go test` or `go build` on the emitted package. Unit-level generated-source
assertions may supplement these tests but do not replace the Go compilation
check.

## Risks / Trade-offs

- [Lowering-aware import analysis can miss a newly introduced generated helper]
  → Centralize the decision at or after lowering and cover both method dispatch
  and no-reference fixtures.
- [Tuple switch handling can accidentally change function-literal behavior]
  → Retain the existing `tupleReturnStruct` distinction and add a top-level
  regression focused on the affected return path.
- [A single generated file's import decision can be masked by package-wide
  usage] → Use multi-file fixtures and assert imports in each individual
  generated file before compiling the complete package.

## Migration Plan

The change is source and generated-output compatible for valid programs. Land
the compiler changes and regression tests together; no user migration or data
migration is needed. Reverting the change restores the prior compiler behavior
without changing user source files.
