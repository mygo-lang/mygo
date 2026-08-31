# MyGO language specification (current implementation)

> This article describes the language surfaces currently supported by the default MyGO compilation pipeline in the repository:
> `parser` -> `typeinference` -> `codegen` -> Go.
> It is a normative summary for language users; the generated Go identifiers, dictionary parameters, and
> Trampoline, etc. are implementation details and do not constitute a source language API unless specifically stated in this article.
>
> The bootstrap pipeline (`mygo --bootstrap`) is implemented independently and is used for bootstrapping and verification; when both
> If there are any differences in behavior, the default pipeline shall prevail.

## 1. Source files, packages and comments

Each `.mygo` file starts with a package declaration, and then writes the declaration directly; historical `module`
Wrapping syntax is no longer supported.

```mygo
package main

import fmt "go:fmt"

# Single line comment
func main() -> () 
let _ = fmt.Println("hello")
end
```

Statements within declarations and blocks are separated by newlines. Newlines inside braced literals are ignored, so you can
Liberty Branch. Most comma-separated lists (parameters, type arguments, call arguments, fields, and collections
element) allows trailing commas.

Imports are divided into two categories:

- `import fmt "go:fmt"` imports the Go package, the alias can be omitted.
- `import name "path/to/package"` Imports the MyGO package; the compiler will compile its dependencies first.

The default compilation will add built-in prelude to non `prelude` packages; `mygo --no-prelude` can
Turning off this behavior is mainly used to compile the prelude itself.

## 2. Type

### 2.1 Basic types

MyGO's basic types map directly to the corresponding Go types:

| MyGO | Go |
| --- | --- |
| `Bool` | `bool` |
| `String` | `string` |
| `Int` / `UInt` | `int` / `uint` |
| `Int8`, `Int16`, `Int32`, `Int64` | `int8`, `int16`, `int32`, `int64` |
| `UInt8`, `UInt16`, `UInt32`, `UInt64` | `uint8`, `uint16`, `uint32`, `uint64` |
| `Float32` / `Float64` | `float32` / `float64` |
| `()` | No return value (unit) |

`()` is both a unit type and a unit value; it is not equivalent to Go's `struct{}`. `Unit` is
Old spelling removed.

Types can be named types, generic instances, function types, or tuple types:

```mygo
let one: Int = 1
let names: Slice[String] = ["Ada", "Lin"]
let lookup: Map[String, Int] = {"Ada": 1}
let f: func(Int, String) -> Bool = check
let pair: (Int, String) = (1, "one")
```

Generic parameters are written after the name: `Box[A]`, `Map[K, V]`. functions, structs, enums and
Interfaces can declare generic parameters.

### 2.2 Type aliases and new named types

Declarations with `=` are aliases and are interchangeable with the target type; declarations without `=` create a new named type.

```mygo
type UserID = Int
type Items[A] = Slice[A]

type AccountID Int
type Queue[A] Slice[A]
```

### 2.3 Reference and predefined boundary types

`Ref[T]` is a non-null reference type recognized by the compiler, generating Go's `*T`. Use
`Ref.new(value)` creates it; calling a value that is already a reference does not add another layer of pointers.

```mygo
let p: Ref[Int] = Ref.new(42)
```

prelude defines `Option[A]` (`Some(A)` / `None`) and
`Result[A, E]` (`Ok(A)` / `Err(E)`). Possibly nullable Go pointer bounds should be used in preference to
`Option[Ref[T]]` means; processes carrying Go `error` should use `Result`.

## 3. Literals and expressions

### 3.1 Literals

Boolean literals are `true` and `false`. Integer supports decimal, hexadecimal `0xff`, octal
`0o777` and binary `0b1010`, you can use `_` as the number separator. Numbers can have type suffixes:

```mygo
let a = 42i8
let b = 200u8
let c = 18_446_744_073_709_551_615u64
let d = 3.14f32
```

Integer suffixes are `i8`, `i16`, `i32`, `i64`, `u`, `u8`, `u16`, `u32`, `u64`;
Floating point suffixes are `f32`, `f64`. Without a suffix the type is determined by context and inference.

Strings come in three forms:

```mygo
let escaped = "line one\\nline two"
let verbatim = """backslash-n: \n stays literal"""
let raw = `line one
line two`
```

Only double-quoted strings process escape sequences. Triple-quoted and
backtick-quoted strings preserve every enclosed character verbatim, including
backslashes and newlines, until the closing delimiter; both forms may span
lines. Also supports Go-style rune literals, such as `'x'`.

### 3.2 Operation, calling and selection

