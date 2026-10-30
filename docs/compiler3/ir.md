# compiler3 IR contract

`ir3` is the package-level structured ANF between typed `ast2` and optimized Go.
It makes control flow, call classification, effects, representation decisions,
and source provenance explicit without converting ordinary code to full CPS.

## Unit and ownership

One IR unit represents one MyGO package. `PackageSource` groups typed
declarations by their originating `.mygo` file; `BuildPackage` merges those
groups into a package while retaining each declaration's `Path` and `Pos`.
Imported packages contribute signatures and typeclass interfaces, not function
bodies. Cross-file calls therefore keep the caller's source location, and
codegen3 can emit each declaration back to its original file.

Top-level `DeclKind` forms are:

- `FunctionDecl`: type parameters, parameters, results, and body;
- `TypeDecl`: source type parameters and a canonical representation;
- `ValueDecl`: package binding with optional type annotation;
- `ImplDecl`: lowered inherent/typeclass methods;
- `ExternalDecl`: imported signature without a body.

## Expressions and ANF

`Expr` pairs an `ExprKind` with optional `MonoType`, optional `Repr`, and exact
source `Pos`/`Span`. The expression model contains:

- literals, variables, field access, casts, and type application;
- `LetExpr`/`LetRecExpr` bindings and `FunctionExpr` closures;
- direct, closure, method, dictionary, and foreign calls;
- structured `IfExpr`, `SwitchExpr`, `WhileExpr`, and `SeqExpr`;
- `BlockExpr` / `JumpExpr` joins and `RaiseExpr` match failure;
- allocation, primitive, assignment, return, break, continue, and mutation;
- explicit multi-return and option/result unboxing nodes;
- Go FFI, inline Go, and dynamic typeclass boxing nodes.

Recursive references use `Ref[Expr]`, `Ref[Decl]`, or slices so the data model
has finite size. Values are introduced by `let` and calls remain explicit
application nodes; codegen3 consumes this structure rather than re-inferring
statements from an arbitrary expression tree.

## Calls and tail positions

`CallKind` has five values:

| Kind | Meaning |
| --- | --- |
| `Direct` | Statically known MyGO function/helper. |
| `Closure` | Call through a function value. |
| `Method` | Receiver method or unresolved selector. |
| `Dictionary` | Typeclass evidence lookup before instance resolution. |
| `Foreign` | Go FFI or inline-Go boundary. |

`ApplyExpr` carries a tail-position flag. `Tailcall3` only builds SCCs from
statically known targets; foreign calls, unknown closures, and dynamic dispatch
remain ordinary calls. The tail marker is consumed by optimizer tail-call
lowering; it is not a backend heuristic.

## Effects and coeffects

`Effect` records whether an operation is `Pure`, `Generative`, `Read`, `Write`,
`Panic`, `Diverge`, or `Foreign`. `Coeffect` independently records
`NoCoeffect`, `ReadsEnv`, or `ArbitraryCoeffect`.

The optimizer uses these annotations as a safety boundary:

- pure unused expressions may be removed;
- generative allocation and reference creation may be retained when effectful;
- reads may not move across writes;
- panic/diverge effects preserve evaluation order;
- Go FFI and inline Go default to `Foreign` and are never removed, duplicated,
  or reordered;
- dictionary/function values remain pure only while construction has no
  external effect.

Effects are conservative. A pass must leave an operation unchanged when it
cannot prove that transformation is safe.

## Joins, matches, and provenance

Pattern lowering uses `BlockExpr` as a join point, `JumpExpr` to transfer a
result, and `RaiseExpr` to continue with the next match arm. The match subject
is bound once; case guards and nested patterns branch from that binding. This
avoids mutable matched flags and repeated interface assertions.

Every node that can affect diagnostics, line directives, deterministic output,
or optimization ordering carries source position. A cross-file call keeps the
caller's position even when the callee signature comes from another file.

## Verification

`VerifyPackage` runs before code generation. It rejects:

- jumps/raises to unknown or out-of-scope continuations;
- duplicate labels, empty labels, or malformed block parameters;
- missing required types on expressions that codegen3 must emit;
- invalid or inconsistent representation metadata;
- unsupported expressions that reached the backend;
- malformed switch tags, bindings, or declaration structure.

Verification is required for both O0 and optimized pipelines. A malformed IR
must fail before any Go source is emitted.

## Determinism

Maps and worklists are not traversed in runtime order. Helpers sort string keys,
source positions, or stable node IDs before affecting IR dumps, pass decisions,
SCCs, imports, or generated declarations. Re-running lowering and optimization
with the same package and compiler version must produce byte-identical IR dumps
and generated Go.

## Pipeline boundary

```text
typed ast2
  -> Lower3
  -> Represent3
  -> optimizer3
  -> Verify3
  -> codegen3
  -> temporary Go overlay / build cache
```

O0 runs lowering and verification with no optimizing pass, providing a
non-optimizing correctness path. Codegen3 never chooses representations or
tail-call policies; those decisions are already explicit in the IR.
