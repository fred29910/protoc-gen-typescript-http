# HTTP Rule Parsing

将 `google.api.http` 注解模式（例如 `/v1/{name=shippers/*}`）解析为结构化的 URL Template。这是在 TypeScript 客户端中生成正确的 HTTP 方法绑定和构建 URL 的基础。

## Package: `internal/httprule`

### Core types

```go
// A parsed URL path template.
// Example: `/v1/{name=books/*}:publish`
type Template struct {
    Segments []Segment   // Path segments
    Verb     string      // Custom verb (e.g., "publish")
}

// A single path segment.
type Segment struct {
    Kind     SegmentKind      // Literal, MatchSingle, MatchMultiple, Variable
    Literal  string           // For literal segments
    Variable VariableSegment  // For variable segments
}

// A {field.path=sub-template} variable binding.
type VariableSegment struct {
    FieldPath FieldPath  // The protobuf field path (e.g., ["name"])
    Segments  []Segment  // The sub-template for matching (default: [*])
}
```

### Grammar

解析器实现了标准的 `google.api.http` 路径 Template Grammar：

```
Template = "/" Segments [ Verb ]    // always starts with /
Segments = Segment { "/" Segment }
Segment  = "*"                      // match single path segment
         | "**"                     // match remaining path (last segment only)
         | LITERAL                  // exact match
         | Variable                 // field variable binding

Variable = "{" FieldPath [ "=" Segments ] "}"
FieldPath = IDENT { "." IDENT }
Verb     = ":" LITERAL
```

### Examples

| Pattern | Segments | Verb |
|---------|----------|------|
| `/v1/shippers` | `["v1", "shippers"]`（全为 literal） | 无 |
| `/v1/{name=shippers/*}` | `["v1", variable(name → [shippers, *])]` | 无 |
| `/v1/{parent=shippers/*}/sites` | `["v1", variable(parent → [shippers, *]), "sites"]` | 无 |
| `/v1/{string}:path` | `["v1", variable(string → [*])]` | `"path"` |
| `/{name=shippers/*}/shipments/{shipment}` | `[variable(name → [shippers, *]), "shipments", variable(shipment → [*])]` | 无 |

### Validation

解析完成后，验证器会进行以下检查：

1. **不能嵌套 Variable**：拒绝 `{a={b}}`
2. **`**` 只能作为最后一个 Segment**：拒绝 `**/foo`；在 Variable 内部同样适用：拒绝 `{a=**/bar}`
3. **顶层不能有裸露的 `*` 或 `**`**：`*` 和 `**` 必须出现在 `{variable=**}` 内部（即顶层只允许 literal 和 variable 类型 segment）
4. **不能有重复的 Variable 绑定**：拒绝 `{a}/{a}`

### API

```go
// Extract the HTTP annotation from a protobuf method descriptor.
rule, ok := httprule.Get(methodDescriptor)

// Parse the annotation into a structured Rule.
parsed, err := httprule.ParseRule(rule)
// parsed.Method           → "GET", "POST", "FETCH" (custom), etc.
// parsed.Template         → the URL Template
// parsed.Body             → the body field selector ("*", field name, or "")
// parsed.AdditionalRules  → additional_bindings
```

### Custom HTTP Method

`google.api.http` 注解支持 `custom` 模式，用于非标准 HTTP 方法：

```protobuf
option (google.api.http) = {
  custom: {
    kind: "FETCH"
    path: "/v1/action"
  }
};
```

在 `httprule` 包中，`httpRuleMethod()` 函数会从 `custom.kind` 中提取方法名（如 `"FETCH"`），`httpRuleURL()` 从 `custom.path` 中提取 URL 模板。

### Relation to Code Generation

在 `servicegen.go` 中，解析后的规则会驱动四个生成阶段：

1. **Path validation**，为路径 Variable 生成 nullish 检查
   - 简单变量（如 `{id}`）：`if (!request.id) throw new Error("missing required field request.id")`
   - 嵌套变量（如 `{shipper.name}`）：`if (!request.shipper?.name) throw new Error("missing required field request.shipper.name")`

2. **URL construction**，使用变量值生成 template literal
   - 标准变量（如 `{id}`）：`${encodeURIComponent(request.id)}`
   - 通配符子模板变量（如 `{name=shippers/*}`）：`${request.name.split('/').map(p => encodeURIComponent(p)).join('/')}`，保留斜杠的语义结构

3. **Body serialization**，根据 body 选择器生成 `JSON.stringify()` 调用
   - `body: "*"` → `JSON.stringify(request)`
   - `body: "field"` → `JSON.stringify(request?.field ?? {})`
   - `body: "nested.field"` → `JSON.stringify(request?.nested?.field ?? {})`
   - 无 body → `const body = null`

4. **Query parameters**，为所有未被路径或 body 覆盖的字段生成查询字符串构建逻辑
   - 标量字段使用严格的 `!== undefined && !== null` 检查（而非简单的 truthy 检查）
   - repeated 字段使用 `.forEach()` 遍历生成多个同名参数
   - map 字段按 key 排序后生成 `field[key]=value` 格式
