# AGENTS.md — Entry Index

> This file serves as the entry index. The actual specifications are distributed across the following sub-files, which Agents read on demand according to context.

## Core Conventions

- **Project shape, type model, workflow**: [AGENTS.core.md](AGENTS.core.md)
  - Project directory structure, type model rules, workflow conventions, numeric types and literal prefixes

## Types and Values

- **Go FFI, Ref, Option/Result, List**: [AGENTS.ffi.md](AGENTS.ffi.md)
  - Go import syntax, Typeclass impl blocks, boundary types Ref/Option/Result
  - List linked-list semantics, Slice/Map/Set mapped to Go native types

## Collections

- **Collection types and literals**: [AGENTS.collections.md](AGENTS.collections.md)
  - List/Slice/Map/Set type definitions and design rationale
  - Literal syntax, type inference strategy, AST node definitions

## Language Semantics

- **Current semantics (let/var/tuples/pipes/structs)**: [AGENTS.semantics.md](AGENTS.semantics.md)
  - Package declarations, function bodies, let/var/assignment, tuple destructuring, pipe operator
  - Struct literals, Go struct tags, Typeclass lookup rules
- **Pattern matching (switch/case)**: [AGENTS.semantics.md#pattern-matching](AGENTS.semantics.md#pattern-matching-switchcase)
- **New block syntax (if => / case then...end)**: [AGENTS.semantics.md#new-block-syntax](AGENTS.semantics.md#new-block-syntax-if--case-thenend)

## Language Features

- **Inline Go embedding**: [AGENTS.inline-go.md](AGENTS.inline-go.md)
  - `go[T] { code: "..."; in x = expr; type T = ... }` syntax
  - Key Files (AST, Lexer, Parser, Compiler)
- **Concurrency primitives**: [AGENTS.concurrency.md](AGENTS.concurrency.md)
  - Channel wrappers, directional channel types, and `Spawn`

## Type Inference

- **HM type inference, unary operators, empty collection inference**: [AGENTS.inference.md](AGENTS.inference.md)
  - Hindley-Milner Algorithm W implementation
  - Unary operators `!`/`-`, multiline strings
  - Empty collection literal type inference fixes

## Code Generation

- **Per‑file code generation, multiline braces**: [AGENTS.genfiles.md](AGENTS.genfiles.md)
  - Per‑`.mygo` file generation to `.gen.go`, HKT type generation
  - Multiline brace NEWLINE support

## Maintenance Information

- **Known issues**: [KNOWN_ISSUES.md](KNOWN_ISSUES.md)
- **Change log**: [HISTORY.md](HISTORY.md)
