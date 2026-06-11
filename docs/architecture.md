# 架构

## 概述

```
┌─────────────┐     stdin      ┌──────────────────────────┐     stdout      ┌──────────────┐
│   protoc    │ ──────────────▶│ protoc-gen-typescript-http│ ──────────────▶│  index.ts    │
│   + buf     │  CodeGenerator  │                          │  CodeGenerator  │  (generated) │
│             │  Request        │        main.go           │  Response       │              │
└─────────────┘                 └──────────────────────────┘                 └──────────────┘
                                        │
                                        ▼
                            ┌───────────────────────┐
                            │   internal/plugin     │
                            │  ┌──────────────────┐ │
                            │  │   generate.go    │ │  ← 入口：分组、调度
                            │  │  packagegen.go   │ │  ← 通过 protowalk 遍历
                            │  │  ┌──────────────┐│ │
                            │  │  │ messagegen   ││ │
                            │  │  │   enumgen    ││ │  ← 专门的类型生成器
                            │  │  │  servicegen  ││ │
                            │  │  │  commentgen  ││ │
                            │  │  │  wellknown   ││ │
                            │  │  │  jsonleafwalk││ │
                            │  │  │   helpers    ││ │
                            │  │  └──────────────┘│ │
                            │  └──────────────────┘ │
                            └───────────────────────┘
                              │                    │
                              ▼                    ▼
                    ┌──────────────────┐   ┌──────────────────┐
                    │ internal/httprule│   │ internal/codegen │
                    │  (URL 模板解析)  │   │  (buffer writer) │
                    └──────────────────┘   └──────────────────┘
                              ▲
                              │
                    ┌──────────────────┐
                    │internal/protowalk│
                    │ (descriptor 遍历)│
                    └──────────────────┘
```

数据流要点：

- **入口**：`main.go` 仅负责 stdin/stdout 协议反序列化和顶层错误处理。
- **调度**：`internal/plugin/generate.go` 在收到请求后构建 proto registry，按 protobuf package 对文件分组，并委托给 `packageGenerator`。
- **遍历**：`internal/plugin/packagegen.go` 通过 `internal/protowalk.WalkFiles` 递归访问 file、message、enum、service 和 method descriptor（带循环检测）。
- **分发**：在 walk 的回调中根据 descriptor 类型分发给 `messageGenerator` / `enumGenerator` / `serviceGenerator`。
- **解析**：`serviceGenerator` 通过 `internal/httprule` 把每个 RPC 的 `google.api.http` 注解解析为结构化 `Rule`。
- **输出**：所有 generator 共用 `internal/codegen.File` 的 buffered writer，每个 package 产出一个 `index.ts`。

## 入口点：`main.go`

该二进制文件是一个标准的 protoc plugin，通过在 stdin/stdout 上序列化的 protobuf 进行通信：

1. 从 stdin 读取 `CodeGeneratorRequest`
2. 委托给 `plugin.Generate()`
3. 将 `CodeGeneratorResponse` 写入 stdout

## Package：`internal/plugin`

核心生成引擎。它负责：

- 从请求的 file descriptor 中构建 proto registry
- 按 protobuf package 对文件进行分组
- 通过 `protowalk` 遍历 descriptor 树
- 识别 Google Well-Known Types（`wellknown.go`），并在遇到时输出对应的 TypeScript 类型声明
- 为每个 package 生成一个 `index.ts` 输出文件，其中包含：
  - 所有 message 和 enum 的 TypeScript 类型定义
  - Service 接口
  - 一个 request handler 类型
  - 带有每个方法 HTTP 路由逻辑的 client 工厂函数

**关键文件：**

