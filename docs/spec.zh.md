# MyGO 语言规范（当前实现）

> 本文描述仓库中默认 MyGO 编译管线当前支持的语言表面：
> `parser` -> `typeinference` -> `codegen` -> Go。
> 它是面向语言使用者的规范性摘要；生成的 Go 标识符、字典参数和
> trampoline 等为实现细节，除非本文特别说明，否则不构成源语言 API。
>
> 自举管线（`mygo --bootstrap`）是独立实现，用于自举与验证；当两者
> 行为尚有差异时，以默认管线为准。

## 1. 源文件、包与注释

每个 `.mygo` 文件以包声明开始，之后直接写声明；历史上的 `module`
包装语法不再支持。

```mygo
package main

import fmt "go:fmt"

# 单行注释
func main() -> ()
  let _ = fmt.Println("hello")
end
```

声明和块中的语句以换行分隔。花括号字面量内部的换行会被忽略，因此可
自由分行。大多数逗号分隔的列表（参数、类型实参、调用实参、字段和集合
元素）允许尾随逗号。

导入分为两类：

- `import fmt "go:fmt"` 导入 Go 包，别名可省略。
- `import name "path/to/package"` 导入 MyGO 包；编译器会先编译其依赖。

默认编译会向非 `prelude` 包加入内建 prelude；`mygo --no-prelude` 可
关闭该行为，主要用于编译 prelude 本身。

## 2. 类型

### 2.1 基础类型

MyGO 的基础类型直接映射为相应 Go 类型：

| MyGO | Go |
| --- | --- |
| `Bool` | `bool` |
| `String` | `string` |
| `Int` / `UInt` | `int` / `uint` |
| `Int8`、`Int16`、`Int32`、`Int64` | `int8`、`int16`、`int32`、`int64` |
| `UInt8`、`UInt16`、`UInt32`、`UInt64` | `uint8`、`uint16`、`uint32`、`uint64` |
| `Float32` / `Float64` | `float32` / `float64` |
| `()` | 无返回值（unit） |

`()` 既是 unit 类型也是 unit 值；它不等价于 Go 的 `struct{}`。`Unit` 是
已移除的旧拼写。

类型可为命名类型、泛型实例、函数类型或元组类型：

```mygo
let one: Int = 1
let names: Slice[String] = ["Ada", "Lin"]
let lookup: Map[String, Int] = {"Ada": 1}
let f: func(Int, String) -> Bool = check
let pair: (Int, String) = (1, "one")
```

泛型参数写在名称后：`Box[A]`、`Map[K, V]`。函数、struct、enum 和
interface 都可声明泛型参数。

### 2.2 类型别名与新命名类型

带 `=` 的声明是别名，与目标类型可互换；不带 `=` 的声明创建新的命名类型。

```mygo
type UserID = Int
type Items[A] = Slice[A]

type AccountID Int
type Queue[A] Slice[A]
```

### 2.3 引用与预定义边界类型

`Ref[T]` 是编译器识别的非空引用类型，生成 Go 的 `*T`。使用
`Ref.new(value)` 创建它；对已经是引用的值调用不会再增加一层指针。

```mygo
let p: Ref[Int] = Ref.new(42)
```

prelude 定义 `Option[A]`（`Some(A)` / `None`）和
`Result[A, E]`（`Ok(A)` / `Err(E)`）。可能为空的 Go 指针边界应优先用
`Option[Ref[T]]` 表示；携带 Go `error` 的流程应使用 `Result`。

## 3. 字面量与表达式

### 3.1 字面量

布尔字面量为 `true` 与 `false`。整数支持十进制、十六进制 `0xff`、八进制
`0o777` 和二进制 `0b1010`，可使用 `_` 作为数字分隔符。数字可带类型后缀：

```mygo
let a = 42i8
let b = 200u8
let c = 18_446_744_073_709_551_615u64
let d = 3.14f32
```

整数后缀为 `i8`、`i16`、`i32`、`i64`、`u`、`u8`、`u16`、`u32`、`u64`；
浮点后缀为 `f32`、`f64`。没有后缀时由上下文和推断决定类型。

字符串有三种形式：

```mygo
let escaped = "line one\\nline two"
let verbatim = """backslash-n: \n stays literal"""
let raw = `line one
line two`
```

