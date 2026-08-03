# MyGO

MyGO is a new programming language that transpiles to Go. It follows Lisette-style nominal concrete types and structural interfaces, with generic enums, structs, interfaces, and functions.

The repository currently contains:

- The MyGO compiler frontend and two code generation pipelines:
  `internal/mygo/parser`, `internal/mygo/typeinference`, and
  `internal/mygo/codegen` for the production path, plus
  `internal/mygo/parser2`, `internal/mygo/typeinference2`, and
  `internal/mygo/codegen2` for the self-hosted bootstrap pipeline.
- A prelude written primarily in MyGO under `prelude/`.
- A language server implemented in MyGO under `lsp/` and exposed as
  `cmd/mygo-lsp`.
- Reusable standard libraries under `lib/`, including channels/spawn,
  sorting/searching, and an FParsec-inspired parser combinator library.

## Quick Start

### Installation

Build from this repository to use the current compiler:

```bash
go build -o mygo ./cmd/mygo
go build -o mygo-lsp ./cmd/mygo-lsp
```

The binary can also be installed through the Go toolchain once a release is
published:

```bash
go install github.com/mygo-lang/mygo/cmd/mygo@latest
```

### Examples

```mygo
package main

import fmt "go:fmt"

enum Shape
  Circle(Float64)
  Rectangle(Float64, Float64)
end

func area(shape: Shape) -> Float64
  switch shape
    case Circle(r) => 3.14 * r * r
    case Rectangle(w, h) => w * h
  end
end

func main() -> ()
  let circle = Shape.Circle(5.0)
  fmt.Println("Area:", area(circle))
end
```

Compile `<package dir>` and generate one `.gen.go` file per `.mygo` source:

```bash
./mygo sync examples/main
```

MyGO files start with a leading `package <name>` header followed directly by
declarations. The old file-level `module` wrapper is no longer used. The build
command also accepts `build` to run `go build` after generating source.

Every package now receives a built-in prelude during compilation. The prelude is written in MyGO when possible and currently provides shared protocols such as `ToString[A]` and `Eq[A]`, so generic formatting and comparison code can rely on those interfaces instead of falling back to ad hoc `any` usage.

Run with `go run`:

```bash
go run ./examples/main/
```

Or build and run:

```bash
go build -o main.exe ./examples/main/
./main.exe
```

Run the canonical example:

```bash
./mygo sync examples/main
go run ./examples/main
```

Use the self-hosted bootstrap pipeline with `--bootstrap`:

```bash
go run ./cmd/mygo --bootstrap sync internal/mygo
```

See [docs/compiler2/core.md](docs/compiler2/core.md) for the differences
between the default pipeline and the self-hosted parser/typechecker/codegen
pipeline.

## Key Features

### Type System

- **Explicit type parameters**: Preserved in the AST and generated Go code.
- **Lisette-style nominal concrete types and structural interfaces**.
- **Generic enums, structs, interfaces, and functions**: Remain generic rather than collapsing to `any`.
- **Named primitive spellings**: `Int`, `String`, `Bool`, and the other primitive spellings map to Go primitives.
- **Unit type**: `()` lowers to a Go function with no return values, not to `struct{}`.

### Expressions

- **Function bodies and other block forms**: Newline-separated statement lists; the last plain expression in a block is the return value.
- **If expression**: Supports `if cond => a else b` and block form `if cond then ... elsif ... else ... end`.
- **Let binding**: Introduces an immutable binding. Rebinding the same source name must use a later `let` and is treated as shadowing, not assignment.
- **Letrec binding**: `letrec ... end` introduces an immutable recursive binding group with explicit type annotations.
- **Var binding**: Introduces a mutable binding and may be assigned again later in the same scope.
- **Let type inference**: May omit its type annotation when the initializer provides enough information for inference.
- **Discard form**: `let _ = ...` is the supported discard form for return values that should not be bound.

### Struct Literals

- **Constructor-like form**: Such as `ABC { aaa: 123 }`.
- **Generic struct literals with explicit type arguments**: Like `Box[Int64] { value: 123 }`.
- **Inherent impl methods**: Methods on a struct are declared in `impl Type` blocks and lower to top-level Go functions with mangled names.

### Go FFI

- **Go package imports**: Use `import "go:pkg/name"`, or an alias such as `import fmt "go:fmt"`.
- **Package-qualified selectors**: Selectors such as `fmt.Sprint(...)` lower as Go selectors.
- **Prelude protocols**: Prefer `ToString[A]`, `Eq[A]`, `Ord[A]`, `Default[A]`, `From[A, B]`, and `Into[A, B]` from the built-in prelude.
- **Option and Result**: `Option[A]` and `Result[A, E]` are prelude algebraic data types mapped to generated Go sum types.
- **Slice, Map, and Set**: `Slice[A]`, `Map[K, V]`, and `Set[A]` lower directly to Go `[]A`, `map[K]V`, and `map[A]struct{}`.
- **List**: `List[A]` is a prelude-implemented singly linked list using `head` and `tail: Option[Ref[List[A]]]`.
- **Ref[T]**: The non-nil reference form at the Go boundary lowers to `*T` in generated Go; use `Ref.new(expr)`.
- **Option[Ref[T]]**: The preferred shape for possibly-nil pointer returns, preserved rather than collapsed to a bare pointer.
- **Inline Go**: `go[T] { code: "..."; in x = expr; type T = ... }` embeds small Go snippets with operand substitution.

### Concurrency

`lib/concurrency` provides channel wrappers on top of inline Go embedding:

- `Chan[T]`, `SendChan[T]`, and `RecvChan[T]` for bidirectional and directional channels.
- `MakeChan[T](buffer)`, `MakeChanUnbuffered[T]()`, `AsSend[T]()`, and `AsRecv[T]()`.
- `Spawn(fn)` for starting goroutines.

The channel types implement `IChannel`, `IReadableChan`, and `IWritableChan`
interfaces. The compiler recognizes directional channel type strings in
inline Go code and generated type emission.

### Language Server

The `lsp/` directory is a MyGO implementation of a language server exposing
completion, hover, diagnostics, symbols, definition, references, and workspace
symbols over the LSP protocol. Build it with:

```bash
go build -o mygo-lsp ./cmd/mygo-lsp
```

## Workflow

- Keep `examples/main/main.mygo` runnable after compiler changes; its `main` function should actually do work.
- Keep `examples/data-structure/data-structure.mygo`, `examples/parsec/`, and `lib/` in sync when collection, parser, or library semantics change.
- Prefer expressing prelude functionality in MyGO first; only fall back to Go for pieces that cannot yet be represented safely in the language itself.
- Generated `.gen.go` files are checked in for the prelude, libraries, examples, LSP, and bootstrap compiler; refresh them when MyGO source changes.
- When checking the build, use a writable Go cache if the default cache path is unavailable in this environment:

```bash
GOCACHE=/tmp/mygo-gocache go test ./...
```

## License

MIT License
