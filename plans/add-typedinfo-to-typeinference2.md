# 完善 typeinference2 中的 InferState 和 TypedInfo 结构

## 目标

根据旧有 `internal/mygo/typeinference` 中的结构，完善 `internal/mygo/typeinference2` 中的 `InferState` 结构体，并添加 `TypedInfo` 等配套结构。

## 背景分析

### 旧版结构（Go 代码）

旧版 `typeinference` 包中的核心结构：

```go
// InferState - 推理状态
type InferState struct {
    FreshVarID       int                     // 新鲜变量计数器
    PkgInfo          *PkgInfo                // 包信息（用于 enum/struct 查找）
    GoPackages       map[string]*GoPackageInfo   // Go 导入包
    MyGoPackages     map[string]*MyGoPackageInfo // MyGO 导入包
    MyGoPackageCache map[string]*MyGoPackageInfo // MyGO 包缓存
    TypedInfo        *TypedInfo              // 类型推理结果
}

// TypedInfo - 推理结果
type TypedInfo struct {
    ExprTypes      map[Expr]MonoType          // 表达式 -> 推断类型
    BindingSchemes map[string]*Scheme         // 绑定名 -> 泛化 Scheme
    Predicates     map[Expr][]Predicate       // 表达式 -> 类型类约束
    MyGoPackages   map[string]*MyGoPackageInfo // 导入的 MyGO 包
}

// PkgInfo - 包声明信息
type PkgInfo struct {
    Dir, WorkspaceRoot, Name string
    Decls          []Decl
    Enums          map[string]*EnumDecl
    Structs        map[string]*StructDecl
    Interfaces     map[string]*InterfaceDecl
    Funcs          map[string]*FuncDecl
    Impls          []*ImplDecl
    SourceFiles    map[any]string
    DotImportTypes map[string]struct{}
    DotImportEnums map[string]*EnumDecl
}

// GoPackageInfo - Go 包信息
type GoPackageInfo struct {
    Alias   string
    Path    string
    Funcs   map[string]TFunc
    Aliases map[string]string
}

// MyGoPackageInfo - MyGO 包信息
type MyGoPackageInfo struct {
    Alias, Path, Name string
    Funcs      map[string]*Scheme
    Types      map[string]struct{}
    Structs    map[string]*StructDecl
    Enums      map[string]*EnumDecl
    Interfaces map[string]*InterfaceDecl
    Impls      []*ImplDecl
}
```

### 新版现有结构（mygo 代码）

新版的 `InferState` 当前只有 `FreshVarID` 字段，`PackageInfo` 已包含部分推理结果但缺少 `BindingSchemes` 和 `Predicates` 的映射关系。

## 计划

### Step 1: 在 `types.mygo` 中添加新结构体

在 [`internal/mygo/typeinference2/types.mygo`](internal/mygo/typeinference2/types.mygo) 中添加：

#### 1.1 添加 `PkgInfo` 结构体

```mygo
struct PkgInfo
  Dir: String
  Name: String
  Decls: Slice[ast2.Decl]
end
```

说明：旧版的 `PkgInfo` 有 `Enums/Structs/Interfaces/Funcs/Impls` 等映射。由于 mygo 中 `ast2.Decl` 是枚举类型，无法直接存储单个变体类型的映射，因此采用存储完整 `Slice[ast2.Decl]` 的方式，需要时通过 pattern matching 查找。

#### 1.2 添加 `MyGoPackageInfo` 结构体

```mygo
struct MyGoPackageInfo
  Alias: String
  Path: String
  Name: String
  Funcs: Map[String, Scheme]
  Types: Slice[String]
end
```

说明：`Funcs` 映射函数名到 Scheme。`Types` 记录导出的类型名称（用于限定名处理）。

#### 1.3 添加 `GoPackageInfo` 结构体

```mygo
struct GoPackageInfo
  Alias: String
  Path: String
  Funcs: Map[String, TFunc]
end
```