只有双引号字符串处理转义。三引号和反引号字符串逐字保留所有包含的字符，
包括反斜杠和换行，直到关闭定界符；两种形式都可以跨行。还支持 Go 风格
rune 字面量，例如 `'x'`。

### 3.2 运算、调用与选择

支持前缀 `!`、`-`，二元算术 `+`、`-`、`*`、`/`，比较
`==`、`!=`、`<`、`>`、`<=`、`>=`，以及逻辑 `&&`、`||`。

函数调用使用 `f(args...)`，字段选择使用 `value.field`。`|>` 与 `<|` 是
函数应用的管道形式：

```mygo
let total = values |> transform |> sum
let answer = parse <| input
```

比较表达式会形成相应的 typeclass 约束（例如相等比较需要 `Eq[A]`）。

## 4. 绑定、赋值与函数

`let` 创建不可变绑定，`var` 创建可变绑定。后续以 `let` 写同名变量表示
遮蔽，而不是赋值；`var` 可通过 `=` 重赋值。类型标注在可推断时可以省略。

```mygo
let x = 1
let x = x + 1       # 新绑定，遮蔽前一个 x
var count: Int = 0
count = count + 1
let _ = effect()    # 丢弃结果
```

包头之后的 `let`/`var` 为包级绑定，对同包的所有声明可见。函数体和其他块
是由换行分隔的语句列表；最后一个普通表达式是块的结果。`return expr`
可提前返回。

函数声明和函数值都使用显式参数与返回类型：

```mygo
func add(x: Int, y: Int) -> Int
  x + y
end

let twice = func(x: Int) -> Int
  x * 2
end
```

签名可跨行书写，参数列表允许尾随逗号。`letrec ... end` 定义一组互相递归的
不可变局部绑定；其中每个绑定都必须写成 `name: Type = expr`，不支持解构、
`_`、省略类型或 `var`。

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

元组使用 `(a, b)`。`let (a, b) = pair` 解构元组，模式可嵌套并可用 `_` 忽略
位置；不解构时元组整体作为一个值。调用返回多个 Go 值的函数时，顶层元组
解构可直接接收这些返回值。

## 5. 控制流与模式匹配

`if` 是表达式，有紧凑和块两种形式：

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

内联链使用 `else if`；块形式使用 `elsif ... then`。旧的 `if cond then a
else b` 内联形式，以及没有 `then` 的块形式均不支持。

`while` 执行一个块并返回 `()`：

```mygo
while i < limit
  i = i + 1
end
```

`break` 立即退出最近的 `while`；`continue` 跳过当前迭代剩余部分并开始
下一次迭代。两者只能在 `while` 体内出现。

`switch` 根据字面量、枚举变体、元组或通配模式分支，也是表达式。箭头主体
或 `then ... end` 块主体均可使用：

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

`_` 不绑定值，并在 switch 中可作为默认分支。case 之间的逗号可选。

## 6. 数据类型

### 6.1 enum

`enum` 声明代数数据类型；变体可不带字段或带位置字段：

```mygo
enum Shape
  Circle(Float64)
  Rectangle(Float64, Float64)
end

let s = Shape.Circle(5.0)
```

枚举变体通过 `Enum.Variant(...)` 构造（prelude 的 `Some`、`None`、`Ok`、
`Err` 也以其既有的简写构造器提供）。使用 `switch` 解构变体字段。

### 6.2 struct

`struct` 声明命名字段，可带泛型参数；字段后的字符串是原样输出到 Go 的
struct tag。

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

struct 字面量使用 `Type { field: value }`。泛型实参可显式给出，也可在预期
类型或字段值足以确定时省略。

### 6.3 collection

`Slice[A]`、`Map[K, V]`、`Set[A]` 直接映射为 Go 原生集合，不是 prelude
struct：

```mygo
let xs: Slice[Int] = [1, 2, 3]
let scores: Map[String, Int] = {"Ada": 100}
let tags: Set[String] = {"compiler", "go"}
```

空 `[]` 是 slice 字面量，元素类型需要由标注或上下文确定。花括号中每项均
有 `:` 时是 map；否则是 set。空 `{}` 默认解释为 map，但在预期 `Set[A]`
类型时解释为 set。没有 `List[A]` 字面量；它是由 prelude 实现的单链表，尾部
为 `Option[Ref[List[A]]]`。

