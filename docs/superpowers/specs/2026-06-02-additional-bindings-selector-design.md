# Additional Bindings 生成支持（first-match-wins 语义）设计文档

**日期**: 2026-06-02  
**项目**: protoc-gen-typescript-http  
**状态**: 已批准，待实现  
**关联**: 评审 `2026-06-02-golang-tool-project-review.md` P0 #1

---

## 1. 问题描述

`internal/httprule/rule.go` 已递归解析 `HttpRule.AdditionalBindings` 并存入 `Rule.AdditionalRules`，但 `internal/plugin/servicegen.go` 的 `generateMethod` 仅消费主 `rule`，对 `AdditionalRules` 视而不见。

结果：带 `additional_bindings` 的 RPC 在生成出的 client 里只有主路由可调用，所有替代绑定静默丢失。文档（`docs/code-generation.md`、`docs/protobuf-annotations.md`）已声明该限制，但 README 功能描述仍容易让用户预期完整 HTTP transcoding。

```
.proto
  rpc GetThing(GetThingRequest) returns (Thing) {
    option (google.api.http) = {
      get: "/v1/{parent}/things/{thing}"
      additional_bindings { get: "/v1/things/{thing}" }
    };
  }
```

当前生成（错误）：

```ts
async getThing(request) {
  // 只生成主 binding /v1/{parent}/things/{thing}
  // /v1/things/{thing} 这个 additional_bindings 永远不会使用
}
```

---

## 2. 根因分析

| 层面 | 实际情况 |
|---|---|
| 解析层 | `httprule.ParseRule` 递归解析 `AdditionalBindings`，`Rule.AdditionalRules` 字段已填充 |
| 生成层 | `servicegen.generateMethod` 调用 `httprule.ParseRule` 后只使用返回的 `rule`（主），没有遍历 `rule.AdditionalRules` |
| 文档 | 限制已写明但 README 未对齐，且无任何生成测试覆盖 `additional_bindings` 路径 |

---

## 3. 产品决策

经评审讨论，选择 **C. 请求级 binding selector** + **first-match-wins** 语义：

- 生成的 client 方法对 `[mainRule, ...additionalRules]` 按定义顺序尝试
- 第一个所有 path 变量都满足的 binding 被选中
- 不支持 binding 的方法保持现状（仅主 binding），不抛错
- 无任何 binding 匹配时，抛 `Error("no matching binding for <Method>")`

理由：与 Google API Gateway/Transcoding 默认行为一致；调用方零代码改动（`request` 字段决定 binding）；不需要引入新的 `meta`/`body` 选择器。

---

## 4. 设计

### 4.1 生成器改造

`servicegen.generateMethod`（`internal/plugin/servicegen.go:82`）改为：

1. 收集 `rules := []httprule.Rule{rule}` 加上 `rule.AdditionalRules`
2. 对每个 `r` in `rules` 独立生成 path / body / query 一套，包装在 `if (pathVarsPresent) { ... return handler(...) }` 分支中
3. 所有分支均不匹配时，落到末尾的 `throw new Error(...)`
4. 0 个 additional binding 时，`rules` 长度为 1，行为与现状等价（回归保护）

### 4.2 每个分支的内部结构

每个 binding 分支内部独立生成：

- **path 变量 presence 校验**（取自 `nullPropagationPath`）
- **path 字符串构造**（取自 `generateMethodPath`）
- **body 构造**（取自 `generateMethodBody`，binding 自带的 `Body` 字段）
- **query 构造**（取自 `generateMethodQuery`，但 `pathCovered` 与 body 排除集是当前 binding 特有的）
- **`return handler({...}, {...})`**

### 4.3 关键约束

- **不抽公共变量**：每个 binding 的 `path`、`body`、`queryParams` 都是 binding-specific，抽公共变量反而让分支代码难以独立阅读
- **不抽公共 `return handler(...)`**：每个分支独立 `return handler`，避免跨分支共享变量
- **path 变量判定严格遵循现有 presence 规则**：使用 `nullPropagationPath` + `!== undefined && !== null`，与 path 校验和 query presence 保持一致

### 4.4 错误处理

调用方未提供任何 binding 的必要 path 字段时，抛错：