说明：旧版 `GoPackageInfo` 包含 `Aliases`（类型别名映射），但在 mygo 中类型别名信息由外部 `go/types` 处理，此处先简化为 `Funcs` 映射。

> 注意：`TFunc` 是 `MonoType` 的一个变体，在 mygo 中用 `MonoType.TFunc(...)` 表示。

#### 1.4 添加 `TypedInfo` 结构体

```mygo
struct TypedInfo
  ExprTypes: Map[Ref[ast2.Expr], MonoType]
  BindingSchemes: Map[String, Scheme]
  Predicates: Map[Ref[ast2.Expr], Slice[Predicate]]
  MyGoPackages: Map[String, MyGoPackageInfo]
end
```

说明：
- `ExprTypes`：每个表达式节点 → 推断出的 `MonoType`，使用 `Ref[ast2.Expr]` 作为键（`ast2.Expr` 是 struct 值类型）
- `BindingSchemes`：变量/函数名 → 泛化后的 Scheme
- `Predicates`：每个表达式节点 → 生成的类型类约束列表
- `MyGoPackages`：别名 → 导入的 MyGO 包信息

#### 1.5 扩展 `InferState` 结构体

将当前的：
```mygo
struct InferState
  FreshVarID: Int
end
```

扩展为：
```mygo
struct InferState
  FreshVarID: Int
  PkgInfo: Option[PkgInfo]
  GoPackages: Map[String, GoPackageInfo]
  MyGoPackages: Map[String, MyGoPackageInfo]
  MyGoPackageCache: Map[String, MyGoPackageInfo]
  TypedInfo: Option[TypedInfo]
end
```

说明：
- 使用 `Option[T]` 表示可选字段（因为 `PkgInfo` 和 `TypedInfo` 可能在推理过程中才被设置）
- `Map[String, ...]` 对应旧版的 `map[string]*Xxx`

### Step 2: 更新 `NewInferState` 函数

更新 [`types.mygo`](internal/mygo/typeinference2/types.mygo) 中 `NewInferState()` 函数，初始化新字段：

```mygo
func NewInferState() -> InferState
  InferState {
    FreshVarID: 1,
    PkgInfo: None,
    GoPackages: MapNew[String, GoPackageInfo](),
    MyGoPackages: MapNew[String, MyGoPackageInfo](),
    MyGoPackageCache: MapNew[String, MyGoPackageInfo](),
    TypedInfo: None,
  }
end
```

> 注意：需要确认 mygo 中创建空 Map 的惯用方式。如果 `MapNew` 函数不可用，则可能需要使用 `GoMapNew` 或其他机制。

### Step 3: 更新 `infer.mygo` 中的推理入口函数

在 [`internal/mygo/typeinference2/infer.mygo`](internal/mygo/typeinference2/infer.mygo) 中：

#### 3.1 更新 `InferFile` 函数

在调用 `inferDecls` 之前，设置 `state.TypedInfo` 为新的 `TypedInfo`，并在推理完成后返回 `state.TypedInfo` 中的内容。

#### 3.2 更新 `InferPackage` / `InferPackageWithGoPackages` 函数

类似地，在入口处初始化 `TypedInfo`。

#### 3.3 更新 `inferExpr` 函数

在 `inferExpr` 中，当 `state.TypedInfo` 为 `Some` 时，记录每个表达式的推断结果到 `state.TypedInfo.ExprTypes` 中（类似旧版的模式）：

```mygo
# 在 inferExpr 内部，当成功推断后：
switch state.TypedInfo
  case Some(ti) =>
    # 记录表达式类型
    ti.ExprTypes.Set(Ref.new(expr), applySubst(value.Subst, value.Type))
    # 记录 predicates
    if value.Predicates.Len() > 0 then
      ti.Predicates.Set(Ref.new(expr), value.Predicates)
    end
  case None => ()
end
```

### Step 4: 更新 `PackageInfo` 结构

考虑是否将 `TypedInfo` 的内容合并到 `PackageInfo` 中，或者让 `PackageInfo` 保留 `TypedInfo` 字段。

