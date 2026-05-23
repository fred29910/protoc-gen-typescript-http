# Query 参数 Presence 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 修复 `servicegen.go` 中 query 参数的 truthy 判断（`if (request.field)`），改为 nullish 判断（`if (request.field !== undefined && request.field !== null)`），使 `0`、`false`、`""` 能正确生成 query 参数。

**架构：** 在 `internal/plugin/servicegen.go` 中新增两个辅助函数 `queryPresenceExpr` 和 `queryValueExpr`，统一生成 TS 条件表达式和值访问表达式。只修改 query 生成逻辑所在函数 `generateMethodQuery` 中的 if 条件输出行。

**技术栈：** Go 1.21+, protobuf protoreflect, TypeScript

**设计文档：** `docs/superpowers/specs/2026-05-22-query-presence-design.md`

---

## 文件结构

| 文件 | 职责 | 变更类型 |
|------|------|----------|
| `internal/plugin/servicegen.go` | 生成器核心，query 参数 presence 判断 | 修改：新增 2 个辅助函数 + 修改 1 行条件输出 |
| `examples/proto/gen/typescript/einride/example/syntax/v1/index.ts` | golden 文件 | 自动重新生成 |
| `examples/proto/gen/typescript/einride/example/freight/v1/index.ts` | golden 文件 | 自动重新生成 |

所有 examples 下的生成文件通过 `make generate` 自动更新，不手工编辑。

---

### 任务 1：新增 `queryPresenceExpr` 和 `queryValueExpr` 辅助函数

**文件：**
- 修改：`internal/plugin/servicegen.go`（新增函数，放置在 `jsonPathSegments` 函数之前或之后）

- [ ] **步骤 1：在 `jsonPath` 函数附近新增 `queryPresenceExpr`**

在 `servicegen.go` 中，`jsonPathSegments` 函数之前（约第 223 行），新增：

```go
// queryPresenceExpr generates a TypeScript nullish check expression for a query field.
// Returns something like "request.pageSize !== undefined && request.pageSize !== null".
// For nested fields, uses optional chaining: "request.nested?.string !== undefined && request.nested?.string !== null".
func queryPresenceExpr(path httprule.FieldPath, message protoreflect.MessageDescriptor) string {
	np := nullPropagationPath(path, message)
	return fmt.Sprintf("request.%s !== undefined && request.%s !== null", np, np)
}

// queryValueExpr generates a TypeScript value access expression for a query field.
// Returns something like "request.pageSize" for direct access.
// For nested fields, uses direct dot access: "request.nested.string".
func queryValueExpr(path httprule.FieldPath, message protoreflect.MessageDescriptor) string {
	jp := jsonPath(path, message)
	return "request." + jp
}
```

- [ ] **步骤 2：运行测试验证编译通过**

```bash
go build ./...
```

预期：编译成功，无错误。

- [ ] **步骤 3：运行单元测试验证现有功能不退化**

```bash
go test ./...
```

预期：所有测试通过。

- [ ] **步骤 4：Commit**

```bash
git add internal/plugin/servicegen.go
git commit -m "feat: add queryPresenceExpr and queryValueExpr helpers"
```

---

### 任务 2：修改 `generateMethodQuery` 使用 nullish 判断

**文件：**
- 修改：`internal/plugin/servicegen.go:199-201`

- [ ] **步骤 1：修改 `generateMethodQuery` 中的条件表达式**

替换 `generateMethodQuery` 函数中（约第 201 行）的 truthy 判断：

**当前代码：**
```go
nullPath := nullPropagationPath(path, input)
jp := jsonPath(path, input)
f.P(t(3), "if (request.", nullPath, ") {")
```

**改为：**
```go
presenceExpr := queryPresenceExpr(path, input)
jp := jsonPath(path, input)
f.P(t(3), "if (", presenceExpr, ") {")
```

注意：`nullPath` 变量已不再需要，可以移除该行。修改后的完整块应为：

```go
walkJSONLeafFields(input, func(path httprule.FieldPath, field protoreflect.FieldDescriptor) {
    if _, ok := pathCovered[path.String()]; ok {
        return
    }
    if rule.Body != "" && path[0] == rule.Body {
        return
    }
    presenceExpr := queryPresenceExpr(path, input)
    jp := jsonPath(path, input)
    f.P(t(3), "if (", presenceExpr, ") {")
    switch {
    case field.IsList():
        f.P(t(4), "request.", jp, ".forEach((x) => {")
        f.P(t(5), "queryParams.push(`", jp, "=${encodeURIComponent(x.toString())}`)")
        f.P(t(4), "})")
    case field.IsMap():
        f.P(t(4), "request.", jp, ".forEach((value, key) => {")
        f.P(t(5), "queryParams.push(`", jp, "[key]=${encodeURIComponent(value.toString())}`)")
        f.P(t(4), "})")
    default:
        f.P(t(4), "queryParams.push(`", jp, "=${encodeURIComponent(request.", jp, ".toString())}`)")
    }
    f.P(t(3), "}")
})
```

- [ ] **步骤 2：编译验证**

```bash
go build ./...
```

预期：编译成功，无错误。

- [ ] **步骤 3：运行单元测试**

```bash
go test ./...
```

预期：所有测试通过。

- [ ] **步骤 4：Commit**

