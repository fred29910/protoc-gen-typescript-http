# Repeated Message Query 索引点语法展开设计文档

**日期**: 2026-06-02  
**项目**: protoc-gen-typescript-http  
**状态**: 已批准，待实现  
**关联**: 评审 `2026-06-02-golang-tool-project-review.md` P0 #2

---

## 1. 问题描述

`internal/plugin/jsonleafwalk.go` 的 `walkJSONLeafFields` 在遇到非 map 的 message 字段时递归下钻，但不区分该 message 字段是否为 `repeated`。

当 request 含 `repeated SomeMessage items` 且该字段进入 query 路径时，生成逻辑把它当作单 message 继续下钻，生成类似 `request.items?.name` 的表达式。运行时 `request.items` 是数组而非对象，导致 query 序列化失败，且 `@ts-nocheck` 掩盖了这类问题。

```
.proto
  message Item { string name = 1; int32 age = 2; }
  message GetItemsRequest {
    repeated Item items = 1;
  }
  rpc GetItems(GetItemsRequest) returns (ListItemsResponse) {
    option (google.api.http) = {
      get: "/v1/items"
    };
  }
```

当前生成（错误）：

```ts
if (request.items?.name !== undefined && request.items?.name !== null) {
  // request.items 是数组，没有 .name 属性
  queryParams.push(`items.name=${encodeURIComponent(request.items.name.toString())}`);
}
```

---

## 2. 根因分析

| 层面 | 实际情况 |
|---|---|
| walker | `walkJSONLeafFields` 把所有非 map、非 WKT 的 message 字段无差别递归下钻 |
| servicegen | `generateMethodQuery` 拿到 leaf 后用 `request.<path>` 直接访问，不知道叶子路径里包含 repeated message |
| 错误掩盖 | 生成文件头部含 `// @ts-nocheck`，此类类型错误在 TS 编译期不暴露 |

---

## 3. 产品决策

经评审讨论，选择 **B. 设计展开策略** + **索引点语法** 形式：

- 生成的 query 形如 `items.0.name=foo&items.0.age=1&items.1.name=bar`
- 仅支持 first-level 形态：`repeated Message` 中 Message 只含 scalar/enum/WKT 字段
- 嵌套情形（Message 内含 nested `repeated message` 或 `map<K, message>`）**生成阶段报错**，不静默通过
- 0 个元素 / 元素为 null 时不生成 query（forEach 跳过）

理由：与现有 `repeated scalar` 行为（多 key）保持概念一致；接收方按 `index` 可明确还原元素；生成期报错保证不会出现 runtime 才暴露的错误。

---

## 4. 设计

### 4.1 行为契约

**支持范围**：
- `repeated SomeMessage items`，其中 `SomeMessage` 字段集合为：scalar / enum / WKT message
- 该 message 内 WKT 字段（如 `google.protobuf.StringValue`）作 leaf 走原 `f(path, field)` 路径

**生成期报错**（条件如下时，generator 直接返回 error）：
- `SomeMessage` 含 `repeated OtherMessage` 字段（nested repeated message）
- `SomeMessage` 含 `map<K, OtherMessage>` 字段（nested map-of-message）
- 错误信息包含 `SomeMessage.FullName()` + 嵌套字段路径，便于定位

**不在 P0 范围（不报错也不展开）**：
- `repeated WKT` 字段（`repeated google.protobuf.XxxValue`）— 评审未涉及，保留原行为
- `map<K, scalar>`、`map<K, message>` 走原 map 路径不变

### 4.2 生成器改造

`jsonleafwalk.go` 拆成两个 walker：

```go
// 现有：处理 scalar/enum/single-message/WKT/mixed
func walkJSONLeafFields(message protoreflect.MessageDescriptor, f jsonLeafWalkFunc)

// 新增：处理 repeated message 内的 scalar/enum/WKT leaves
type repeatedMessageLeaf struct {
    RepeatedField protoreflect.FieldDescriptor  // the `items` field
    Element       protoreflect.MessageDescriptor // SomeMessage
    LeafPath      httprule.FieldPath            // path within element
    LeafField     protoreflect.FieldDescriptor  // the leaf field
}
func walkJSONRepeatedMessageLeaves(
    message protoreflect.MessageDescriptor,
    f func(repeatedMessageLeaf),
) error
```

`walkJSONRepeatedMessageLeaves` 在递归下钻 `repeated message` 时检查元素 message 内是否含 nested `repeated message` / `map<K, message>`，发现则返回 error。

### 4.3 servicegen 处理

`generateMethodQuery` 中，对原 `f(path, field)` 回调保持不变；对新 `repeatedMessageLeaf` 回调生成如下结构：

```ts
if (request.items !== undefined && request.items !== null) {
  request.items.forEach((item, index) => {
    if (item.name !== undefined && item.name !== null) {
      queryParams.push(`items.${index}.name=${encodeURIComponent(item.name)}`);
    }
    if (item.age !== undefined && item.age !== null) {
      queryParams.push(`items.${index}.age=${encodeURIComponent(String(item.age))}`);
    }
  });
}
```

每个 leaf 的 `${index}` 模板插入点由 `forEach` 回调的第二个参数提供。

