# Makefile 工具安装 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 在根 Makefile 中新增 buf、mage 等开发工具的自动安装目标，固定版本号，实现可复现的开发环境搭建。

**架构：** 工具二进制安装到项目本地 `.tools/<tool>/<version>/` 目录，Makefile 自动检测并管理 PATH。版本变量集中配置在 Makefile 顶部。

**技术栈：** GNU Make, Go (go install), curl, tar

**设计文档：** `docs/superpowers/specs/2026-05-22-makefile-tools-installation-design.md`

---

## 文件结构

| 文件 | 职责 | 变更类型 |
|------|------|----------|
| `.gitignore` | 排除 `.tools/` 目录 | 追加一行 |
| `go.mod` | Go 模块依赖，升级 mage 版本 | 修改版本号 |
| `Makefile` | 工具安装、build/test/integration/lint/generate/clean 入口 | 重写 |

---

### 任务 1：更新 `.gitignore` 添加 `.tools/` 排除

**文件：**
- 修改：`.gitignore`

- [ ] **步骤 1：在 .gitignore 末尾追加 `.tools/`**

在 `/opt/codes/workspace/protoc-gen-typescript-http/.gitignore` 末尾添加一行：

```
.tools/
```

确保保留末行空行。

- [ ] **步骤 2：验证**

```bash
cat .gitignore | grep '\.tools'
```

预期输出包含 `.tools/`。

- [ ] **步骤 3：Commit**

```bash
git add .gitignore
git commit -m "chore: add .tools/ to gitignore"
```

---

### 任务 2：升级 go.mod 中 mage 版本

**文件：**
- 修改：`go.mod`

- [ ] **步骤 1：修改 go.mod**

将 `github.com/magefile/mage v1.17.1` 改为 `github.com/magefile/mage v1.17.2`。

```go
require (
	github.com/magefile/mage v1.17.2
	// ... 其他依赖保持不变
)
```

- [ ] **步骤 2：运行 go mod tidy 验证**

```bash
go mod tidy
```

预期：无错误，go.sum 更新。

- [ ] **步骤 3：验证版本**

```bash
grep magefile go.mod
```

预期输出：`github.com/magefile/mage v1.17.2`

- [ ] **步骤 4：Commit**

```bash
git add go.mod go.sum
git commit -m "chore: bump mage from v1.17.1 to v1.17.2"
```

---

### 任务 3：重写根 Makefile

**文件：**
- 修改：`Makefile`

将 `/opt/codes/workspace/protoc-gen-typescript-http/Makefile` 整体替换为以下内容。

- [ ] **步骤 1：写入全新 Makefile**

```makefile
.PHONY: all
all: build test

# === 工具版本 ===
BUF_VERSION ?= v1.69.0
MAGE_VERSION ?= v1.17.2

TOOLS_DIR := $(abspath .tools)
BUF_DIR   := $(TOOLS_DIR)/buf/$(BUF_VERSION)
MAGE_DIR  := $(TOOLS_DIR)/mage/$(MAGE_VERSION)
TOOL_PATH := $(BUF_DIR):$(MAGE_DIR)

# === 工具安装 ===

.PHONY: install-buf
install-buf: $(BUF_DIR)/buf

$(BUF_DIR)/buf:
	@echo "Installing buf $(BUF_VERSION)..."
	@mkdir -p $(BUF_DIR) && \
	UNAME_OS=$$(uname -s | tr '[:upper:]' '[:lower:]') && \
	UNAME_ARCH=$$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') && \
	curl -fsSL "https://github.com/bufbuild/buf/releases/download/$(BUF_VERSION)/buf-$${UNAME_OS}-$${UNAME_ARCH}.tar.gz" | \
	  tar -xz -C $(BUF_DIR) --strip-components=1 buf && \
	chmod +x $(BUF_DIR)/buf

.PHONY: install-mage
install-mage: $(MAGE_DIR)/mage

$(MAGE_DIR)/mage:
	@echo "Installing mage $(MAGE_VERSION)..."
	@mkdir -p $(MAGE_DIR) && \
	GOBIN=$(MAGE_DIR) go install github.com/magefile/mage@$(MAGE_VERSION)

# === Build / Test ===

.PHONY: build
build: install-mage
	PATH=$(TOOL_PATH):$$PATH mage build

.PHONY: test
test: install-mage
	PATH=$(TOOL_PATH):$$PATH mage test

.PHONY: integration
integration: install-buf install-mage
	PATH=$(TOOL_PATH):$$PATH mage integration

# === Lint / Generate (examples/proto) ===

.PHONY: lint
lint: install-buf
	cd examples/proto && PATH=$(BUF_DIR):$$PATH buf lint

.PHONY: generate
generate: install-buf
	cd examples/proto && PATH=$(BUF_DIR):$$PATH buf generate

# === Clean ===

.PHONY: clean
clean:
	rm -rf $(TOOLS_DIR)
	rm -rf bin
	rm -rf examples/proto/gen/typescript
```

**关键设计说明：**
- `BUF_VERSION`/`MAGE_VERSION` 使用 `?=` 允许环境变量覆盖（CI 场景可传入不同版本）
- buf 安装使用 shell 变量 `UNAME_OS`/`UNAME_ARCH` 实现跨平台检测，`&&` 链式保证原子性
- mage 安装通过 `GOBIN` 重定向到 `.tools/mage/<version>/`
- `install-buf` 和 `install-mage` 使用文件依赖模式——`.tools/buf/v1.69.0/buf` 存在即跳过
- `build`/`test` 只需 mage，`integration` 需要 mage + buf
- `cd examples/proto && buf ...` 中的 buf 通过 `PATH=$(BUF_DIR):$$PATH` 注入

- [ ] **步骤 2：验证 Makefile 语法**

```bash
make -n build
make -n test
make -n integration
make -n lint
make -n generate
make -n clean
```

预期：所有命令输出正确的展开命令，无语法错误。

- [ ] **步骤 3：Commit**

```bash
git add Makefile
git commit -m "feat: add tool installation targets (buf v1.69.0, mage v1.17.2)"
```

---

### 任务 4：端到端验证

**文件：**
- 验证：整个工具链

- [ ] **步骤 1：安装 tools 并执行 build**

```bash
make clean
make build
```

预期：
- 首次运行自动下载 buf v1.69.0 和安装 mage v1.17.2
- 编译成功，`bin/protoc-gen-typescript-http` 生成
- 第二次运行跳过安装（文件已存在）

- [ ] **步骤 2：运行测试**

```bash
make test
```

预期：单元测试通过。

- [ ] **步骤 3：运行 integration 测试**

```bash
make integration
```

预期：buf generate 成功，`examples/proto/gen/typescript/` 生成代码与 git 版本一致。

- [ ] **步骤 4：验证 lint**

```bash
make lint
```

预期：buf lint 通过，无错误。

- [ ] **步骤 5：验证 generate**

```bash
make generate
```

预期：buf generate 成功，生成代码无差异。

- [ ] **步骤 6：验证 clean**

```bash
make clean
ls .tools 2>&1 || echo ".tools removed (expected)"
ls bin 2>&1 || echo "bin removed (expected)"
```

预期：`.tools/` 和 `bin/` 被删除。

- [ ] **步骤 7：验证旧路径仍然可用**

```bash
go run github.com/magefile/mage build
```

预期：`go run` 方式仍然工作正常。

- [ ] **步骤 8：最终 Commit（如果有额外修复）**

```bash
git add -A
git commit -m "test: verify tool installation and build pipeline"
```
