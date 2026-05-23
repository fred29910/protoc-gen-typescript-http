# ============================================================================
# protoc-gen-typescript-http — Makefile
# 
# This Makefile is a thin orchestrator that delegates Go-specific tasks to
# magefile.go via the `mage` build tool. Run `mage -l` to see all available
# mage targets.
# ============================================================================

# === Default ===

.PHONY: all
all: build test

# === Tool Versions ===

BUF_VERSION   ?= v1.69.0
MAGE_VERSION  ?= v1.17.2

TOOLS_DIR := $(abspath .tools)
BUF_DIR   := $(TOOLS_DIR)/buf/$(BUF_VERSION)
MAGE_DIR  := $(TOOLS_DIR)/mage/$(MAGE_VERSION)
TOOL_PATH := $(BUF_DIR):$(MAGE_DIR)

# === Tool Installation ===

.PHONY: install-buf
install-buf: $(BUF_DIR)/buf

$(BUF_DIR)/buf:
	@echo "Installing buf $(BUF_VERSION)..."
	@mkdir -p $(BUF_DIR) && \
	GOBIN=$(BUF_DIR) go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)

.PHONY: install-mage
install-mage: $(MAGE_DIR)/mage

$(MAGE_DIR)/mage:
	@echo "Installing mage $(MAGE_VERSION)..."
	@mkdir -p $(MAGE_DIR) && \
	GOBIN=$(MAGE_DIR) go install github.com/magefile/mage@$(MAGE_VERSION)

# === Go Tasks (delegated to magefile.go) ===
# The following targets delegate to their mage equivalents.
# All require mage to be installed.

.PHONY: build
build: install-mage
	PATH=$(TOOL_PATH):$$PATH mage build

.PHONY: test
test: install-mage
	PATH=$(TOOL_PATH):$$PATH mage test

.PHONY: test-unit
test-unit: test   # Alias for make test — runs only unit tests

.PHONY: vet
vet: install-mage
	PATH=$(TOOL_PATH):$$PATH mage vet

.PHONY: fmt
fmt: install-mage
	PATH=$(TOOL_PATH):$$PATH mage fmt

# === Integration Tests ===
# Delegates to mage Integration(), which:
#   1. Builds the plugin (mg.Deps(Build))
#   2. Adds bin/ to PATH so the plugin binary is discoverable
#   3. Runs go test -tags=integration ./tests/integration/...
# The integration test calls `buf generate` in examples/proto/
# and verifies generated output matches committed code (git diff --exit-code).

.PHONY: integration
integration: install-buf install-mage
	PATH=$(TOOL_PATH):$$PATH mage integration

# === Proto Lint / Generate ===

.PHONY: lint
lint: install-buf
	cd examples/proto && PATH=$(BUF_DIR):$$PATH buf lint

.PHONY: generate
generate: build
	cd examples/proto && PATH=$(abspath bin):$(BUF_DIR):$$PATH buf generate

# === CI Pipeline ===
# Runs the full check suite: vet → build → test → integration

.PHONY: ci
ci: vet build test integration
	@echo "✓ All checks passed"

# === Clean ===

.PHONY: clean
clean:
	rm -rf $(TOOLS_DIR)
	rm -rf bin
	rm -rf examples/proto/gen/typescript