```bash
git add internal/plugin/servicegen.go
git commit -m "feat: replace truthy check with nullish check in query presence"
```

---

### 任务 3：更新 golden 文件（重新生成 + 验证）

**文件：**
- 自动生成：`examples/proto/gen/typescript/einride/example/syntax/v1/index.ts`
- 自动生成：`examples/proto/gen/typescript/einride/example/freight/v1/index.ts`

- [ ] **步骤 1：构建插件**

```bash
make build
```

预期：`bin/protoc-gen-typescript-http` 生成。

- [ ] **步骤 2：重新生成 examples**

```bash
make generate
```

预期：buf generate 成功，输出文件更新。

- [ ] **步骤 3：验证生成的 query 判断已改为 nullish 模式**

检查 `examples/proto/gen/typescript/einride/example/syntax/v1/index.ts` 中的 `QueryOnly` 方法：

```ts
// 预期不再出现：
//   if (request.string) {
// 应该出现：
//   if (request.string !== undefined && request.string !== null) {
```

同样检查 `examples/proto/gen/typescript/einride/example/freight/v1/index.ts` 中的 `ListShippers`：

```ts
// 预期不再出现：
//   if (request.pageSize) {
// 应该出现：
//   if (request.pageSize !== undefined && request.pageSize !== null) {
```

- [ ] **步骤 4：运行 integration 测试确认生成代码与 git 一致**

```bash
make integration
```

预期：所有集成测试通过。如果 integration 失败，先验证步骤 3 的检查。

- [ ] **步骤 5：Commit golden 文件变更**

```bash
git add examples/proto/gen/typescript/
git commit -m "chore: update golden files after query presence change"
```

---

### 任务 4：遍历所有字段形态验证生成正确性

**文件：**
- 验证：生成的分析（无需修改代码）

- [ ] **步骤 1：验证 `int32`（数值类型 0）的生成**

检查 `examples/proto/gen/typescript/einride/example/freight/v1/index.ts` 中 `ListShippers`：

```ts
// 预期：
if (request.pageSize !== undefined && request.pageSize !== null) {
  queryParams.push(`pageSize=${encodeURIComponent(request.pageSize.toString())}`)
}
```

- [ ] **步骤 2：验证 `string`（空字符串 `""`）的生成**

检查 `examples/proto/gen/typescript/einride/example/syntax/v1/index.ts` 中 `QueryOnly`：

```ts
// 预期：
if (request.string !== undefined && request.string !== null) {
  queryParams.push(`string=${encodeURIComponent(request.string.toString())}`)
}
```

- [ ] **步骤 3：验证 `repeated`（空数组 `[]`）的生成**

```ts
// 预期：
if (request.repeatedString !== undefined && request.repeatedString !== null) {
  request.repeatedString.forEach((x) => {
    queryParams.push(`repeatedString=${encodeURIComponent(x.toString())}`)
  })
}
```

- [ ] **步骤 4：验证 nested 字段 `?.` 路径**

```ts
// 预期：
if (request.nested?.string !== undefined && request.nested?.string !== null) {
  queryParams.push(`nested.string=${encodeURIComponent(request.nested.string.toString())}`)
}
```

- [ ] **步骤 5：验证 regression——确保不再出现 truthy 判断**

```bash
! grep -n 'if (request\.' examples/proto/gen/typescript/einride/example/syntax/v1/index.ts | grep -v '!== undefined'
```

预期：所有 `if (request\.` 行都包含 `!== undefined`。

- [ ] **步骤 6：最终 Commit（如果有额外修复）**

```bash
git add -A
git commit -m "test: verify query presence patterns across all field types"
```

---

### 自检

**1. 规格覆盖度：**

| 规格章节 | 对应任务 |
|----------|----------|
| 5.1 抽象 presence 表达式——新增两个 helper | 任务 1 |
| 5.2 统一输出格式——标量字段 nullish 判断 | 任务 2 |
| 5.2 统一输出格式——嵌套字段 `?.` | 任务 2（复用 `nullPropagationPath`） |
| 5.2 统一输出格式——repeated 字段数组本身 nullish 判断 | 任务 2（switch `case field.IsList()`） |
| 6. 影响范围——servicegen.go + 示例输出 | 任务 2（修改）+ 任务 3（重新生成） |
| 7.1 集成 golden 验证——mage integration | 任务 3 步骤 4 |
| 7.2 新增覆盖点——int32/bool/string/repeated 验证 | 任务 4 |
| 7.3 回归检查——不再出现 truthy 判断 | 任务 4 步骤 5 |

**2. 占位符扫描：** 所有步骤包含实际代码、命令和预期输出，无 TODO/待定。

**3. 类型一致性：**
- `queryPresenceExpr` 返回 `string`，使用 `nullPropagationPath`（已有函数）
- `queryValueExpr` 返回 `string`，使用 `jsonPath`（已有函数）
- `generateMethodQuery` 中 `jp` 变量保持不变，`nullPath` 替换为 `presenceExpr`
- 所有被引用的函数（`nullPropagationPath`, `jsonPath`, `walkJSONLeafFields`）均已在代码库中定义

**4. 非目标检查：** 未修改 map query 格式、path 编码、int64 表达、additional_bindings、runtime helper 引入、TypeScript 严格类型化。
