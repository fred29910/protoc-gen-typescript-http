# 计划：集成 Mage 管理集成测试

## 目标
1. 在项目中集成 `mage`。
2. 将集成测试（目前的插件生成流程）从 `Makefile` 迁移到 `magefile.go`。
3. 实现集成测试与程序的彻底分离，通过构建标签或独立任务管理。

## 阶段 1：准备工作 (已完成)
- [x] 调研 `mage` 的安装和基本使用。
- [x] 确定集成测试的具体范围（目前主要是 `examples/proto` 的生成）。
- [x] 检查是否需要将生成后的验证逻辑也纳入集成测试。

## 阶段 2：集成 Mage (已完成)
- [x] 在项目根目录创建 `magefile.go`。
- [x] 定义基础任务：`Build`（构建插件）、`Test`（运行单元测试）。
- [x] 添加 `mage` 到 `go.mod`。

## 阶段 3：迁移集成测试 (已完成)
- [x] 在 `magefile.go` 中定义 `Integration` 任务。
- [x] 在 `tests/integration/` 创建带有 `//go:build integration` 标签的 Go 测试文件。
- [x] 迁移验证逻辑（通过 `git diff` 验证生成的代码）。

## 阶段 4：清理与优化 (已完成)
- [x] 更新根目录 `Makefile`，使其调用 `mage`。
- [x] 编写文档说明如何运行测试。
- [x] 验证 CI 流程是否正常。

## 验收标准
- [x] 运行 `mage test` 仅执行单元测试。
- [x] 运行 `mage integration` 执行完整的构建和生成验证流程。
- [x] 集成测试逻辑不再散落在 `Makefile` 中，而是由 Go 代码管理。
