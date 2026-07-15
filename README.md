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
  Circle(r: Float64),
  Rectangle { width: Float64, height: Float64 },
}

func area(shape: Shape) -> Float64
  switch shape
    case Shape.Circle(r) => 3.14 * r * r,
    case Shape.Rectangle { width, height } => width * height,
  end
end

func main() -> ()
  let circle = Shape.Circle(5.0)
  fmt.Println("Area:", area(circle))
end
```

Compile:

```bash
mygo examples/main/main.mygo
```

MyGO files now declare the generated Go package with a leading `package <name>` header. The old file-level `module` wrapper is no longer used, and declarations follow directly after the package line.

Every package now receives a built-in prelude during compilation. The prelude is written in MyGO when possible and currently provides shared protocols such as `ToString[A]` and `Eq[A]`, so generic formatting and comparison code can rely on those interfaces instead of falling back to ad hoc `any` usage.

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
- **If expression**: Supports `if cond => a else b` and block form `if cond then ... elsif ... else ... end`
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
- **Prelude protocols**: Prefer `ToString[A]` for textual conversion and formatting of generic values; the built-in prelude provides the common instances
- **Ref[T]**: The non-nil reference form at the Go boundary lowers to `*T` in generated Go
- **Option[Ref[T]]**: The preferred shape for possibly-nil pointer returns, preserved rather than collapsed to a bare pointer

## Workflow

- Keep `examples/main/main.mygo` runnable after compiler changes; its `main` function should actually do work
- When checking the build, use a writable Go cache if the default cache path is unavailable in this environment
- Prefer expressing prelude functionality in MyGO first; only fall back to Go for pieces that cannot yet be represented safely in the language itself

## License

MIT License
