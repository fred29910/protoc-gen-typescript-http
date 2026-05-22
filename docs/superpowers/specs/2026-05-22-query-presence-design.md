# Query 参数 Presence 设计文档

**日期**: 2026-05-22
**项目**: protoc-gen-typescript-http
**状态**: 设计稿

## 1. 目标

修复生成的 TypeScript HTTP client 对 query 参数的存在性判断，避免 `0`、`false`、`""` 被错误地当成“未提供”而跳过。

本次变更只处理 query 参数的 presence 语义，不扩展到 map query 格式、path 编码、int64 表达或 additional bindings。

## 2. 当前问题

当前 `servicegen.go` 在生成 query 参数时使用 truthy 判断：

```ts
if (request.pageSize) {
  queryParams.push(`pageSize=${encodeURIComponent(request.pageSize.toString())}`)
}
```

这会带来两个错误：

1. `0`、`false`、`""` 被当成未提供而跳过
2. 生成语义与 TypeScript 类型中的 `T | undefined` 不一致，调用方无法显式传入这些合法值

## 3. 设计原则

1. **只把 `undefined` 和 `null` 视为未提供**
2. **保留显式值**，即使该值是 falsy
3. **不引入运行时 helper**，由 Go 生成器直接输出正确的 TS 条件表达式
4. **保持最小变更**，只改 query presence 判断和对应测试

## 4. 语义定义

query 参数的 presence 规则统一定义为：

```ts
value !== undefined && value !== null
```

在这个规则下：

| 值 | 行为 |
|---|---|
| `undefined` | 不生成 query 参数 |
| `null` | 不生成 query 参数 |
| `0` | 生成 query 参数 |
| `false` | 生成 query 参数 |
| `""` | 生成 query 参数，结果为 `field=` |
| `[]` | 数组为空时不生成任何元素 |
| `{}` | map 为空时不生成任何元素 |

## 5. 生成器改动

### 5.1 抽象 presence 表达式

在 `internal/plugin/servicegen.go` 附近引入小型生成辅助函数，用于统一生成 query 条件判断。

建议新增两个 helper：

```go
func queryPresenceExpr(path httprule.FieldPath, message protoreflect.MessageDescriptor) string
func queryValueExpr(path httprule.FieldPath, message protoreflect.MessageDescriptor) string
```

职责如下：

1. `queryPresenceExpr` 生成 nullish 判断
2. `queryValueExpr` 生成可直接用于 `toString()` 的 JS 访问表达式

### 5.2 统一输出格式

对于标量字段，生成：

```ts
if (request.pageSize !== undefined && request.pageSize !== null) {
  queryParams.push(`pageSize=${encodeURIComponent(request.pageSize.toString())}`)
}
```

对于嵌套字段，生成：

```ts
if (request.nested?.string !== undefined && request.nested?.string !== null) {
  queryParams.push(`nested.string=${encodeURIComponent(request.nested.string.toString())}`)
}
```

对于 repeated 字段，数组本身使用 nullish 判断：

```ts
if (request.repeatedString !== undefined && request.repeatedString !== null) {
  request.repeatedString.forEach((x) => {
    queryParams.push(`repeatedString=${encodeURIComponent(x.toString())}`)
  })
}
```

## 6. 影响范围

### 6.1 受影响代码

- `internal/plugin/servicegen.go`

### 6.2 可能需要同步更新的示例输出

- `examples/proto/gen/typescript/einride/example/syntax/v1/index.ts`
- `examples/proto/gen/typescript/einride/example/freight/v1/index.ts`
- `examples/proto/gen/typescript/einride/example/syntax/v2/index.ts`

这些文件应通过重新生成更新，而不是手工编辑。

## 7. 测试策略

### 7.1 集成 golden 验证

继续使用现有 `mage integration` / `make integration` 作为主验证方式，确认生成输出无 diff。

### 7.2 新增覆盖点

至少覆盖以下字段形态：

1. `int32` 或 `int64` query 字段，验证 `0` 生成 query
2. `bool` query 字段，验证 `false` 生成 query
3. `string` query 字段，验证 `""` 生成 `field=`
4. `repeated` query 字段，验证空数组不生成任何参数

### 7.3 回归检查

测试中应显式验证生成代码不再使用 truthy 判断，例如避免出现：

```ts
if (request.pageSize) {
```

## 8. 非目标

本次不处理以下内容：

1. map query 的生成语义修正
2. path 参数编码问题
3. int64 / uint64 的 JSON 表达修正
4. `additional_bindings` 生成
5. runtime helper 引入
6. TypeScript 严格类型化重构

## 9. 规格自检

- [x] 占位符扫描: 无 TODO、待定或模糊章节
- [x] 内部一致性: presence 规则与示例代码一致
- [x] 范围检查: 只覆盖 query presence，不扩展到其他生成逻辑
- [x] 模糊性检查: 明确选择 `undefined` / `null` 为唯一未提供值
