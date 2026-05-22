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
