---
name: protoc-gen-typescript-http
description: >
  AI Agent 配置文档 — protoc-gen-typescript-http 项目分析、架构说明与开发指南。
  涵盖项目概述、代码结构、功能地图、依赖分析、代码质量、关键算法、安全性、可扩展性等内容。
---

# protoc-gen-typescript-http — AI Agent 配置文档

**生成时间:** 2026-06-17 | **项目版本:** 实验性 | **Go 版本:** 1.25.7+

---

## 1. 项目概述

### 1.1 项目定位

`protoc-gen-typescript-http` 是一个 **protoc 插件**（同时也是 buf 插件），用于从带有 `google.api.http` 规则注解的 Protobuf 定义中生成 **TypeScript 类型**和 **HTTP 服务客户端**。

核心目标：让 go-kratos 微服务生态中的 Protobuf API 能够自动生成类型安全的 TypeScript 客户端，实现前后端类型一致性。

### 1.2 技术栈

| 层级 | 技术 | 版本 |
|------|------|------|
| 编程语言 | Go | 1.25.7+ |
| 构建工具 | Mage | v1.17.2 |
| 代码生成 | protoc / buf | — |
| 发布工具 | GoReleaser | — |
| 测试框架 | gotest.tools/v3 | v3.5.2 |
| Protobuf 库 | google.golang.org/protobuf | v1.36.11 |
| HTTP 注解 | google.golang.org/genproto/googleapis/api | v0.0.0-20260209200024 |

### 1.3 许可证

**MIT License** — Copyright 2020 Einride AB（仓库已 fork 至 `fred29910/protoc-gen-typescript-http`）

### 1.4 项目活跃度

| 指标 | 数据 |
|------|------|
| 总贡献者 | 10+ |
| 核心贡献者 | fred29910 (107 commits), ericwenn (50 commits) |
| 最近提交 | 2026-06-02 |
| 当前分支 | `feat/ts_type_op` |
| 开发状态 | **实验性** — 积极开发中，API 可能有破坏性变更 |

---

## 2. 代码结构分析

### 2.1 目录结构

```
protoc-gen-typescript-http/
├── main.go                          # 入口：stdin/stdout 协议通信
├── magefile.go                      # Mage 任务定义
├── Makefile                         # 构建包装器（工具安装 + 任务委托）
├── go.mod / go.sum                   # Go 模块依赖
├── .goreleaser.yml                  # 跨平台发布配置
├── internal/
│   ├── codegen/
│   │   └── file.go                 # 代码生成缓冲区 writer
│   ├── httprule/
│   │   ├── rule.go                 # HTTP 注解解析入口
│   │   ├── template.go             # URL 模板解析器（递归下降）
│   │   ├── template_test.go        # 模板解析器单元测试
│   │   └── fieldpath.go            # 字段路径类型
│   ├── plugin/
│   │   ├── generate.go             # 生成调度器（分组 + 协调）
│   │   ├── packagegen.go           # Package 级别生成器（protowalk 遍历）
│   │   ├── messagegen.go           # Message → TypeScript type 生成
│   │   ├── enumgen.go              # Enum → TypeScript union type 生成
│   │   ├── servicegen.go           # Service 接口 + 客户端工厂生成
│   │   ├── servicegen_test.go      # 服务生成核心 helper 测试
│   │   ├── commentgen.go           # Protobuf 注释传播
│   │   ├── type.go                 # Protobuf → TypeScript 类型映射
│   │   ├── wellknown.go            # Google Well-Known Types 处理
│   │   ├── jsonleafwalk.go         # JSON 叶子字段遍历器（query 参数发现）
│   │   ├── jsonleafwalk_test.go    # 遍历器测试（含 P0 bug 回归）
│   │   └── helpers.go              # 类型命名、迭代工具
│   └── protowalk/
│       └── walk.go                 # Protobuf descriptor 树遍历器
├── tests/
│   └── integration/
│       └── integration_test.go     # 集成测试（buf generate + git diff 校验）
└── examples/proto/                 # 示例 proto 定义 + 生成的 TypeScript
```

### 2.2 代码规模

| 指标 | 数值 |
|------|------|
| Go 源文件 | 19 个 |
| Go 代码行数 | ~2,716 行 |
| 测试代码行数 | ~841 行 |
| 测试覆盖率（代码行） | ~31% |
| 文档文件 | 12+ 个 Markdown |