`Slice[A]` 是唯一的 slice 类型拼写；`A[]` 不支持。Slice、Map、Set 与
String 可通过 `.Len()` 取得 `Int` 长度。

## 7. interface、impl 与约束

MyGO 使用名义 concrete type 与结构性 interface。interface 的方法签名写为：

```mygo
interface Eq[A]
  func Equals(left: A, right: A) -> Bool
end
```

函数和 interface 方法可通过 `using` 声明 typeclass 约束：

```mygo
func contains[A](xs: Slice[A], x: A) -> Bool using Eq[A]
  xs.Contains(x)
end
```

也可为约束命名，以在函数体中使用其绑定。`where` 已移除，使用它会得到迁移
错误。

`impl` 有三种形态：

```mygo
# struct 的固有方法
impl Rectangle
  func area(self: Rectangle) -> Float64
    self.width * self.height
  end
end

# 类型实现 interface/typeclass
impl[A] SliceEnumerable[A]: IEnumerable[Slice[A], A]
  # interface 方法实现
end

# 匿名默认实例
impl Eq[String]
  # 实现
end
```

固有实例方法把接收者显式放在第一个参数；`value.method(args)` 是其调用糖。
若首个参数不是 impl 类型，则它是静态方法，使用 `Type.method(args)` 调用。
固有方法在 Go 中生成为经过名称改写的顶层函数；源级方法名可在不同接收类型
间重复。

typeclass 调用会优先解析 `using` 绑定，再解析词法可见绑定，最后选择包级
impl。多候选会偏好更具体的类型覆盖；无法确定时是歧义错误。

## 8. Go 互操作

`go:` 导入将 Go 包及其选择器暴露给 MyGO：

```mygo
import fmt "go:fmt"

let text = fmt.Sprint(42)
```

小型、边界性的 Go 操作可使用内联 Go 表达式。`go[T]` 显式声明结果类型，
`in` 绑定值占位符，`type` 绑定类型占位符：

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

占位符写作 `{name}`。`go[()]` 用于只执行副作用的语句。内联代码应保持小而
清晰：普通 MyGO、Go 包导入、`Ref.new` 与 prelude 类型优先于大段嵌入代码。

`lib/concurrency` 基于此机制提供 `Chan[T]`、`SendChan[T]`、`RecvChan[T]`，
以及 `MakeChan`、`MakeChanUnbuffered`、`AsSend`、`AsRecv` 和
`Spawn(func() -> ())`。

## 9. 类型推断与编译

默认管线在生成 Go 前执行 Hindley--Milner（Algorithm W）推断：

- `let` 可泛化，随后每次使用会重新实例化；`var` 保持单态。
- 显式标注必须与推断结果统一；不匹配会阻止生成。
- 函数调用、闭包、条件分支、switch、管道和集合字面量参与推断。
- 空集合必须依赖标注或预期类型获得元素类型。
- Go FFI 选择器也参与推断；Go 的可变参数与 `any` 在边界处处理。

使用 `mygo sync <dir>` 生成与每个 `.mygo` 源文件对应的 `zz_*.gen.go`，使用
`mygo build <dir>` 生成后运行 Go 构建。生成文件是派生物，应通过编译器刷新
而非手工编辑。

顶层、ABI 兼容的尾递归和互相尾递归函数可被优化为 Go trampoline；原函数名和
签名保持不变。该优化不改变源语言语义，也不保证每个递归组都会被优化。

## 10. 当前限制

- 含多个类型参数的 typeclass impl（例如 `Result[A, E]` 的通用实例）分派
  尚可能歧义或匹配失败；优先使用单参数实例或内联 Go 作为临时边界实现。
- `{...}` 的 map/set 判别依赖 `:`，在类型信息不足的歧义写法中可能不符合
  直觉；为变量写明确集合类型可避免问题。
- 内联 Go 的生成器会特别处理少量简单 Go 表达式；复杂嵌入代码不应假定得到
  与完整 Go 解析器相同的结构化重写能力。
- parser 仍含有 yacc 冲突；已覆盖的语法应以本文及测试为准，新增或边缘语法
  应先以默认编译器验证。
- 自举管线是正在演进的自托管实现，不承诺与默认编译器在所有边缘情形完全
  一致。