推荐方案：在 `PackageInfo` 中添加 `TypedInfo` 字段：

```mygo
struct PackageInfo
  Env: Slice[EnvEntry]
  Fields: Slice[FieldEntry]
  GoPackages: Slice[GoPackageEntry]
  Instances: Slice[Instance]
  Solver: Solver
  ExprTypes: Map[Ref[ast2.Expr], String]
  TypedInfo: TypedInfo  # 新增
end
```

这样既能保留现有的 `ExprTypes: Map[Ref[ast2.Expr], String]`（字符串形式），又能通过 `TypedInfo` 获得完整的 `MonoType` 类型信息。

### Step 5: 重新生成 `.gen.go` 文件

运行 mygo 编译器重新生成 `zz_types.gen.go`、`zz_infer.gen.go` 等文件，确保 Go 代码与 mygo 源文件同步。

### Step 6: 编译验证

运行 `go build ./internal/mygo/typeinference2/...` 确保编译通过。

## 数据结构对应关系

| 旧版 (Go) | 新版 (mygo) | 说明 |
|-----------|-------------|------|
| `InferState.FreshVarID` | `InferState.FreshVarID` | 已存在 |
| `InferState.PkgInfo` | `InferState.PkgInfo: Option[PkgInfo]` | 新增 |
| `InferState.GoPackages` | `InferState.GoPackages: Map[String, GoPackageInfo]` | 新增 |
| `InferState.MyGoPackages` | `InferState.MyGoPackages: Map[String, MyGoPackageInfo]` | 新增 |
| `InferState.MyGoPackageCache` | `InferState.MyGoPackageCache: Map[String, MyGoPackageInfo]` | 新增 |
| `InferState.TypedInfo` | `InferState.TypedInfo: Option[TypedInfo]` | 新增 |
| `TypedInfo.ExprTypes` | `TypedInfo.ExprTypes: Map[Ref[ast2.Expr], MonoType]` | 新增 |
| `TypedInfo.BindingSchemes` | `TypedInfo.BindingSchemes: Map[String, Scheme]` | 新增 |
| `TypedInfo.Predicates` | `TypedInfo.Predicates: Map[Ref[ast2.Expr], Slice[Predicate]]` | 新增 |
| `TypedInfo.MyGoPackages` | `TypedInfo.MyGoPackages: Map[String, MyGoPackageInfo]` | 新增 |
| `PkgInfo` | `PkgInfo{ Dir, Name, Decls }` | 简化版 |
| `GoPackageInfo` | `GoPackageInfo{ Alias, Path, Funcs }` | 简化版 |
| `MyGoPackageInfo` | `MyGoPackageInfo{ Alias, Path, Name, Funcs, Types }` | 简化版 |

## 涉及的文件

1. **修改**: [`internal/mygo/typeinference2/types.mygo`](internal/mygo/typeinference2/types.mygo)
   - 添加 `PkgInfo`, `MyGoPackageInfo`, `GoPackageInfo`, `TypedInfo` 结构体
   - 扩展 `InferState` 结构体
   - 更新 `NewInferState()` 函数

2. **修改**: [`internal/mygo/typeinference2/infer.mygo`](internal/mygo/typeinference2/infer.mygo)
   - 更新 `inferExpr` 记录类型到 `TypedInfo`
   - 更新 `InferFile`/`InferPackage` 初始化 `TypedInfo`

3. **重新生成**: `zz_types.gen.go`, `zz_infer.gen.go` 等文件

## 注意事项

1. mygo 中的 `Map[K, V]` 直接映射到 Go 的 `map[K]V`，支持任意可比较的键类型
2. `Ref[ast2.Expr]` 作为 Map 键是安全的，因为 `Ref` 包装了 Go 指针
3. `Option[T]` 用于可选字段，对应 Go 中的零值判断或 `*T`
4. 旧版中 map 的值类型为指针（如 `*Scheme`），新版中使用值类型（如 `Scheme`），需要确认 mygo 编译器的支持