### 2.3 架构模式

项目采用 **管道-过滤器（Pipeline-Filter）** 模式：

```
┌──────────┐    stdin     ┌────────────────────────────┐    stdout    ┌──────────────┐
│  protoc  │ ──────────▶ │  main.go → plugin.Generate │ ──────────▶ │  index.ts    │
│  + buf   │             │         │                  │             │  (generated) │
└──────────┘             │    ┌────▼─────┐            │             └──────────────┘
                         │    │ protowalk│            │
                         │    │  .Walk   │            │
                         │    └────┬─────┘            │
                         │    ┌────▼─────┐            │
                         │    │ dispatch │            │
                         │    │ ┌──────┐ │            │
                         │    │ │ msg  │ │            │
                         │    │ │ enum │ │            │
                         │    │ │ svc  │ │            │
                         │    │ └──────┘ │            │
                         │    └──────────┘            │
                         └────────────────────────────┘
```

**设计模式：**

- **访问者模式（Visitor）**：`protowalk.WalkFiles` 遍历 descriptor 树，在回调中分发给各生成器
- **策略模式（Strategy）**：`messageGenerator`、`enumGenerator`、`serviceGenerator` 各自实现 `Generate(f *codegen.File)` 接口
- **模板方法（Template Method）**：`servicegen.go` 中的 `writeMethodDispatchBody` 实现了单绑定快速路径和多绑定分发两条路径
- **Builder 模式**：`codegen.File` 的 `P(v ...interface{})` 方法链式构建输出

---

## 3. 功能地图

### 3.1 核心功能

| 功能 | 描述 | 实现文件 |
|------|------|----------|
| TypeScript 类型生成 | Protobuf message → TypeScript `type` 别名 | `messagegen.go` |
| 枚举生成 | Protobuf enum → TypeScript 字符串字面量联合类型 | `enumgen.go` |
| 服务接口生成 | Protobuf service → TypeScript `interface` | `servicegen.go` |
| 客户端工厂 | `create<Service>Client(handler)` 函数 | `servicegen.go` |
| HTTP 规则解析 | `google.api.http` 注解 → 结构化 `Rule` | `httprule/rule.go` |
| URL 模板解析 | HTTP 路径模板 → 分段 Template | `httprule/template.go` |
| Well-Known Types | Google WKT → TypeScript 类型声明 | `wellknown.go` |
| 注释传播 | Protobuf 源码注释 → TypeScript 注释 | `commentgen.go` |
| 跨包引用 | 跨包类型名使用扁平化前缀限定 | `helpers.go` |
| Query 参数发现 | 递归遍历 message 发现 query 候选字段 | `jsonleafwalk.go` |

### 3.2 数据流

```
CodeGeneratorRequest
  │
  ▼
generate.go: 构建 Proto Registry
  │
  ▼
按 Package 分组 FileDescriptor
  │
  ▼
packagegen.go: WalkFiles 遍历 descriptor 树
  │
  ├─▶ WellKnownType? → wellknown.go: TypeDeclaration()
  ├─▶ MessageDescriptor → messagegen.go: Generate()
  ├─▶ EnumDescriptor → enumgen.go: Generate()
  └─▶ ServiceDescriptor → servicegen.go: Generate()
        │
        ├─▶ generateInterface() → TypeScript interface
        ├─▶ generateHandler() → RequestHandler type
        └─▶ generateClient() → createServiceClient()
              │
              ├─▶ generateMethod() → httprule.Get() + ParseRule()
              ├─▶ writeMethodDispatchBody()
              │     ├─ 单绑定快速路径
              │     └─ 多绑定 if/else if/else 分发
              ├─▶ generateMethodBinding()
              │     ├─ generateMethodPathValidation()
              │     ├─ generateMethodPath()
              │     ├─ generateMethodBody()
              │     └─ generateMethodQuery() → jsonleafwalk
              └─▶ writeMethodHandlerCall()
```

### 3.3 支持的 HTTP 方法

| HTTP 方法 | Protobuf 注解 | 支持状态 |
|-----------|--------------|----------|
| GET | `option (google.api.http) = {get: "..."}` | ✅ |
| POST | `option (google.api.http) = {post: "..."}` | ✅ |
| PUT | `option (google.api.http) = {put: "..."}` | ✅ |
| DELETE | `option (google.api.http) = {delete: "..."}` | ✅ |
| PATCH | `option (google.api.http) = {patch: "..."}` | ✅ |
| Custom | `option (google.api.http) = {custom: {kind: "...", path: "..."}}` | ✅ |

