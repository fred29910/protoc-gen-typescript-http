<!-- superpowers-zh:begin (do not edit between these markers) -->
# Superpowers-ZH 中文增强版

本项目已安装 superpowers-zh 技能框架（20 个 skills）。

## 核心规则

1. **收到任务时，先检查是否有匹配的 skill** — 哪怕只有 1% 的可能性也要检查
2. **设计先于编码** — 收到功能需求时，先用 brainstorming skill 做需求分析
3. **测试先于实现** — 写代码前先写测试（TDD）
4. **验证先于完成** — 声称完成前必须运行验证命令

## 可用 Skills

Skills 位于 `.agents/skills/` 目录，每个 skill 有独立的 `SKILL.md` 文件。

- **brainstorming**: 在任何创造性工作之前必须使用此技能——创建功能、构建组件、添加功能或修改行为。在实现之前先探索用户意图、需求和设计。
- **chinese-code-review**: 中文 review 沟通参考——话术模板、分级标注（必须修复/建议修改/仅供参考）、国内团队常见反模式应对。仅在用户显式 /chinese-code-review 时调用，不要根据上下文自动触发。
- **chinese-commit-conventions**: 中文 commit 与 changelog 配置参考——Conventional Commits 中文适配、commitlint/husky/commitizen 中文模板、conventional-changelog 中文配置。仅在用户显式 /chinese-commit-conventions 时调用，不要根据上下文自动触发。
- **chinese-documentation**: 中文文档排版参考——中英文空格、全半角标点、术语保留、链接格式、中文文案排版指北约定。仅在用户显式 /chinese-documentation 时调用，不要根据上下文自动触发。
- **chinese-git-workflow**: 国内 Git 平台配置参考——Gitee、Coding.net、极狐 GitLab、CNB 的 SSH/HTTPS/凭据/CI 接入差异与镜像同步配置。仅在用户显式 /chinese-git-workflow 时调用，不要根据上下文自动触发。
- **dispatching-parallel-agents**: 当面对 2 个以上可以独立进行、无共享状态或顺序依赖的任务时使用
- **executing-plans**: 当你有一份书面实现计划需要在单独的会话中执行，并设有审查检查点时使用
- **finishing-a-development-branch**: 当实现完成、所有测试通过、需要决定如何集成工作时使用——通过提供合并、PR 或清理等结构化选项来引导开发工作的收尾
- **mcp-builder**: MCP 服务器构建方法论 — 系统化构建生产级 MCP 工具，让 AI 助手连接外部能力
- **receiving-code-review**: 收到代码审查反馈后、实施建议之前使用，尤其当反馈不明确或技术上有疑问时——需要技术严谨性和验证，而非敷衍附和或盲目执行
- **requesting-code-review**: 完成任务、实现重要功能或合并前使用，用于验证工作成果是否符合要求
- **subagent-driven-development**: 当在当前会话中执行包含独立任务的实现计划时使用
- **systematic-debugging**: 遇到任何 bug、测试失败或异常行为时使用，在提出修复方案之前执行
- **test-driven-development**: 在实现任何功能或修复 bug 时使用，在编写实现代码之前
- **using-git-worktrees**: 当需要开始与当前工作区隔离的功能开发，或在执行实现计划之前使用——通过原生工具或 git worktree 回退机制确保隔离工作区存在
- **using-superpowers**: 在开始任何对话时使用——确立如何查找和使用技能，要求在任何响应（包括澄清性问题）之前调用 Skill 工具
- **verification-before-completion**: 在宣称工作完成、已修复或测试通过之前使用，在提交或创建 PR 之前——必须运行验证命令并确认输出后才能声称成功；始终用证据支撑断言
- **workflow-runner**: 在 Claude Code / OpenClaw / Cursor 中直接运行 agency-orchestrator YAML 工作流——无需 API key，使用当前会话的 LLM 作为执行引擎。当用户提供 .yaml 工作流文件或要求多角色协作完成任务时触发。
- **writing-plans**: 当你有规格说明或需求用于多步骤任务时使用，在动手写代码之前
- **writing-skills**: 当创建新技能、编辑现有技能或在部署前验证技能是否有效时使用

## 如何使用

当任务匹配某个 skill 时，使用 `Skill` 工具加载对应 skill 并严格遵循其流程。绝不要用 Read 工具读取 SKILL.md 文件。

如果你认为哪怕只有 1% 的可能性某个 skill 适用于你正在做的事情，你必须调用该 skill 检查。
<!-- superpowers-zh:end -->


# 智能体协同与 Superpowers 工作流指南

本项目深度集成了 `superpowers-zh` 的标准操作程序（SOP），并完全依赖 OpenCode 内置的 5 个代理进行协作。请主代理（Primary Agent）严格遵守以下调度路由规则：

## 1. 架构与规划 (使用 Plan 模式)
- 当用户提出新需求或模糊的想法时，建议用户切换到 `plan` 代理（或你可以自主提议）。
- 此时应运用 **`brainstorming`** 技能明确需求边界，并使用 **`writing-plans`** 技能输出分步执行清单。

## 2. 子代理调度路由 (Subagent Routing Rules)
作为 `build` 代理，在执行开发任务时，你（父代理）负责推进主流程。当遇到以下专项任务时，**禁止自己大包大揽**，必须派发给内置子代理，派发格式要求明确指明使用的 **Skill**：

### 🟢 场景 A: 测试驱动开发 (TDD)
- **触发条件**：开始开发新功能模块前，需要编写测试。
- **调用指令**：`@general 请接管当前任务，严格使用 test-driven-development (TDD) 技能为模块 [X] 编写并运行测试，直到测试用例本身无误。`

### 🔴 场景 B: 解决复杂 Bug 或报错
- **触发条件**：测试运行失败，或代码出现逻辑/运行时错误，尝试 1 次仍未解决。
- **调用指令**：`@general 遇到复杂报错，请接管此问题，严格使用 systematic-debugging 技能（定位->分析->假设->修复）进行排查并修复。`

### 🔵 场景 C: 代码审查 (Code Review)
- **触发条件**：一个功能模块开发完毕，准备合并或结束任务。
- **调用指令**：`@explore 请审查我刚刚修改的文件。严格调用 requesting-code-review 和 chinese-code-review 技能，检查是否存在边界条件遗漏或性能隐患。`

### 🟡 场景 D: 外部资料与文档检索
- **触发条件**：遇到不熟悉的第三方库、API 弃用或缺乏当前上下文。
- **调用指令**：`@scout 请通过网络搜索或查阅文档，帮我总结 [某技术/某报错] 的最新上下文。`

## 3. 协作原则
- **保持上下文干净**：子代理在独立会话中执行完毕后，主代理只需获取其结论/报告，然后继续推进主线任务。
- **中文习惯优先**：在编写注释、文档以及 Code Review 时，默认遵循中文开发团队的阅读习惯。