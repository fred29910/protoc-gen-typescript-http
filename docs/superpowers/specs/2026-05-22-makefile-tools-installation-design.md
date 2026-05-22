# Makefile 工具安装设计文档

**日期**: 2026-05-22
**项目**: protoc-gen-typescript-http
**状态**: 设计稿

## 1. 目标

在项目根 Makefile 中补充和完善 `buf`、`mage` 等开发工具的安装逻辑，固定版本号，实现可复现的开发环境搭建。

## 2. 工具版本

| 工具 | 版本 | 安装方式 | 备注 |
|------|------|----------|------|
| buf | v1.69.0 | GitHub Releases 下载二进制 | 预编译，无 Go 依赖 |
| mage | v1.17.2 | `go install` + GOBIN 重定向 | Go 模块依赖，需更新 go.mod |

> 当前 go.mod 中 mage 为 v1.17.1，同步升级至 v1.17.2。

## 3. 目录结构

```
.tools/
├── buf/
│   └── v1.69.0/
│       └── buf                    # 从 GitHub Releases 下载的预编译二进制
└── mage/
    └── v1.17.2/
        └── mage                   # go install 产物 (GOBIN 重定向)
.tools/ 已加入 .gitignore
```

每个工具独立子目录 + 版本子目录，支持多版本共存，清理时直接删除整个 `.tools/`。

## 4. Makefile 设计

### 4.1 版本变量（顶部集中管理）

```makefile
BUF_VERSION ?= v1.69.0
MAGE_VERSION ?= v1.17.2

TOOLS_DIR := $(abspath .tools)
BUF_DIR   := $(TOOLS_DIR)/buf/$(BUF_VERSION)
MAGE_DIR  := $(TOOLS_DIR)/mage/$(MAGE_VERSION)
TOOL_PATH := $(BUF_DIR):$(MAGE_DIR)
```

- 使用 `?=` 允许环境变量覆盖版本号（CI 场景）
- `TOOL_PATH` 组合所有工具的 bin 目录，供 PATH 注入使用

### 4.2 buf 安装

```makefile
.PHONY: install-buf
install-buf: $(BUF_DIR)/buf

$(BUF_DIR)/buf:
	@echo "Installing buf $(BUF_VERSION)..."
	@mkdir -p $(BUF_DIR)
	$(eval UNAME_OS := $(shell uname -s | tr '[:upper:]' '[:lower:]'))
	$(eval UNAME_ARCH := $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/'))
	curl -fsSL https://github.com/bufbuild/buf/releases/download/$(BUF_VERSION)/buf-$(UNAME_OS)-$(UNAME_ARCH).tar.gz \
	  | tar -xz -C $(BUF_DIR) --strip-components=1 buf
	@chmod +x $(BUF_DIR)/buf
```

- **Makefile 文件依赖模式**: 使用 `$(BUF_DIR)/buf` 作为 target，如果文件已存在则跳过。
- **跨平台**: 根据 `uname` 动态检测 OS 和架构，支持 Linux/macOS、amd64/arm64。
- **无锁问题**: `mkdir -p` 是幂等的，curl 管道 tar 是原子写入——多进程同时构建也不会冲突。

### 4.3 mage 安装

```makefile
.PHONY: install-mage
install-mage: $(MAGE_DIR)/mage

$(MAGE_DIR)/mage:
	@echo "Installing mage $(MAGE_VERSION)..."
	@mkdir -p $(MAGE_DIR)
	GOBIN=$(MAGE_DIR) go install github.com/magefile/mage@$(MAGE_VERSION)
```

- `GOBIN` 环境变量将 `go install` 产物重定向到 `.tools/mage/<version>/`，不影响全局 GOPATH/bin。
- 同样使用文件依赖模式避免重复安装。

### 4.4 Build/Test/Integration 集成

```makefile
.PHONY: build
build: install-buf install-mage
	PATH=$(TOOL_PATH):$$PATH mage build

.PHONY: test
test: install-buf install-mage
	PATH=$(TOOL_PATH):$$PATH mage test

.PHONY: integration
integration: install-buf install-mage
	PATH=$(TOOL_PATH):$$PATH mage integration
```

- 每个 target 自动依赖对应工具安装，无需手动 `make install-tools`。
- `PATH=$(TOOL_PATH):$$PATH` 确保 `.tools/` 中的二进制优先被找到。
- `mage integration` 中已包含 `PATH=bin/:$PATH`，二者互补。

### 4.5 Lint/Generate（examples/proto）

```makefile
.PHONY: lint
lint: install-buf
	cd examples/proto && PATH=$(BUF_DIR):$$PATH buf lint

.PHONY: generate
generate: install-buf
	cd examples/proto && PATH=$(BUF_DIR):$$PATH buf generate
```

- 将 examples/proto/Makefile 中的常用目标提升到根 Makefile，方便一键操作。

### 4.6 Clean

```makefile
.PHONY: clean
clean:
	rm -rf $(TOOLS_DIR)
	rm -rf bin
	rm -rf examples/proto/gen/typescript
```

- 清理时同时删除 `.tools/`、构建产物和示例生成代码。

## 5. 涉及的文件变更

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `Makefile` | 重写 | 新增完整的 install/build/test/lint/generate/clean 体系 |
| `.gitignore` | 追加 | 添加 `.tools/` |
| `go.mod` | 更新 | mage v1.17.1 → v1.17.2 |
| `magefile.go` | 无变更 | Makefile 已接管 PATH 管理，magefile 原有逻辑不变 |
| `examples/proto/Makefile` | 暂不修改 | 保持独立兼容性；也可考虑后续简化 |

## 6. 使用方式

```bash
# 首次使用（自动安装工具）
make build
make test
make integration

# 仅安装工具
make install-buf
make install-mage

# 清理
make clean
```

## 7. 不受影响的使用路径

- `go run github.com/magefile/mage` 仍然可用（不需要 mage 二进制）
- `mage build`/`mage test` 等方式仍然可用（需要 mage 和 buf 在系统 PATH 上）
- examples/proto/ 下的 Makefile 保持不变

## 8. 规格自检

- [x] 占位符扫描: 无 TODO/待定/模糊内容
- [x] 内部一致性: 版本变量在 Makefile 顶部分配，各 target 引用一致
- [x] 范围检查: 聚焦于 Makefile 工具安装，不涉及 magefile 逻辑变更
- [x] 模糊性检查: 所有版本号显式锁定，平台检测逻辑明确