### 3.4 示例 API 分析

项目包含 4 个示例 Protobuf 包：

| 示例包 | 服务 | 方法数 | 说明 |
|--------|------|--------|------|
| `einride.example.freight.v1` | FreightService | 12 | 完整 CRUD REST API |
| `einride.example.syntax.v1` | SyntaxService | 6 | 所有 protobuf 特性演示 |
| `einride.example.syntax.v2` | — | — | 跨包引用演示 |
| `einride.example.additional_bindings.v1` | — | — | additional_bindings 演示 |

---

## 4. 依赖关系分析

### 4.1 外部依赖

| 依赖 | 版本 | 用途 | 风险评估 |
|------|------|------|----------|
| `google.golang.org/protobuf` | v1.36.11 | Protobuf 反射、descriptor 解析 | 低 — 官方维护，稳定 |
| `google.golang.org/genproto/googleapis/api` | v0.0.0-20260209200024 | `google.api.http` 注解定义 | 低 — 官方维护 |
| `github.com/magefile/mage` | v1.17.2 | 构建任务管理 | 低 — 成熟工具 |
| `gotest.tools/v3` | v3.5.2 | 测试断言库 | 低 — 广泛使用 |
| `github.com/google/go-cmp` | v0.7.0 (indirect) | 测试比较 | 低 |

### 4.2 内部模块依赖图

```
main.go
  └── internal/plugin
        ├── internal/codegen        (缓冲区 writer)
        ├── internal/httprule       (HTTP 规则解析)
        │     └── fieldpath.go      (字段路径)
        ├── internal/protowalk      (descriptor 遍历)
        │     └── 被 packagegen.go 使用
        └── 自身模块间：
              ├── packagegen.go → messagegen.go, enumgen.go, servicegen.go
              ├── servicegen.go → httprule (解析 HTTP 注解)
              ├── servicegen.go → jsonleafwalk (query 参数发现)
              └── messagegen.go, enumgen.go, servicegen.go → type.go, helpers.go, commentgen.go
```

### 4.3 依赖风险评估

- ✅ **依赖数量极少**：仅 3 个直接依赖 + 1 个间接依赖
- ✅ **无第三方框架依赖**：仅依赖 Google 官方 Protobuf 库
- ⚠️ **genproto 版本较新**：`v0.0.0-20260209200024` 为 2026 年 2 月版本，可能存在兼容性问题
- ⚠️ **Go 版本要求高**：需要 Go 1.25.7+，较新

---

## 5. 代码质量评估

### 5.1 代码可读性

| 维度 | 评分 | 说明 |
|------|------|------|
| 命名规范 | ⭐⭐⭐⭐⭐ | Go 标准命名，一致性强 |
| 函数长度 | ⭐⭐⭐⭐ | 大部分函数简短，`servicegen.go` 中部分函数较长 |
| 错误处理 | ⭐⭐⭐⭐⭐ | 统一使用 `fmt.Errorf("context: %w", err)` 包装 |
| 代码注释 | ⭐⭐⭐⭐ | 关键逻辑有注释，generator 有详细文档字符串 |

### 5.2 注释和文档

| 文档 | 状态 | 质量 |
|------|------|------|
| README.md | ✅ 完整 | 包含安装、使用、生成输出说明 |
| docs/architecture.md | ✅ 详细 | 含 ASCII 架构图和数据流说明 |
| docs/code-generation.md | ✅ 详细 | 完整的代码生成参考 |
| docs/development.md | ✅ 详细 | 构建、测试、CI/CD 说明 |
| docs/http-rule-parsing.md | ✅ 存在 | HTTP 规则解析文档 |
| docs/examples.md | ✅ 详细 | 示例 proto 和生成输出 |
| docs/protobuf-annotations.md | ✅ 存在 | Protobuf 注解参考 |
| docs/ci-cd.md | ✅ 存在 | CI/CD 说明 |
| 代码注释 | ✅ 良好 | 关键函数有详细注释 |

### 5.3 测试覆盖