```ts
throw new Error("no matching binding for getThing");
```

错误信息含方法名，便于定位。生成阶段不再因 `additional_bindings` 解析失败而报错（解析层已有覆盖）。

---

## 5. 改动文件清单

| 文件 | 改动 |
|---|---|
| `internal/plugin/servicegen.go` | `generateMethod` 重构为多 binding 分支；每个分支独立 path/body/query；末尾统一抛错 |
| `internal/plugin/servicegen_test.go`（或新文件）| 新增 binding 选择测试：first-match-wins / 都满足走主 / 都不满足抛错 / 无 additional 回归 |
| `examples/proto/` | 增加一个使用 `additional_bindings` 的示例 service，验证 golden 输出 |
| `docs/code-generation.md` | "additional_bindings" 行从"仅生成主绑定"改为说明 first-match-wins 行为 |
| `docs/protobuf-annotations.md` | "当前行为" 段落同步更新 |
| `README.md` | 如功能列表提及 transcoding，对齐 |

---

## 6. 测试矩阵

| # | 用例 | 期望 |
|---|---|---|
| 1 | 主 binding path 变量全有值 | 走主分支 |
| 2 | 主 binding 缺变量、附加 binding 满足 | 走附加分支 |
| 3 | 多个 binding 都满足 | 走第一个（按 proto 定义顺序） |
| 4 | 所有 binding 都不满足 | `throw new Error("no matching binding for <Method>")` |
| 5 | 无 `additional_bindings`（length=1）| 走单分支（回归测试，输出与现状一致）|
| 6 | 附加 binding 有独立 `body` 字段 | 各分支 body 独立 |
| 7 | 附加 binding path 变量不同 | query 排除集独立 |
| 8 | 附加 binding 含 wildcard sub-template | wildcard 编码正确 |
| 9 | 附加 binding 含 custom verb | verb 正确应用 |
| 10 | 集成测试 `buf generate` golden diff | 通过 |

---

## 7. 风险

| 风险 | 缓解 |
|---|---|
| 生成代码体积随 `additional_bindings` 数量线性增长 | 接受。N 较小（通常 1-3）|
| 分支结构改变行号，影响调试体验 | 在生成代码前加注释 `// binding 1/3: <pattern>` 帮助定位 |
| `pathCovered` 计算与现有 `generateMethodQuery` 耦合 | 需确认现有函数对 `Rule` 整体已 binding-aware；如不是，把 `rule` 参数从主 binding 改为当前 binding |
| `nullPropagationPath` 在嵌套 message 上的行为需在每个 binding 上独立验证 | 测试 #1-#4 覆盖 |
| 集成测试 golden diff 会变更（添加示例后）| 评审已要求示例 → golden 必须更新，提交时一并提交 |

---

## 8. 非目标（YAGNI）

- 不实现 `meta.bindingIndex` 显式选择器
- 不实现按"最具体 binding 优先"自动选择
- 不改 path encoding 策略、不改 body encoding 策略
- 不动 httprule 解析层
- 不支持 cross-package path validation（仅在 input 同一包时校验）

---

## 9. 实施步骤

1. 重构 `generateMethod` 为 `generateMethodMultiBinding`（内部按 binding 数循环）
2. 每个 binding 分支独立调用现有 `generateMethodPath`/`generateMethodBody`/`generateMethodQuery`/`generateMethodPathValidation`，传当前 binding 的 `rule` 而非最外层 `rule`
3. 分支结构生成（含 if/else/throw）
4. 单元测试先行（cases 1-9）
5. 集成测试：新增 `examples/proto/.../additional_bindings.proto`，运行 `buf generate` + `deno fmt` + `git diff` 验证
6. 文档同步

---

## 10. 验收标准

- [ ] `go test -count=1 ./...` 全部通过，新增 binding 选择测试覆盖 §6
- [ ] `go vet ./...` 通过
- [ ] `gofmt -l` 无输出
- [ ] 集成测试 `go test -tags integration` 通过
- [ ] `buf generate` + `deno fmt` 后 `git diff examples/proto/gen` 干净
- [ ] 文档中所有 `additional_bindings` 描述一致指向 first-match-wins
- [ ] 评审 P0 #1 关闭