### 4.4 关键约束

- **path 覆盖不变**：若 `repeated message` 字段是 path 变量，重复绑定校验会拦截，不会进入 query
- **body 覆盖不变**：若 `repeated message` 字段被 `body` 字段包含，从 query 排除
- **WKT leaf 走原路径**：repeated message 元素内若含 `google.protobuf.StringValue` 等 WKT 字段，作为 leaf 走 `f(path, field)` 路径，路径仍带 `${index}` 前缀
- **0 元素 / null 元素**：forEach 内部空跑，不生成 query

### 4.5 错误信息示例

生成时若 `Item` 内含 `repeated Tag tags` 字段：

```
generate method GetItems: repeated message field walktest.Item.tags: nested repeated message is not supported in query serialization
```

错误信息含：
- 包含问题字段的 message 全名
- 嵌套字段路径
- 简短的原因说明

---

## 5. 改动文件清单

| 文件 | 改动 |
|---|---|
| `internal/plugin/jsonleafwalk.go` | 新增 `walkJSONRepeatedMessageLeaves` 函数 + `repeatedMessageLeaf` 类型；现有 `walkJSONLeafFields` 保持不变 |
| `internal/plugin/jsonleafwalk_test.go` | 新增测试：first-level repeated message / 嵌套 repeated 报错 / 嵌套 map-message 报错 / WKT leaf inside repeated message |
| `internal/plugin/servicegen.go` | `generateMethodQuery` 增加对 `repeatedMessageLeaf` 回调的 `forEach` 处理 |
| `internal/plugin/servicegen_test.go` | 新增测试：runtime 检查生成代码结构 + 错误注入测试 |
| `examples/proto/` | 增加使用 `repeated message` query 的示例 service |
| `docs/code-generation.md` | "repeated message" 行从缺省描述改为说明索引点语法 + 嵌套报错 |
| `docs/protobuf-annotations.md` | 同步更新 |
| `README.md` | 如有相关功能描述，对齐 |

---

## 6. 测试矩阵

| # | 用例 | 期望 |
|---|---|---|
| 1 | `repeated scalar` query | 原行为不变（回归） |
| 2 | `repeated message`，元素内全 scalar/enum | `items.0.x=...&items.1.x=...` |
| 3 | `repeated message`，元素内含 WKT 字段 | WKT 走 leaf 路径，仍带 `items.${index}.` 前缀 |
| 4 | `repeated message`，元素内含 nested `repeated message` | **生成期报错**，含字段路径 |
| 5 | `repeated message`，元素内含 `map<K, OtherMessage>` | **生成期报错**，含字段路径 |
| 6 | path 变量包含 `repeated message` | 该 repeated 字段从 query 排除（httprule 已拦截）|
| 7 | body 包含 `repeated message` 字段 | 该字段从 query 排除（servicegen 已排除）|
| 8 | 0 元素 / 元素 null | forEach 跳过，不生成 query |
| 9 | 集成测试 `buf generate` golden diff | 通过 |

---

## 7. 风险

| 风险 | 缓解 |
|---|---|
| walker 拆分会带来 API surface 扩大 | 两个 walker 各自单一职责；测试独立覆盖 |
| `forEach` 回调 index 与原 query `forEach` 嵌套 | 缩进与命名变量清晰化；生成代码加 binding 注释 |
| 现有 `repeated message` 用户依赖 buggy 行为 | 该行为本来就是 bug，无用户应依赖 |
| 集成测试 golden diff 会变更 | 一并提交并评审 |
| `repeated WKT` 未处理 | 文档明示为"暂未实现"，留 TODO 标记 |
| nested 报错信息含内部字段路径，可能泄露内部结构 | 路径仅含 proto 字段名（snake_case），不是敏感信息 |

---

## 8. 非目标（YAGNI）

- 不处理 `repeated WKT` 字段展开
- 不实现 protobuf JSON 标准（json_name + camelCase 转换）下的 repeated message 编码
- 不实现 `?items=<URL-encoded JSON>` 单 key 编码
- 不实现 flat 重复 key（评审已否决）
- 不抽出公共 `encodeURIComponent(item.x.toString())` helper

---

## 9. 实施步骤

1. 在 `jsonleafwalk.go` 新增 `walkJSONRepeatedMessageLeaves` + 错误返回
2. 添加单元测试（cases 2-5, 8）
3. 在 `servicegen.go` 的 `generateMethodQuery` 中加入新回调处理 + `forEach` 块生成
4. 添加 servicegen 单元测试（cases 2-3, 6-7）
5. 集成测试：新增示例 proto，运行 `buf generate` + `deno fmt` + `git diff`
6. 文档同步
7. 评审 P0 #2 关闭

---

## 10. 验收标准

- [ ] `go test -count=1 ./...` 全部通过
- [ ] `go vet ./...` 通过
- [ ] `gofmt -l` 无输出
- [ ] 集成测试 `go test -tags integration` 通过
- [ ] `buf generate` + `deno fmt` 后 `git diff examples/proto/gen` 干净
- [ ] 文档中所有 repeated message 描述一致指向索引点语法 + 嵌套报错
- [ ] 评审 P0 #2 关闭