| 测试文件 | 覆盖内容 | 测试用例数 |
|----------|----------|-----------|
| `template_test.go` | URL 模板解析器 | ~15 |
| `servicegen_test.go` | 服务生成 helper + 多绑定分发 | ~12 |
| `jsonleafwalk_test.go` | JSON 叶子遍历 + P0 bug 回归 | ~4 |
| `integration_test.go` | 端到端集成测试 | 1 |

**测试覆盖率估算：** ~31%（841 行测试 / 2,716 行源码）

### 5.4 代码异味和改进空间

| 异味 | 位置 | 严重程度 | 建议 |
|------|------|----------|------|
| `wellknown.go` 使用自定义 `writer` 而非 `strings.Builder` | `wellknown.go:143-157` | 低 | 可统一使用 `strings.Builder` |
| `servicegen.go` 函数较长（`generateMethod` 相关） | `servicegen.go` | 中 | 可进一步拆分 |
| 测试中大量重复构建 `FileDescriptorProto` | `jsonleafwalk_test.go` | 中 | 可提取公共 test helper |
| 缺少 `go.mod` 中的 `retract` 指令 | `go.mod` | 低 | 实验性项目可考虑添加 |
| 集成测试依赖外部工具（buf, deno） | `integration_test.go` | 中 | 可添加 skip 条件 |

---

## 6. 关键算法和数据结构

### 6.1 URL 模板解析器

**位置：** `internal/httprule/template.go`

**算法：** 递归下降解析器（Recursive Descent Parser）

```
Template = "/" Segments [ Verb ]
Segments = Segment { "/" Segment }
Segment  = "*" | "**" | LITERAL | Variable
Variable = "{" FieldPath [ "=" Segments ] "}"
FieldPath = IDENT { "." IDENT }
Verb     = ":" LITERAL
```

**关键数据结构：**

```go
type Template struct {
    Segments []Segment
    Verb     string
}

type Segment struct {
    Kind     SegmentKind  // Literal, MatchSingle, MatchMultiple, Variable
    Literal  string
    Variable VariableSegment
}

type VariableSegment struct {
    FieldPath FieldPath    // []string, snake_case
    Segments  []Segment    // 可选的子模板
}
```

**验证规则：**
- 禁止嵌套变量段
- `**` 仅允许在模板末尾
- 顶层禁止裸 `*` / `**`
- 禁止重复变量绑定

### 6.2 JSON 叶子字段遍历器

**位置：** `internal/plugin/jsonleafwalk.go`

**算法：** 深度优先遍历 + 循环检测（基于 `ancestors` 集合）

**关键数据结构：**

```go
type jsonWalker struct {
    ancestors map[protoreflect.FullName]struct{}
}
```

**核心逻辑：**
- `enter(name)` → 检查是否已访问，防止循环（自引用 message 不会无限递归）
- `leave(name)` → 离开节点时从 ancestors 中移除（允许同一类型在不同分支中重复出现）
- 对 Well-Known Types 直接作为叶子处理，不递归进入

**P0 Bug 修复历史：** 修复了同一 message 类型在多个兄弟字段中使用时（如 `Address source` + `Address destination`），第二个字段的叶子被全局 `seen` 集合错误丢弃的问题。

### 6.3 类型映射系统

**位置：** `internal/plugin/type.go`

**核心数据结构：**

```go
type Type struct {
    IsNamed   bool
    Name      string
    IsList    bool
    IsMap     bool
    Underlying *Type
}
```

**映射规则：**

| Protobuf Kind | TypeScript |
|---------------|------------|
| string, bytes | `string` |
| bool | `boolean` |
| 所有数值类型 | `number` |
| enum | 作用域类型名 |
| message | 作用域类型名 |
| repeated T | `T[]` |
| map<K,V> | `{ [key: string]: V }` |

### 6.4 多绑定分发

**位置：** `internal/plugin/servicegen.go:writeMethodDispatchBody`

**算法：** 单绑定快速路径 + 多绑定 if/else if/else 分发

- **单绑定（快速路径）：** 直接生成路径验证 → 路径构建 → body 构建 → query 构建 → handler 调用
- **多绑定：** 为每个 binding 生成 `if (pathVarsPresent) { ... }` 条件分支，无匹配时抛出 `Error("no matching binding for <Method>")`

---

## 7. 函数调用图

### 7.1 主要入口函数

