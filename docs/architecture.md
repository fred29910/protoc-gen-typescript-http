# 架构

## 概述

```
┌─────────────┐     stdin      ┌──────────────────────────┐     stdout      ┌──────────────┐
│   protoc    │ ──────────────▶│ protoc-gen-typescript-http│ ──────────────▶│  index.ts    │
│   + buf     │  CodeGenerator  │                          │  CodeGenerator  │  (generated) │
│             │  Request        │        main.go           │  Response       │              │
└─────────────┘                 └──────────────────────────┘                 └──────────────┘
                                        │
                          ┌─────────────┼──────────────────┐
                          │             │                  │
                          ▼             ▼                  ▼
                   ┌──────────┐  ┌──────────┐    ┌──────────┐
                   │  plugin  │  │ httprule │    │ codegen  │
                   │ (core)   │  │ (parser) │    │ (writer) │
                   └──────────┘  └──────────┘    └──────────┘
                          │                           ┌──────────┐
                          └──────────────────────────▶│ protowalk│
                                                      │ (walker) │
                                                      └──────────┘
```

## 入口点：`main.go`

该二进制文件是一个标准的 protoc plugin，通过在 stdin/stdout 上序列化的 protobuf 进行通信：

1. 从 stdin 读取 `CodeGeneratorRequest`
2. 委托给 `plugin.Generate()`
3. 将 `CodeGeneratorResponse` 写入 stdout

## Package：`internal/plugin`

核心生成引擎。它负责：

- 从请求的 file descriptor 中构建 proto registry
- 按 protobuf package 对文件进行分组
- 为每个 package 生成一个 `index.ts` 输出文件，其中包含：
  - 所有 message 和 enum 的 TypeScript 类型定义
  - Service 接口
  - 一个 request handler 类型
  - 带有每个方法 HTTP 路由逻辑的 client 工厂函数

**关键文件：**

| 文件 | 用途 |
|------|---------|
| `generate.go` | 入口点，构建 registry，按 package 对文件进行分组，并协调生成过程 |
| `packagegen.go` | Package 级别的生成器，遍历 descriptor 并分发给专门的生成器 |
| `messagegen.go` | 为 protobuf message 生成 TypeScript 类型别名 |
| `enumgen.go` | 为 protobuf enum 生成 TypeScript 联合类型 |
| `servicegen.go` | 生成 service 接口、request handler 类型以及 client 实现 |
| `commentgen.go` | 提取并输出 protobuf 源码注释和 field behavior 注解 |
| `type.go` | 将 protobuf 字段类型映射为 TypeScript 类型 |
| `wellknown.go` | 处理 Google Well-Known Types（Any、Duration、Timestamp、Struct 等） |
| `jsonleafwalk.go` | 递归遍历 message 字段以发现 query-parameter 候选字段 |
| `helpers.go` | 用于类型命名、字段迭代、缩进的辅助函数 |

## Package：`internal/httprule`

将 `google.api.http` 注解模式解析为结构化的 URL 模板。

**关键类型：**

- `Template`，一个解析后的 URL 模板，包含 segment 和可选的 verb
- `Segment`，单个路径 segment（literal、wildcard、variable）
- `VariableSegment`，一个命名变量，带有可选的 sub-segment

该解析器实现了 `google.api.http` 中定义的语法：
```
Template = "/" Segments [ Verb ]
Segments = Segment { "/" Segment }
Segment  = "*" | "**" | LITERAL | Variable
Variable = "{" FieldPath [ "=" Segments ] "}"
FieldPath = IDENT { "." IDENT }
Verb     = ":" LITERAL
```

## Package：`internal/codegen`

一个极简的代码生成工具，提供了一个带有 `P(v ...interface{})` 方法的 buffered writer，用于输出格式化的行。

## Package：`internal/protowalk`

一个通用的 protobuf descriptor 树遍历器。它递归地访问文件、message、enum、service 和字段，并带有循环检测功能。`packagegen.go` 使用它来按正确的顺序遍历 descriptor。

## Package：`internal/httprule/template_test.go`

HTTP 模板解析器的单元测试。测试涵盖了所有 segment 类型（literal、wildcard、variable）的解析、校验规则、错误情况和边缘情况。
