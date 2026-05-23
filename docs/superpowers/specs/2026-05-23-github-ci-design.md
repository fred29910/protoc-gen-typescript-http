# GitHub CI 设计文档

## 概述

为 `protoc-gen-typescript-http` 项目创建 3 个 GitHub Actions workflow，参考 `migra-go` 的模式，适配当前项目特点。

当前项目是纯 Go 项目（Go 1.25.7），无 C 依赖，使用 goreleaser 进行多平台发布。

## Workflow 列表

| 文件 | 触发条件 | 职责 |
|---|---|---|
| `lint.yml` | push to main, PR to main | 代码格式检查 + 静态分析 |
| `test.yml` | push to main, PR to main | 单元测试 + 集成测试 |
| `release.yml` | push tag `v*` | 多平台构建 + GitHub Release |

---

## 1. Lint Workflow (`.github/workflows/lint.yml`)

### 触发条件
- push 到 `main` 分支
- 针对 `main` 分支的 pull request

### Job: `lint`

**运行环境**：`ubuntu-latest`

**步骤**：
1. `actions/checkout@v4` — 检出代码
2. `actions/setup-go@v5` — 设置 Go 环境（版本从 `go.mod` 读取，启用缓存）
3. **gofmt 检查** — 运行 `gofmt -l .`，如果有未格式化的文件则失败
4. **go vet** — 运行 `go vet ./...`
5. **golangci-lint** — 使用 `golangci/golangci-lint-action@v7`，`version: latest`，`args: --timeout=5m`
6. **buf lint** — 安装 `buf`（版本 `v1.69.0`），运行 `cd examples/proto && buf lint`

---

## 2. Test Workflow (`.github/workflows/test.yml`)

### 触发条件
- push 到 `main` 分支
- 针对 `main` 分支的 pull request

### Job: `test`

**运行环境**：`ubuntu-latest`

**步骤**：
1. `actions/checkout@v4` — 检出代码
2. `actions/setup-go@v5` — 设置 Go 环境（启用缓存）
3. **安装 buf** — `go install github.com/bufbuild/buf/cmd/buf@v1.69.0`
4. **安装 mage** — `go install github.com/magefile/mage@v1.17.2`
5. **单元测试** — `mage test`（运行 `go test -v ./...`）
6. **集成测试** — `mage integration`（先 build 插件，再运行 `buf generate`，用 `git diff` 验证生成的代码与提交一致）

---

## 3. Release Workflow (`.github/workflows/release.yml`)

### 触发条件
- push 标签 `v*`（如 `v1.0.0`）

### Job: `release`

**运行环境**：`ubuntu-latest`

**权限**：`contents: write`

**步骤**：
1. `actions/checkout@v4` — 检出代码（`fetch-depth: 0`，goreleaser 需要完整 git 历史）
2. `actions/setup-go@v5` — 设置 Go 环境（启用缓存）
3. `goreleaser/goreleaser-action@v6` — 安装并运行 goreleaser
   - `version: latest`
   - `args: release --clean`
4. 认证方式：`secrets.RELEASE_TOKEN`（自定义 PAT，需在仓库 Settings > Secrets 中配置）

### 说明
- 当前项目是纯 Go 项目，无 C 依赖，不需要 `goreleaser-cross` 容器
- 使用单 job 构建，goreleaser 根据 `.goreleaser.yml` 配置一次性构建 linux/darwin/windows 所有平台
- goreleaser 自动创建 GitHub Release 并上传产物

---

## 与 migra-go 的关键差异

| 项目 | migra-go | protoc-gen-typescript-http |
|---|---|---|
| C 依赖 | 有（pg_query_go） | 无 |
| 交叉编译 | 需要 goreleaser-cross 容器 + 手动安装交叉编译工具链 | 不需要，Go 原生支持 |
| Release job 结构 | 3 个构建 job + 1 个汇总 job | 单 job |
| git safe.directory | 需要修复容器内 git 所有权 | 不需要 |
| Windows 构建 | 需要 mingw-w64 | goreleaser 自动处理 |

---

## 文件结构

```
.github/
└── workflows/
    ├── lint.yml
    ├── test.yml
    └── release.yml
```

## 前置条件

1. 仓库需配置 secret `RELEASE_TOKEN`（具有 `contents: write` 权限的 PAT）
2. 确保 `.goreleaser.yml` 已正确配置（已存在，无需修改）
