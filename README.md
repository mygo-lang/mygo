# MyGO

MyGO is a new programming language that transpiles to Go. It follows Lisette-style nominal concrete types and structural interfaces, with generic enums, structs, interfaces, and functions.

## Quick Start

### Installation

```bash
go install github.com/mygo-lang/mygo/cmd/mygo@latest
```

### Examples

```mygo
package main

import "go:fmt"

enum Shape {
    Circle(r: float64),
    Rectangle { width: float64, height: float64 },
}

fn area(shape: Shape) -> float64 {
    match shape {
        Shape.Circle(r) => 3.14 * r * r,
        Shape.Rectangle { width, height } => width * height,
    }
}

fn main() {
    let circle = Shape.Circle(5.0)
    fmt.Println("Area:", area(circle))
}
```

Compile:

```bash
mygo examples/main/main.mygo
```

MyGO files now declare the generated Go package with a leading `package <name>` header. The old file-level `module` wrapper is no longer used, and declarations follow directly after the package line.

Run with `go run`:

```bash
go run examples/main/zz_mygo.gen.go
```

Or build and run:

```bash
go build -o zz_mygo.gen.exe examples/main/zz_mygo.gen.go
./zz_mygo.gen.exe
```

## Key Features

### Type System

- **Explicit type parameters**: Preserved in the AST and generated Go code
- **Lisette-style nominal concrete types and structural interfaces**
- **Generic enums, structs, interfaces, and functions**: Remain generic rather than collapsing to `any`
- **Named primitive spellings**: `Int`, `String`, and `Bool` map to Go primitives in generation
- **Unit type**: A return-only marker in MyGO lowers to a Go function with no return values, not to `struct{}`

### Expressions

- **Function bodies and other block forms**: Newline-separated statement lists; the last plain expression in a block is the return value
- **If expression**: Supports a single-line expression form like `if cond then a else b`
- **Let binding**: Introduces an immutable binding. Rebinding the same source name must use a later `let` and is treated as shadowing, not assignment
- **Var binding**: Introduces a mutable binding and may be assigned again later in the same scope
- **Let type inference**: May omit its type annotation when the initializer provides enough information for inference
- **Discard form**: `let _ = ...` is the supported discard form for return values that should not be bound

### Struct Literals

- **Constructor-like form**: Such as `ABC { aaa: 123 }`
- **Generic struct literals with explicit type arguments**: Like `Box[Int64] { value: 123 }`

### Go FFI

- **Go package imports**: Use `import "go:pkg/name"` for Go packages
- **Package-qualified selectors**: Selectors such as `fmt.Sprint(...)` lower as Go selectors
- **Ref[T]**: The non-nil reference form at the Go boundary lowers to `*T` in generated Go
- **Option[Ref[T]]**: The preferred shape for possibly-nil pointer returns, preserved rather than collapsed to a bare pointer

## Workflow

- Keep `examples/main/main.mygo` runnable after compiler changes; its `main` function should actually do work
- When checking the build, use a writable Go cache if the default cache path is unavailable in this environment

## License

MIT License