Supports prefixes `!`, `-`, binary arithmetic `+`, `-`, `*`, `/`, comparison
`==`, `!=`, `<`, `>`, `<=`, `>=`, and logical `&&`, `||`.

Use `f(args...)` for function calls and `value.field` for field selection. `|>` and `<|` are
Pipeline form of function application:

```mygo
let total = values |> transform |> sum
let answer = parse <| input
```

Comparison expressions form corresponding typeclass constraints (e.g. equality comparison requires `Eq[A]`).

## 4. Binding, assignment and functions

`let` creates immutable bindings, `var` creates mutable bindings. Subsequently, use `let` to write a variable with the same name to represent
Masking, not assignment; `var` can be reassigned with `=`. Type annotations can be omitted if they can be inferred.

```mygo
let x = 1
let x = x + 1 # New binding, masking the previous x
var count: Int = 0
count = count + 1
let _ = effect() # discard the result
```

The `let`/`var` after the header is package-level binding, visible to all declarations in the same package. Function bodies and other blocks
is a list of statements separated by newlines; the last ordinary expression is the result of the block. `return expr`
Can return early.

Both function declarations and function values use explicit parameters and return types:

```mygo
func add(x: Int, y: Int) -> Int 
x + y
end

let twice = func(x: Int) -> Int 
x*2
end
```

Signatures can be written across lines, and trailing commas are allowed in parameter lists. `letrec ... end` defines a set of mutually recursive
Immutable local binding; each binding must be written as `name: Type = expr`, destructuring is not supported,
`_`, omitted type, or `var`.

```mygo
letrec 
even: func(Int) -> Bool = func(n: Int) -> Bool 
if n == 0 => true else odd(n - 1) 
end 
odd: func(Int) -> Bool = func(n: Int) -> Bool 
if n == 0 => false else even(n - 1) 
end
end
```

Use `(a, b)` for tuples. `let (a, b) = pair` deconstructs tuples, patterns can be nested and available `_` is ignored
Position; the tuple as a whole is treated as a value when not destructured. When calling a function that returns multiple Go values, the top-level tuple
Destructuring can receive these return values directly.

## 5. Control flow and pattern matching

`if` is an expression, available in compact and block forms:

```mygo
let max = if a > b => a else b

let label = if n > 0 then 
"positive"
elsif n == 0 then 
"zero"
else 
"negative"
end
```

Use `else if` for inline chaining; use `elsif ... then` for block form. Old `if cond then a
Else b` inline form, and block form without `then` are not supported.

`while` executes a block and returns `()`:

```mygo
while i < limit 
i = i + 1
end
```

`break` exits the nearest enclosing `while` immediately; `continue` skips the
rest of the current iteration and begins the next one. Both are only valid
inside a `while` body.

`switch` branches based on a literal, enumeration variant, tuple or wildcard pattern, which is also an expression. arrow body
or `then ... end` block body can be used:

```mygo
func show(x: Option[Int]) -> String 
switch x 
case Some(n) => n.ToString() 
case None => "none" 
end
end

let message = switch pair 
case (Some(x), None) then 
"only left: " + x.ToString() 
end 
case _ => "other"
end
```

`_` does not bind a value and can be used as the default branch in a switch. The comma between cases is optional.

## 6. Data type

### 6.1 enum

`enum` declares an algebraic data type; variants can have no fields or with positional fields:

```mygo
enumShape 
Circle(Float64) 
Rectangle(Float64, Float64)
end

let s = Shape.Circle(5.0)
```

Enumeration variants are constructed through `Enum.Variant(...)` (prelude's `Some`, `None`, `Ok`,
`Err` is also provided with its existing shorthand constructor). Use `switch` to destructure variant fields.

### 6.2 struct

`struct` declares named fields, which can take generic parameters; the string after the field is output to Go as is
struct tag.

```mygo
struct Person 
name: String `json:"name"` 
age: Int
end

struct Box[A] 
value: A
end

let p = Person { name: "Ada", age: 37 }
let b = Box[Int] { value: 42 }
```

struct literals use `Type { field: value }`. Generic arguments can be given explicitly or when expected
Omit when the type or field value is sufficient to determine.

### 6.3 collection

`Slice[A]`, `Map[K, V]`, `Set[A]` are directly mapped to Go native collections, not prelude
struct:

```mygo
let xs: Slice[Int] = [1, 2, 3]
let scores: Map[String, Int] = {"Ada": 100}
let tags: Set[String] = {"compiler", "go"}
```

An empty `[]` is a slice literal, and the element type needs to be determined by annotation or context. Each item in the curly brackets is equal to
When `:` is present, it is map; otherwise, it is set. Empty `{}` is interpreted as map by default, but when `Set[A]` is expected
Type is interpreted as set. There is no `List[A]` literal; it is a singly linked list implemented by prelude, with the tail
is `Option[Ref[List[A]]]`.

