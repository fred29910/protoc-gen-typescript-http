# 替换 ESLint 格式化为 Deno 格式化设计规范

## 1. 目标
将本项目生成代码的格式化工具从 ESLint 替换为 `deno fmt`，同时保留生成代码中已有的 ESLint 注释以兼容插件使用者的环境。

## 2. 背景
当前 `protoc-gen-typescript-http` 生成的 TypeScript 代码包含了如 `/* eslint-disable camelcase */` 等注释。虽然这些注释需要保留给下游使用者，但在开发插件本身时，对生成代码的格式化应该采用更现代化和快速的 `deno fmt`。

## 3. 具体修改

### 3.1 代码生成器更新
*   **保持原有注释**：`internal/plugin/packagegen.go` 和 `internal/plugin/servicegen.go` 中的现有 `eslint-disable` 注释将原封不动地保留。
*   **增加 Deno 兼容性**：在 `internal/plugin/packagegen.go` 的生成文件头中增加一行 `// deno-lint-ignore-file`，以使得使用 Deno 作为工具链的下游项目同样能够屏蔽这些生成文件的 lint 报错。

### 3.2 测试与生成流程集成
*   **集成测试更新**：修改 `tests/integration/integration_test.go`。在执行 `buf generate` 之后、执行 `git diff` 验证之前，添加调用 `deno fmt gen/typescript/` (实际目录视相对路径而定) 格式化生成的 TypeScript 代码。
*   **Makefile / Magefile 更新**：在 `generate` 步骤（或对应的生成脚本）中，在执行了 `buf generate` 之后，紧接着运行 `deno fmt examples/proto/gen/typescript`，确保开发人员在本地生成的代码也被正确格式化。

### 3.3 CI 环境配置
*   修改 `.github/workflows/test.yml` 文件。在执行集成测试的步骤前，增加使用 `denoland/setup-deno@v1`（或其他最新稳定版）的步骤，以便 CI 环境拥有运行 `deno fmt` 的能力。

### 3.4 遗留文件清理
*   删除项目根目录的 `.eslintrc.js` 文件（因为本项目不再使用 ESLint 检查生成的代码格式）。
*   检查 `package.json` 如果没有其他用处（当前仅依赖 `typescript`），可一并移除或仅精简。

## 4. 验证方式
*   `make generate` 后，`examples/proto/gen/typescript` 下的文件将被格式化为 Deno 标准格式。
*   运行 `make integration` 或 `mage integration` 应该通过测试，无 `git diff` 失败。
*   GitHub Actions CI 测试通过。