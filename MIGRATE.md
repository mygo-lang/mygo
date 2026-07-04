# MyGO Typeclass 新语法方案

## Summary
把 typeclass 语义收敛成“具名/匿名实例 + `using` 显式约束”的单一路线，保留 MyGO 命名风格，目标是让解析、编译、示例代码三者一致，不再混用旧的 runtime registry 和局部降级逻辑。

## Key Changes
- 语法层保留三件事：
  - `interface` 定义 typeclass
  - `impl` 定义实例
  - `using` 定义上下文需求
- 增加两类实例形态：
  - 具名实例：`impl intShow: Show[Int]`
  - 匿名实例：`impl Show[String]`
- `using` 统一写成显式依赖列表，支持带实例名的绑定：
  - `func demo(x: Int) -> String using intShow: Show[Int]`
  - `func demo[A](x: A) -> String using showA: Show[A]`
- 解析规则上移除 `where` 作为主入口，只保留迁移兼容或直接报错。
- 编译语义改成显式字典传递：
  - `using` 约束在调用点自动补齐为参数
  - `show(x)` 这类方法调用绑定到已解析的 dictionary，不再走全局注册表
- 实例选择顺序固定为：
  - 显式传入实例
  - 词法作用域可见实例
  - 包级默认实例
  - 多候选并列则报歧义

## 示例语法
```mygo
import strconv "go:strconv"

interface Show[A]
  func show(value: A) -> String
end

impl intShow: Show[Int]
  func show(value: Int) -> String
    strconv.Itoa(value)
  end
end

impl Show[String]
  func show(value: String) -> String
    value
  end
end

func demo(x: Int) -> String using intShow: Show[Int]
  x.show()
end

func demo2[A](x: A) -> String using showA: Show[A]
  x.show()
end
```

## Test Plan
- Parser
  - `interface / impl / using` 都能解析
  - 具名实例与匿名实例都能区分
  - `where` 在新语法下明确失败或给出迁移提示
  - 接口方法签名里的 `using` 能保留
- Compiler
  - 单方法和多方法 typeclass 都能生成显式参数传递
  - 同名方法在不同 interface 下不串台
  - 泛型实例保持泛型，不压扁成 `any`
  - 歧义实例能正确报错
- Regression
  - `prelude/prelude.mygo` 能迁移到新语法
  - `examples/main/main.mygo` 和 `examples/data-structure/data-structure.mygo` 保持可运行
  - 生成 Go 中不再出现 registry / reflect fallback 作为 typeclass 主路径

## Assumptions
- 主语法最终以 MyGO 命名为准，继续使用 `interface / impl / using`。
- `impl Show[String]` 代表匿名默认实例，`impl intShow: Show[Int]` 代表命名实例。
- `using` 的目标是静态字典传递，不再引入新的运行期查找中心。
- 这次重构会一并迁移 prelude，避免主语言和 prelude 双轨并存。