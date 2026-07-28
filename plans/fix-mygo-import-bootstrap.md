# 修复 bootstrap 编译 MyGO 包导入和别名支持

## 问题分析

bootstrap 编译 `internal/mygo/parser2` 时，遇到 `import ps "github.com/mygo-lang/mygo/lib/text/parsec"` 时报错 `unknown identifier ps`。

### 根本原因

当前代码把所有 import 混在一起通过 `GoPackageEntry` 处理：

1. **`collectGoPackageImports`** 收集所有 import（包括 `go:xxx` 和 MyGO 包路径）并存在 `GoPackageEntry` 中
2. **`seedGoPackageEnv`** 把所有条目作为 `TGoPackage(path)` 加入环境
3. **`inferField`** 中遇到非 `go:` 路径的 `TGoPackage` 时，直接返回**新鲜类型变量**而不是类型信息

## 设计方案

### 方案：Go 导入和 MyGO 导入完全分离

```
┌─────────────────────────────────────────────────────────┐
│                   类型推断层                               │
│                                                         │
│  Go FFI 导入 (go:xxx)      MyGO 导入 (package 路径)      │
│  ┌─────────────────────┐   ┌────────────────────────┐   │
│  │  GoPackageEntry      │   │  MyGoPackageEntry      │   │
│  │  - Alias: "fmt"     │   │  - Alias: "ps"         │   │
│  │  - Path: "go:fmt"   │   │  - Path: "lib/text/.." │   │
│  │  - Funcs: [...]     │   │  - Funcs: [...]        │   │
│  │  - Types: [...]     │   │  - Types: [...]        │   │
│  └─────────────────────┘   │  - Enums: [...]        │   │
│                            │  - Structs: [...]      │   │
│  TGoPackage(path)          └────────────────────────┘   │
│  → env中的别名                                             │
│  → seedGoPackageEnv           TMyGoPackage(path)        │
│                                → env中的别名             │
│                                → seedMyGoPackageEnv     │
└─────────────────────────────────────────────────────────┘
```

### 具体修改

#### 1. `typeinference2/types.mygo` — 添加 `MyGoPackageEntry` 类型

```mygo
struct MyGoPackageEntry
  Alias: String
  Path: String
  # Funcs 记录 MyGO 包中导出的函数签名，用 path 区分
  FuncSignatures: Slice[GoFuncSignature]
  # Types 记录 MyGO 包中导出的类型签名（struct, enum, interface 等）
  TypeSignatures: Slice[GoTypeSignature]
end
```

#### 2. `bootstrap.mygo` — 让 `bootstrapWalkImports` 收集所有签名

`bootstrapMyGoPackageDeclSignatures` 目前只收集 `FuncDecl`，需要扩展为也收集：
- `StructDecl` → `GoTypeSignature`
- `EnumDecl` → `GoTypeSignature`（含 variants 作为方法）
- `InterfaceDecl` → `GoTypeSignature`

#### 3. `typeinference2/types.mygo` — 添加 `MyGoPackageEnv` 和 `seedMyGoPackageEnv`

```mygo
func seedMyGoPackageEnv(imports: Slice[MyGoPackageEntry], env: Slice[EnvEntry]) -> Slice[EnvEntry]
```

为 MyGO 包注册 `TMyGoPackage(path)` 绑定。

#### 4. `bootstrap.mygo` — 收集 MyGO 包完整签名

- 解析依赖包的所有 decl（`StructDecl`, `EnumDecl`, `InterfaceDecl`, `FuncDecl`）
- 填入 `MyGoPackageEntry.Types`