```
main()
  └── run()
        ├── io.ReadAll(os.Stdin)
        ├── proto.Unmarshal()
        ├── plugin.Generate()           ← 核心入口
        │     ├── protodesc.NewFiles()
        │     ├── packageGenerator.Generate()
        │     │     ├── generateHeader()
        │     │     └── protowalk.WalkFiles()
        │     │           ├── wellknown.TypeDeclaration()  (WKT)
        │     │           ├── messageGenerator.Generate()
        │     │           │     ├── commentGenerator.generateLeading()
        │     │           │     ├── typeFromField()
        │     │           │     │     └── namedTypeFromField()
        │     │           │     │           └── typeFromMessage()
        │     │           │     └── f.P() (输出)
        │     │           ├── enumGenerator.Generate()
        │     │           └── serviceGenerator.Generate()
        │     │                 ├── generateInterface()
        │     │                 ├── generateHandler()
        │     │                 └── generateClient()
        │     │                       └── generateMethod()
        │     │                             ├── httprule.Get()
        │     │                             ├── httprule.ParseRule()
        │     │                             │     ├── httpRuleMethod()
        │     │                             │     ├── httpRuleURL()
        │     │                             │     └── ParseTemplate()
        │     │                             │           └── parser.parse()
        │     │                             │                 └── validate()
        │     │                             └── writeMethodDispatchBody()
        │     │                                   ├── generateMethodBinding()
        │     │                                   │     ├── generateMethodPathValidation()
        │     │                                   │     ├── generateMethodPath()
        │     │                                   │     ├── generateMethodBody()
        │     │                                   │     └── generateMethodQuery()
        │     │                                   │           └── walkJSONLeafFields()
        │     │                                   └── writeMethodHandlerCall()
        │     └── proto.Marshal()
        └── os.Stdout.Write()
```

### 7.2 高频调用路径

1. **类型解析路径：** `typeFromField` → `namedTypeFromField` → `typeFromMessage` → `WellKnownType` / `scopedDescriptorTypeName`
2. **代码输出路径：** 所有 generator → `codegen.File.P()` → `bytes.Buffer`
3. **模板解析路径：** `ParseTemplate` → `parser.parse()` → `parseSegments()` → `parseSegment()` → `parseVariableSegment()`

---

## 8. 安全性分析

### 8.1 安全评估

| 维度 | 评估 | 说明 |
|------|------|------|
| 输入验证 | ✅ 安全 | Protobuf 官方库处理输入解析，有完善的验证 |
| 注入风险 | ⚠️ 低风险 | 生成的 TypeScript 代码中使用 `encodeURIComponent` 编码路径变量和 query 参数，但 `JSON.stringify` 序列化 body 时未做额外过滤 |
| 敏感数据 | ✅ 无 | 不处理认证、密钥等敏感数据 |
| 依赖安全 | ✅ 安全 | 依赖极少，均为 Google 官方库 |
| 文件操作 | ✅ 安全 | 仅读写 stdin/stdout，不直接操作文件系统 |

### 8.2 潜在安全风险

| 风险 | 严重程度 | 说明 |
|------|----------|------|
| 生成的 `eval` 风险 | 无 | 不生成 `eval` 或动态代码执行 |
| 路径遍历 | 低 | 生成的代码使用 `encodeURIComponent` 编码路径变量，可防止路径注入 |
| 64位整数精度丢失 | 中 | `int64`/`uint64` 映射为 `number`，超出 `2^53-1` 会丢失精度 |
| 未转义的 template literal | 低 | 生成的代码中使用反引号模板 literal，但内容来自 `encodeURIComponent` 或 `strconv.Quote` |

---

## 9. 可扩展性和性能

### 9.1 可扩展性评估

| 维度 | 评分 | 说明 |
|------|------|------|
| 新增 Protobuf 特性 | ⭐⭐⭐⭐ | 通过添加新的 generator 或扩展现有 generator 即可支持 |
| 新增输出格式 | ⭐⭐⭐ | 当前仅支持 TypeScript，扩展其他语言需要重构 `codegen.File` |
| 配置灵活性 | ⭐⭐⭐ | 当前无配置选项（如自定义类型映射），所有行为硬编码 |
| 插件生态 | ⭐⭐⭐⭐ | 同时支持 protoc 和 buf 插件体系 |

### 9.2 性能分析

