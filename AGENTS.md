# MyGO 语言规范

## 概述
MyGO 是一个函数式编程语言，具有代数数据类型、Typeclass 系统、模式匹配等特性。

## 核心特性

### 1. 代数数据类型 (ADT)
```mygo
# 枚举类型
enum Option a where
  Some a
  None
end

# 记录类型
struct User where
  name: String
  age: Int
  email: String
end
```

### 2. Typeclass 系统
多态接口定义：
```mygo
interface Eq a where
  (==): a -> a -> Bool
  (!=): a -> a -> Bool
end

interface Ord a where
  (<): a -> a -> Bool
  (<=): a -> a -> Bool
  (>): a -> a -> Bool
  (>=): a -> a -> Bool
end

interface Monad m where
  return: a -> m a
  bind: m a -> (a -> m b) -> m b
end
```

### 3. 模式匹配
```mygo
match expr
  Pattern -> Expr
  _ -> Default
end
```

### 4. 管道操作符
```mygo
# a |> f 等价于 f(a)
# a |> f |> g 等价于 g(f(a))
```

### 5. 类型约束
```mygo
func process(items: List[a]) where Eq[a]: ...
```

## 标准库

### Option 类型
```mygo
func option_some[a](x: a): Option[a]
func option_none[a](): Option[a]
func option_map[a, b](f: a -> b, opt: Option[a]): Option[b]
```

### List 类型
```mygo
func list_cons[a](h: a, t: List[a]): List[a]
func list_nil[a](): List[a]
func list_map[a, b](f: a -> b, list: List[a]): List[b]
func list_filter[a](pred: a -> Bool, list: List[a]): List[a]
```

### 辅助函数
```mygo
# 数学
func math_abs[n: Int]: Int
func math_max[n: Int](list: List[Int]): Int
func math_sum[n: Int](list: List[Int]): Int

# 字符串
func str_toUpper(s: String): String
func str_trim(s: String): String
func str_split(s: String, sep: String): List[String]
```

## 模块系统
```mygo
module Math where
  func add(a: Int, b: Int): Int
    a + b
end
```

## IO Monad
```mygo
func io_liftGo[a](a: a): IO[a]
func io_run[a](io: IO[a]): a
func io_bind[a, b](io: IO[a], f: a -> IO[b]): IO[b]
```

## 示例程序
```mygo
func main(): IO[Unit]
  run <| func()
    result = Math.add(1, 2)
    msg = "Result: " + strconv.Itoa(result * 2)
    IO.print(msg)
  end
end
```

## 实现
- **prelude**: Go 泛型实现核心库（Option, Result, List, Monad）
- **ave-musica**: Go 解析器 + 转译器（MyGO → Go）