| 文件 | 用途 |
|------|---------|
| `generate.go` | 入口点，构建 registry，按 package 对文件进行分组，并协调生成过程 |
| `packagegen.go` | Package 级别的生成器，遍历 descriptor 并分发给专门的生成器；遇到 WKT 时优先输出 `TypeDeclaration()` |
| `messagegen.go` | 为 protobuf message 生成 TypeScript 类型别名 |
| `enumgen.go` | 为 protobuf enum 生成 TypeScript 联合类型 |
| `servicegen.go` | 生成 service 接口、request handler 类型以及 client 实现 |
| `commentgen.go` | 提取并输出 protobuf 源码注释和 field behavior 注解 |
| `type.go` | 将 protobuf 字段类型映射为 TypeScript 类型（scalar、enum、message、map、list） |
| `wellknown.go` | 识别 Google Well-Known Types，并为每个 WKT 输出 TypeScript 类型声明（含 JSON 编码注释） |
| `jsonleafwalk.go` | 递归遍历 message 字段以发现 query-parameter 候选字段（带 cycle detection） |
| `helpers.go` | 用于类型命名（跨包前缀、嵌套扁平化）、字段迭代、缩进的辅助函数 |

### Well-Known Types 处理

WKT 不通过通用的 `messageGenerator` 处理。当 `protowalk` 回调中遇到一个 `FullName` 以 `google.protobuf.` 为前缀的 `MessageDescriptor` 或 `EnumDescriptor` 时，`packagegen.go` 会跳过通用的 message/enum 生成，改而调用 `WellKnownType(desc).TypeDeclaration()` 输出对应的 TypeScript 类型声明。每个 WKT 类型都会附带从 [官方规范](https://protobuf.dev/reference/protobuf/google.protobuf/) 派生的 JSON 编码说明注释。

详见 [code-generation.md § Well-Known Types](./code-generation.md)。

## Package：`internal/httprule`

将 `google.api.http` 注解模式解析为结构化的 URL 模板。

**关键类型：**

- `Rule`：解析后的 HTTP 绑定，包含 `Method`（`GET`/`POST`/... 或 custom）、`Template`、`Body` selector 和 `AdditionalRules`。
- `Template`：解析后的 URL 模板，包含 segment 和可选的 verb。
- `Segment`：单个路径 segment（literal、wildcard、variable）。
- `VariableSegment`：一个命名变量，带有可选的 sub-segment。
- `FieldPath`：snake_case 形式的字段路径（与 `.proto` 源一致）。

该解析器实现了 `google.api.http` 中定义的语法：

```
Template = "/" Segments [ Verb ]
Segments = Segment { "/" Segment }
Segment  = "*" | "**" | LITERAL | Variable
Variable = "{" FieldPath [ "=" Segments ] "}"
FieldPath = IDENT { "." IDENT }
Verb     = ":" LITERAL
```

详细的 API 与 grammar 规则、validation 行为、custom HTTP method 的解析见 [http-rule-parsing.md](./http-rule-parsing.md)。

## Package：`internal/codegen`

一个极简的代码生成工具，提供了一个带有 `P(v ...interface{})` 方法的 buffered writer，用于输出格式化的行。所有 generator 都通过这个统一的 writer 输出来保证缩进与换行风格一致。

## Package：`internal/protowalk`

一个通用的 protobuf descriptor 树遍历器。它递归地访问文件、message、enum、service 和字段，并带有循环检测功能（按 descriptor FullName 去重）。`packagegen.go` 使用它来按正确的顺序遍历 descriptor，避免在递归 message 时进入循环。

## Package：`internal/httprule`（测试）

`template_test.go` 覆盖 HTTP 模板解析器的全部 segment 类型（literal、wildcard、variable）、自定义 verb 以及 validation 规则（嵌套变量、`**` 必须在末位、顶层禁止裸 `*`/`**`、变量重复绑定）的解析、错误和边缘情况。

`internal/plugin/servicegen_test.go` 覆盖 `servicegen.go` 的两个核心 helper：`isWildcardVariable`（判断是否为带子模板的通配符变量，决定是否保留语义斜杠）与 `pathStartsWith`（判断嵌套 path 是否以 body selector 为前缀）。
