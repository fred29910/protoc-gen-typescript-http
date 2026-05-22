# 开发指南

## 前提条件

- **Go** 1.25.7+
- **[buf](https://buf.build/docs/installation)**，用于构建和校验示例 proto 文件
- **[mage](https://magefile.org/)**，任务运行器（可选，可通过 `go run github.com/magefile/mage` 运行）

## 项目结构

```
.
├── main.go                    # Protoc plugin entry point
├── magefile.go                # Mage task definitions
├── Makefile                   # Optional Makefile wrapper
├── go.mod / go.sum            # Go module dependencies
├── .goreleaser.yml            # GoReleaser configuration
├── internal/
│   ├── codegen/               # Simple code generation buffer
│   │   └── file.go
│   ├── httprule/              # HTTP annotation parser
│   │   ├── rule.go
│   │   ├── template.go
│   │   ├── template_test.go   # Parser unit tests
│   │   └── fieldpath.go
│   ├── plugin/                # Core generation engine
│   │   ├── generate.go        # Main generation orchestrator
│   │   ├── packagegen.go      # Package-level dispatcher
│   │   ├── messagegen.go      # Message type generation
│   │   ├── enumgen.go         # Enum type generation
│   │   ├── servicegen.go      # Service interface & client generation
│   │   ├── commentgen.go      # Protobuf comment extraction
│   │   ├── type.go            # Protobuf→TypeScript type mapping
│   │   ├── wellknown.go       # Well-Known Type handling
│   │   ├── jsonleafwalk.go    # JSON leaf field walker
│   │   └── helpers.go         # Type naming & iteration utilities
│   └── protowalk/             # Protobuf descriptor tree walker
│       └── walk.go
├── tests/
│   └── integration/           # Integration tests (build tag: integration)
│       └── integration_test.go
├── examples/
│   └── proto/                 # Example proto definitions and generated code
│       ├── buf.gen.yaml       # Buf code generation config
│       ├── buf.yaml           # Buf module config
│       └── einride/example/   # Example protos
└── docs/                      # Documentation
    └── ...
```

## 可用任务

所有项目任务都由 [Mage](https://magefile.org/) 管理：

| 命令 | 描述 |
|---|---|
| `mage build` | 构建插件二进制文件到 `bin/protoc-gen-typescript-http` |
| `mage test` | 运行单元测试 |
| `mage integration` | 运行集成测试（完整构建 + 代码生成 + 差异验证） |
| `mage clean` | 清理构建产物 |

或者使用 Makefile 包装器：

```bash
make build
make test
make integration
make clean
```

## 工作流

### 构建

```bash
mage build
```

二进制文件会输出到 `bin/protoc-gen-typescript-http`。

### 运行单元测试

```bash
mage test
```

### 运行集成测试

集成测试步骤：
1. 构建插件二进制文件
2. 使用构建好的插件在 `examples/proto/` 中运行 `buf generate`
3. 通过 `git diff --exit-code` 验证生成的代码与已提交的代码一致

```bash
mage integration
```

### 直接使用插件

```bash
protoc \
  --typescript-http_out=./output \
  --proto_path=./protos \
  ./protos/*.proto
```

或者使用 buf：

```bash
cd examples/proto
buf generate
```

### 添加新示例

1. 在 `examples/proto/` 下添加你的 `.proto` 文件
2. 生成 TypeScript 代码：在 `examples/proto/` 目录下运行 `buf generate`
3. 验证生成的输出：`git diff --exit-code examples/proto/gen/typescript`
4. 提交 `.proto` 文件和生成的 `.ts` 文件

## 代码规范

- **Go**：标准的 `gofmt` 格式化，使用 `fmt.Errorf("context: %w", err)` 进行错误包装
- **Protobuf**：在示例中遵循 [Google AIP](https://google.aip.dev/) 规范
- **生成的 TypeScript**：使用 `@ts-nocheck` 和 `eslint-disable camelcase` 来抑制 proto 命名风格带来的警告
- **测试**：通过标准的 `go test` 进行单元测试，通过构建标签 `//go:build integration` 进行集成测试

## 发布流程

发布流程通过 `.goreleaser.yml` 中的 [GoReleaser](https://goreleaser.com/) 配置实现自动化：

```bash
goreleaser release
```

在禁用 CGO 的情况下，为 linux、windows 和 darwin (amd64) 构建。
