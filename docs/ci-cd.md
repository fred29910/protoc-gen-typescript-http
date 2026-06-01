# CI/CD

本文档描述项目的持续集成与发布流程。所有 CI/CD 任务由 [GitHub Actions](https://github.com/features/actions) 编排，配置文件位于 [`.github/workflows/`](../.github/workflows/)。

## 工作流总览

| 工作流文件 | 名称 | 触发条件 | 主要职责 |
|---|---|---|---|
| [`lint.yml`](../.github/workflows/lint.yml) | Lint | push/PR 到 `main` | 静态检查：`gofmt`、`go vet`、`golangci-lint`、`buf lint` |
| [`test.yml`](../.github/workflows/test.yml) | Test | push/PR 到 `main` | 单元测试 + 集成测试（含 `deno fmt` 校验生成产物） |
| [`release.yml`](../.github/workflows/release.yml) | Release | 推送 `v*` tag | 通过 [GoReleaser](https://goreleaser.com) 跨平台构建并发布 release |

## Lint 工作流

**职责**：在合并前确保代码风格与静态检查通过，不做编译或测试。

**执行步骤**（按顺序）：

1. `actions/checkout@v6` — 拉取代码
2. `actions/setup-go@v6` — 读取 `go.mod` 安装 Go
3. `gofmt -l .` — 列出未格式化文件，存在则失败
4. `go vet ./...` — 静态分析
5. `golangci/golangci-lint-action@v9`（latest）— 运行 `golangci-lint`（5 分钟超时）
6. `bufbuild/buf-setup-action@v1`（`1.69.0`）— 安装 buf
7. `cd examples/proto && buf lint` — 对示例 proto 做 lint

## Test 工作流

**职责**：执行单元测试与集成测试，校验生成产物与已提交代码一致。

**执行步骤**（按顺序）：

1. `actions/checkout@v6`
2. `actions/setup-go@v6`（读取 `go.mod`）
3. `denoland/setup-deno@v2`（`v1.x`）— 集成测试的 `deno fmt` 步骤需要
4. `bufbuild/buf-setup-action@v1`（`1.69.0`）
5. `go install github.com/magefile/mage@v1.17.2` — 安装固定版本的 mage
6. `mage test` — 运行单元测试
7. `mage integration` — 运行集成测试（构建 → `buf generate` → `deno fmt` → `git diff` 校验）

> **注意**：`mage integration` 会构建插件二进制到 `bin/`，把 `bin/` 加入 `PATH`，再调用 `buf generate`；集成测试本身通过 `git diff --exit-code` 校验 `examples/proto/gen/typescript/` 没有未提交变更。

## Release 工作流

**职责**：在推送版本 tag（`v*`）时自动构建多平台二进制并发布 GitHub Release。

**执行步骤**（按顺序）：

1. `actions/checkout@v6`（`fetch-depth: 0`，GoReleaser 需要完整 git 历史计算变更日志）
2. `actions/setup-go@v6`（读取 `go.mod`）
3. `goreleaser/goreleaser-action@v7.2.2`（latest）— 调用 `goreleaser release --clean`

**权限**：授予 `contents: write`，以便创建 release 与上传制品。

**环境变量**：需要 `secrets.RELEASE_TOKEN`（PAT，权限需 `repo`）作为 `GITHUB_TOKEN` 使用——这是因为 GoReleaser 默认 token 无权创建跨仓库的 release。

**触发方式**：

```bash
git tag v0.1.0
git push origin v0.1.0
```

**构建产物**（由 [`.goreleaser.yml`](../.goreleaser.yml) v2 格式配置）：

- **平台**：`linux`、`windows`、`darwin`
- **CGO**：禁用（`CGO_ENABLED=0`），便于静态分发
- **二进制名**：`protoc-gen-typescript-http`
- **归档命名**：`protoc-gen-typescript-http_<OS>_<ARCH>`（`amd64` 归一为 `x86_64`）
- **校验和**：`checksums.txt`
- **预发布**：自动检测（提交信息含 `BREAKING` 或 `!` 时标记为 prerelease）

## 与本地工具链的关系

CI 工作流刻意使用与本地一致的固定版本，以减少"我本地能跑"问题：

| 工具 | CI 版本 | 本地管理 |
|---|---|---|
| Go | 跟随 `go.mod`（`go-version-file`） | 用户安装 |
| buf | `1.69.0`（`bufsetup-action`） | 通过 `make install-buf` 安装到 `.tools/buf/1.69.0/` |
| mage | `1.17.2`（`go install`） | 通过 `make install-mage` 安装到 `.tools/mage/1.17.2/` |
| Deno | `v1.x`（`setup-deno`） | 用户安装 |
| golangci-lint | `latest`（action） | 用户安装 |

`.tools/` 目录被 `.gitignore` 排除，但工具版本由 `Makefile` 中的 `BUF_VERSION` / `MAGE_VERSION` 变量管理。

## 本地复现 CI

推荐在提交前按 `make ci` 顺序本地跑一次：

```bash
make ci
# 等价于：
make vet
make build
make test
make integration
```

详见 [development.md § CI/CD](./development.md#cicd)。