`Slice[A]` is the only spelling of the slice type; `A[]` is not supported. Slice, Map, Set and
String can obtain the length of `Int` through `.Len()`.

## 7. interface, impl and constraints

MyGO uses nominal concrete types and structural interfaces. The method signature of interface is written as:

```mygo
interface Eq[A] 
func Equals(left: A, right: A) -> Bool
end
```

Functions and interface methods can declare typeclass constraints through `using`:

```mygo
func contains[A](xs: Slice[A], x: A) -> Bool using Eq[A] 
xs.Contains(x)
end
```

You can also name a constraint to use its binding in the function body. `where` has been removed, using it will get migration
Error.

`impl` has three forms:

```mygo
#Intrinsic methods of struct
impl Rectangle 
func area(self: Rectangle) -> Float64 
self.width * self.height 
end
end

# Type implementation interface/typeclass
impl[A] SliceEnumerable[A]: IEnumerable[Slice[A], A] 
# interface method implementation
end

# Anonymous default instance
impl Eq[String] 
# Implementation
end
```

Intrinsic instance methods put the receiver explicitly as the first argument; `value.method(args)` is its calling sugar.
If the first parameter is not of type impl, it is a static method and is called using `Type.method(args)`.
Intrinsic methods are generated as name-mangled top-level functions in Go; source-level method names can be used in different receiving types.
Repeat between.

The typeclass call will first parse the `using` binding, then parse the lexically visible binding, and finally select the package level.
impl. Multiple candidates favor more specific type coverage; an ambiguity error occurs when this cannot be determined.

## 8. Go interop

The `go:` import exposes the Go package and its selectors to MyGO:

```mygo
import fmt "go:fmt"

let text = fmt.Sprint(42)
```

Small, bounded Go operations can use inline Go expressions. `go[T]` explicitly declares the result type,
`in` binds value placeholder, `type` binds type placeholder:

```mygo
let next: Int = go[Int] { 
code: "{x} + 1" 
in x = n
}

let empty: Map[String, Int] = go[Map[String, Int]] { 
code: "map[{K}]{V}{}" 
type K = String 
type V = Int
}
```

The placeholder is written as `{name}`. `go[()]` is used for statements that only perform side effects. Inline code should be kept small and
Clarity: Normal MyGO, Go package imports, `Ref.new` and prelude types take precedence over large blocks of embedded code.

`lib/concurrency` provides `Chan[T]`, `SendChan[T]`, `RecvChan[T]` based on this mechanism,
and `MakeChan`, `MakeChanUnbuffered`, `AsSend`, `AsRecv` and
`Spawn(func() -> ())`.

## 9. Type inference and compilation

The default pipeline performs Hindley--Milner (Algorithm W) inference before generating Go:

- `let` is generalizable and is re-instantiated on each subsequent use; `var` remains monomorphic.
- Explicit annotations must be consistent with inferred results; mismatches prevent generation.
- Function calls, closures, conditional branches, switches, pipes, and collection literals participate in inference.
- Empty collections must rely on annotations or expected types to obtain element types.
- Go FFI selectors also participate in inference; Go's variadic arguments and `any` are handled at the boundary.

Use `mygo sync <dir>` to generate `zz_*.gen.go` corresponding to each `.mygo` source file, use
`mygo build <dir>` Runs the Go build after building. Makefiles are derivatives and should be refreshed by the compiler
rather than manual editing.

Top-level, ABI-compatible tail-recursive and mutually tail-recursive functions can be optimized into Go trampolines; the original function names and
The signature remains unchanged. This optimization does not change the source language semantics, and there is no guarantee that every recursive group will be optimized.

## 10. Current Limitations

- dispatch of typeclass impl with multiple type parameters (e.g. a generic instance of `Result[A, E]`) 
Ambiguities or matching failures are still possible; prefer single-argument instances or inline Go as temporary boundary implementations.
- The map/set discrimination of `{...}` relies on `:`, which may not be consistent in ambiguous writing methods with insufficient type information. 
Intuition; writing explicit collection types for variables avoids problems.
- Inline Go generators handle a small number of simple Go expressions specially; complex embedded code should not assume 
The same structural rewriting capabilities as the full Go parser.
- parser still contains yacc conflicts; the covered grammar should be based on this article and tests, new or marginal grammar 
This should be verified first with the default compiler.
- The bootstrap pipeline is an evolving self-hosted implementation and is not guaranteed to work completely with the default compiler in all edge cases 
consistent.