| 维度 | 评估 | 说明 |
|------|------|------|
| 生成速度 | ✅ 快 | 纯内存操作，无 I/O 瓶颈 |
| 内存使用 | ✅ 低 | 使用 `bytes.Buffer` 流式输出 |
| 并发安全 | ✅ 安全 | 无共享状态，每次请求独立处理 |
| 瓶颈 | 无 | 对于大型 proto 文件集，descriptor 构建可能较慢 |

### 9.3 已知限制

| 限制 | 影响 |
|------|------|
| 不支持 client-streaming / server-streaming | 无法生成 gRPC-Web streaming 客户端 |
| 64 位整数映射为 number | 大整数精度丢失 |
| oneof 无法表达互斥语义 | 调用方需自行保证 |
| 生成的 URL 不包含前导 `/` | 调用方需自行拼接 root URL |
| `RequestHandler` 未导出 | 用户需自行声明等效类型 |

---

## 10. 总结和建议

### 10.1 项目整体评价

| 维度 | 评分 | 说明 |
|------|------|------|
| 代码质量 | ⭐⭐⭐⭐ | Go 代码规范、清晰，错误处理一致 |
| 架构设计 | ⭐⭐⭐⭐⭐ | 管道-过滤器模式，职责分离清晰 |
| 文档完整性 | ⭐⭐⭐⭐⭐ | 架构、代码生成、开发指南齐全 |
| 测试覆盖 | ⭐⭐⭐ | 核心逻辑有测试，但覆盖率可提升 |
| 工程化程度 | ⭐⭐⭐⭐ | Mage + Makefile + GoReleaser + CI/CD 完善 |

### 10.2 主要优势

- ✅ **架构清晰**：模块职责单一，依赖关系简单
- ✅ **文档齐全**：架构图、代码生成参考、开发指南一应俱全
- ✅ **标准兼容**：遵循 Protobuf JSON 编码规范、Google AIP 规范
- ✅ **HTTP 客户端无关**：通过 `RequestHandler` 抽象解耦
- ✅ **Well-Known Types 支持完整**：覆盖所有 Google WKT
- ✅ **工程化完善**：CI/CD、集成测试、自动化发布

### 10.3 改进建议

| 优先级 | 建议 | 说明 |
|--------|------|------|
| 🔴 高 | 提升测试覆盖率 | 当前 ~31%，建议提升至 60%+，补充 `generate.go`、`packagegen.go`、`messagegen.go` 的测试 |
| 🔴 高 | 64 位整数精度问题 | 考虑将 `int64`/`uint64` 映射为 `string`（符合 Protobuf JSON 规范建议） |
| 🟡 中 | 导出 `RequestHandler` 类型 | 方便用户直接使用生成的类型 |
| 🟡 中 | 支持 gRPC-Web streaming | 扩展 streaming 方法支持 |
| 🟡 中 | 添加配置选项 | 如自定义类型映射、输出格式选项 |
| 🟢 低 | 统一 `wellknown.go` 的 writer | 使用标准 `strings.Builder` |
| 🟢 低 | 提取测试公共 helper | 减少 `jsonleafwalk_test.go` 中的重复代码 |

---

## 附录 A：快速参考

### 构建命令

```bash
make build       # 构建插件二进制
make test        # 运行单元测试
make vet         # 运行 go vet
make fmt         # 运行 go fmt
make integration # 运行集成测试
make ci          # 完整 CI 流水线
make clean       # 清理构建产物
```

### 关键文件速查

| 需求 | 文件 |
|------|------|
| 修改生成逻辑 | `internal/plugin/generate.go` |
| 添加新的类型映射 | `internal/plugin/type.go` |
| 修改 HTTP 规则解析 | `internal/httprule/rule.go` |
| 修改 URL 模板解析 | `internal/httprule/template.go` |
| 修改服务生成 | `internal/plugin/servicegen.go` |
| 修改 Well-Known Types | `internal/plugin/wellknown.go` |
| 修改注释传播 | `internal/plugin/commentgen.go` |
| 添加示例 | `examples/proto/` |

### 相关文档

- [架构概览](architecture.md)
- [代码生成详细参考](code-generation.md)
- [HTTP 规则解析](http-rule-parsing.md)
- [Protobuf 注解](protobuf-annotations.md)
- [开发指南](development.md)
- [示例](examples.md)
- [CI/CD](ci-cd.md)
