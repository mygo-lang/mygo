# AGENTS.md — Entry Index

> This file serves as the entry index. The actual specifications are distributed across the following sub-files, which Agents read on demand according to context.

## Core Conventions

- **Project shape, type model, workflow**: [docs/compiler/core.md](docs/compiler/core.md)
  - Project directory structure, type model rules, workflow conventions, numeric types and literal prefixes

## Types and Values

- **Go FFI, Ref, Option/Result, List**: [docs/compiler/ffi.md](docs/compiler/ffi.md)
  - Go import syntax, Typeclass impl blocks, boundary types Ref/Option/Result
  - List linked-list semantics, Slice/Map/Set mapped to Go native types

## Collections

- **Collection types and literals**: [docs/compiler/collections.md](docs/compiler/collections.md)
  - List/Slice/Map/Set type definitions and design rationale
  - Literal syntax, type inference strategy, AST node definitions

## Language Semantics

- **Current semantics (let/var/tuples/pipes/structs/inherent impls)**: [docs/compiler/semantics.md](docs/compiler/semantics.md)
  - Package declarations, function bodies, let/var/assignment, tuple destructuring, pipe operator
  - Struct literals, Go struct tags, inherent impl method mangling, Typeclass lookup rules
- **Pattern matching (switch/case)**: [docs/compiler/semantics.md#pattern-matching](docs/compiler/semantics.md#pattern-matching-switchcase)
- **New block syntax (if => / case then...end)**: [docs/compiler/semantics.md#new-block-syntax](docs/compiler/semantics.md#new-block-syntax-if--case-thenend)

## Language Features

- **Inline Go embedding**: [AGENTS.inline-go.md](AGENTS.inline-go.md)
  - `go[T] { code: "..."; in x = expr; type T = ... }` syntax
  - Key Files (AST, Lexer, Parser, Compiler)
- **Concurrency primitives**: [docs/compiler/concurrency.md](docs/compiler/concurrency.md)
  - Channel wrappers, directional channel types, and `Spawn`

## Type Inference

- **HM type inference, unary operators, empty collection inference**: [docs/compiler/inference.md](docs/compiler/inference.md)
  - Hindley-Milner Algorithm W implementation
  - Unary operators `!`/`-`, multiline strings
  - Empty collection literal type inference fixes

## Code Generation

- **Per‑file code generation, multiline braces**: [docs/compiler/genfiles.md](docs/compiler/genfiles.md)
  - Per‑`.mygo` file generation to `.gen.go`, HKT type generation
  - Multiline brace NEWLINE support

## Maintenance Information

- **Known issues**: [KNOWN_ISSUES.md](KNOWN_ISSUES.md)
- **Change log**: [HISTORY.md](HISTORY.md)
