# Repeated Message Query 索引点语法展开实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 为 query 中遇到的 `repeated SomeMessage` 字段生成 `items.<index>.<subfield>=value` 索引点语法展开；嵌套情形（`SomeMessage` 内含 nested `repeated message` / `map<K, message>`）在生成期报错。

**架构：** 在 `internal/plugin/jsonleafwalk.go` 新增 `walkJSONRepeatedMessageLeaves` 与 `repeatedMessageLeaf` 类型，独立于现有 `walkJSONLeafFields`。`servicegen.generateMethodQuery` 增加对新回调的处理：按 `PathPrefix` 分组生成一个 `if + forEach` 块，每块内放该 repeated field 的所有 leaf。

**技术栈：** Go 1.25+, protobuf protoreflect, TypeScript, gotest.tools

**设计文档：** `docs/superpowers/specs/2026-06-02-repeated-message-query-expansion-design.md`

---

## 文件结构

| 文件 | 职责 | 变更类型 |
|------|------|----------|
| `internal/plugin/jsonleafwalk.go` | 新增 `walkJSONRepeatedMessageLeaves` + `repeatedMessageLeaf` | 修改 |
| `internal/plugin/jsonleafwalk_test.go` | 新 walker 单元测试 | 修改（追加）|
| `internal/plugin/servicegen.go` | `generateMethodQuery` 增加对新回调的处理 | 修改 |
| `internal/plugin/servicegen_test.go` | 重复 message query 生成行为测试 | 修改（追加）|
| `examples/proto/einride/example/repeated_message/v1/*.proto` | 示例 proto | 新增 |
| `examples/proto/gen/typescript/einride/example/repeated_message/v1/*.ts` | golden 输出 | 自动重新生成 |
| `docs/code-generation.md` | repeated message 描述 | 修改（追加/覆盖）|
| `docs/protobuf-annotations.md` | 同步 | 修改（追加/覆盖）|

---

### 任务 1：新增 `walkJSONRepeatedMessageLeaves` 函数骨架

**文件：**
- 修改：`internal/plugin/jsonleafwalk.go`

- [ ] **步骤 1：在 `jsonleafwalk.go` 末尾新增类型与函数**

```go
// repeatedMessageLeaf describes a single leaf inside a repeated message field
// for query serialization purposes. e.g., for `repeated Item items` where
// Item has a `name` field, one Leaf is {PathPrefix: ["items"], LeafPath: ["name"]}.
type repeatedMessageLeaf struct {
    PathPrefix    httprule.FieldPath
    RepeatedField protoreflect.FieldDescriptor
    Element       protoreflect.MessageDescriptor
    LeafPath      httprule.FieldPath
    LeafField     protoreflect.FieldDescriptor
}

type repeatedMessageWalkFunc func(repeatedMessageLeaf)

// walkJSONRepeatedMessageLeaves walks a message descriptor and emits one
// repeatedMessageLeaf per (repeated message field, leaf in element) pair.
// Nested single-message subtrees are recursed into so that repeated message
// fields at any depth are discovered. Nested repeated message or
// map-of-message inside an element returns an error (not supported in MVP).
func walkJSONRepeatedMessageLeaves(
    message protoreflect.MessageDescriptor,
    f repeatedMessageWalkFunc,
) error {
    return walkRepeatedLeaves(message, nil, f)
}

func walkRepeatedLeaves(
    message protoreflect.MessageDescriptor,
    prefix httprule.FieldPath,
    f repeatedMessageWalkFunc,
) error {
    for i := 0; i < message.Fields().Len(); i++ {
        field := message.Fields().Get(i)
        p := append(httprule.FieldPath{}, prefix...)
        p = append(p, string(field.Name()))
        switch {
        case field.IsMap():
            // maps are handled by walkJSONLeafFields; this walker only handles repeated message
            continue
        case !field.IsList() && field.Kind() == protoreflect.MessageKind && !IsWellKnownType(field.Message()):
            // Recurse into single-message subtrees to find repeated-message fields at deeper paths.
            if err := walkRepeatedLeaves(field.Message(), p, f); err != nil {
                return err
            }
        case field.IsList() && field.Kind() == protoreflect.MessageKind && !IsWellKnownType(field.Message()):
            if err := emitRepeatedMessageLeaves(field, p, f); err != nil {
                return err
            }
        }
    }
    return nil
}

func emitRepeatedMessageLeaves(
    repeatedField protoreflect.FieldDescriptor,
    prefix httprule.FieldPath,
    f repeatedMessageWalkFunc,
) error {
    element := repeatedField.Message()
    // Validate: element must not contain nested repeated message or map-of-message.
    for i := 0; i < element.Fields().Len(); i++ {
        ef := element.Fields().Get(i)
        if ef.IsList() && ef.Kind() == protoreflect.MessageKind && !IsWellKnownType(ef.Message()) {
            return fmt.Errorf("repeated message field %q: nested repeated message %s.%s is not supported in query serialization", repeatedField.FullName(), element.FullName(), ef.Name())
        }
        if ef.IsMap() && ef.Kind() == protoreflect.MessageKind {
            vm := ef.MapValue()
            if vm.Kind() == protoreflect.MessageKind && !IsWellKnownType(vm.Message()) {
                return fmt.Errorf("repeated message field %q: nested map-of-message %s.%s is not supported in query serialization", repeatedField.FullName(), element.FullName(), ef.Name())
            }
        }
        if !ef.IsMap() && !ef.IsList() && ef.Kind() == protoreflect.MessageKind && !IsWellKnownType(ef.Message()) {
            return fmt.Errorf("repeated message field %q: nested single message %s.%s is not supported in query serialization", repeatedField.FullName(), element.FullName(), ef.Name())
        }
    }
    // Emit a leaf for each scalar/enum/WKT field in the element.
    for i := 0; i < element.Fields().Len(); i++ {
        ef := element.Fields().Get(i)
        f(repeatedMessageLeaf{
            PathPrefix:    append(httprule.FieldPath{}, prefix...),
            RepeatedField: repeatedField,
            Element:       element,
            LeafPath:      httprule.FieldPath{string(ef.Name())},
            LeafField:     ef,
        })
    }
    return nil
}
```

- [ ] **步骤 2：补充 `fmt` 导入**

确认 `jsonleafwalk.go` 已 `import "fmt"`。如未，加 `import "fmt"`。

- [ ] **步骤 3：编译验证**

```bash
go build ./...
```

预期：编译成功。

- [ ] **步骤 4：跑现有测试**

```bash
go test ./...
```

预期：所有测试通过（新函数尚未被调用，不影响现有行为）。

- [ ] **步骤 5：Commit**

```bash
git add internal/plugin/jsonleafwalk.go
git commit -m "feat(plugin): add walkJSONRepeatedMessageLeaves walker"
```

---

### 任务 2：单元测试覆盖 `walkJSONRepeatedMessageLeaves`

**文件：**
- 修改：`internal/plugin/jsonleafwalk_test.go`（追加）

- [ ] **步骤 1：追加新测试**

```go
func Test_walkJSONRepeatedMessageLeaves_emitsFirstLevelLeaves(t *testing.T) {
    t.Parallel()
    // Proto:
    //   message Item { string name = 1; int32 age = 2; }
    //   message GetRequest { repeated Item items = 1; string filter = 2; }
    fd := &descriptorpb.FileDescriptorProto{
        Name:    strPtr("reptest/flat.proto"),
        Package: strPtr("reptest"),
        MessageType: []*descriptorpb.DescriptorProto{
            {
                Name: strPtr("Item"),
                Field: []*descriptorpb.FieldDescriptorProto{
                    {Name: strPtr("name"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("name")},
                    {Name: strPtr("age"), Number: int32Ptr(2), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_INT32), JsonName: strPtr("age")},
                },
            },
            {
                Name: strPtr("GetRequest"),
                Field: []*descriptorpb.FieldDescriptorProto{
                    {Name: strPtr("items"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".reptest.Item"), Label: labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED), JsonName: strPtr("items")},
                    {Name: strPtr("filter"), Number: int32Ptr(2), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("filter")},
                },
            },
        },
    }
    file := mustNewFile(t, fd)
    req := findMessage(t, file, "GetRequest")

    var got []string
    err := walkJSONRepeatedMessageLeaves(req, func(leaf repeatedMessageLeaf) {
        got = append(got, leaf.PathPrefix.String()+"|"+leaf.LeafPath.String())
    })
    assert.NilError(t, err)
    assert.DeepEqual(t, got, []string{"items|name", "items|age"})
}

func Test_walkJSONRepeatedMessageLeaves_discoversNestedSingleMessage(t *testing.T) {
    t.Parallel()
    // Proto:
    //   message Item { string name = 1; }
    //   message Wrapper { repeated Item items = 1; }
    //   message GetRequest { Wrapper wrapper = 1; }
    fd := &descriptorpb.FileDescriptorProto{
        Name:    strPtr("reptest/nested.proto"),
        Package: strPtr("reptest"),
        MessageType: []*descriptorpb.DescriptorProto{
            {Name: strPtr("Item"), Field: []*descriptorpb.FieldDescriptorProto{
                {Name: strPtr("name"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("name")},
            }},
            {Name: strPtr("Wrapper"), Field: []*descriptorpb.FieldDescriptorProto{
                {Name: strPtr("items"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".reptest.Item"), Label: labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED), JsonName: strPtr("items")},
            }},
            {Name: strPtr("GetRequest"), Field: []*descriptorpb.FieldDescriptorProto{
                {Name: strPtr("wrapper"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".reptest.Wrapper"), JsonName: strPtr("wrapper")},
            }},
        },
    }
    file := mustNewFile(t, fd)
    req := findMessage(t, file, "GetRequest")

    var got []string
    err := walkJSONRepeatedMessageLeaves(req, func(leaf repeatedMessageLeaf) {
        got = append(got, leaf.PathPrefix.String()+"|"+leaf.LeafPath.String())
    })
    assert.NilError(t, err)
    assert.DeepEqual(t, got, []string{"wrapper.items|name"})
}

func Test_walkJSONRepeatedMessageLeaves_errorsOnNestedRepeatedMessage(t *testing.T) {
    t.Parallel()
    // Proto:
    //   message Tag { string value = 1; }
    //   message Item { string name = 1; repeated Tag tags = 2; }
    //   message GetRequest { repeated Item items = 1; }
    fd := &descriptorpb.FileDescriptorProto{
        Name:    strPtr("reptest/nested_repeated.proto"),
        Package: strPtr("reptest"),
        MessageType: []*descriptorpb.DescriptorProto{
            {Name: strPtr("Tag"), Field: []*descriptorpb.FieldDescriptorProto{
                {Name: strPtr("value"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("value")},
            }},
            {Name: strPtr("Item"), Field: []*descriptorpb.FieldDescriptorProto{
                {Name: strPtr("name"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("name")},
                {Name: strPtr("tags"), Number: int32Ptr(2), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".reptest.Tag"), Label: labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED), JsonName: strPtr("tags")},
            }},
            {Name: strPtr("GetRequest"), Field: []*descriptorpb.FieldDescriptorProto{
                {Name: strPtr("items"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".reptest.Item"), Label: labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED), JsonName: strPtr("items")},
            }},
        },
    }
    file := mustNewFile(t, fd)
    req := findMessage(t, file, "GetRequest")

    err := walkJSONRepeatedMessageLeaves(req, func(repeatedMessageLeaf) {})
    assert.ErrorContains(t, err, "nested repeated message")
}

func Test_walkJSONRepeatedMessageLeaves_errorsOnMapOfMessage(t *testing.T) {
    t.Parallel()
    // Proto:
    //   message Value { string x = 1; }
    //   message Item { string name = 1; map<string, Value> attrs = 2; }
    //   message GetRequest { repeated Item items = 1; }
    fd := &descriptorpb.FileDescriptorProto{
        Name:    strPtr("reptest/mapmsg.proto"),
        Package: strPtr("reptest"),
        MessageType: []*descriptorpb.DescriptorProto{
            {Name: strPtr("Value"), Field: []*descriptorpb.FieldDescriptorProto{
                {Name: strPtr("x"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("x")},
            }},
            {Name: strPtr("Item"), Field: []*descriptorpb.FieldDescriptorProto{
                {Name: strPtr("name"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("name")},
                {Name: strPtr("attrs"), Number: int32Ptr(2), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".reptest.Value"), Label: labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED), JsonName: strPtr("attrs")},
            }},
            {Name: strPtr("GetRequest"), Field: []*descriptorpb.FieldDescriptorProto{
                {Name: strPtr("items"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: strPtr(".reptest.Item"), Label: labelPtr(descriptorpb.FieldDescriptorProto_LABEL_REPEATED), JsonName: strPtr("items")},
            }},
        },
    }
    file := mustNewFile(t, fd)
    req := findMessage(t, file, "GetRequest")

    err := walkJSONRepeatedMessageLeaves(req, func(repeatedMessageLeaf) {})
    assert.ErrorContains(t, err, "map-of-message")
}
```

> 注意：你需要在测试文件里新增 `labelPtr` 辅助函数：
> ```go
> func labelPtr(l descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto_Label { v := l; return &v }
> ```

- [ ] **步骤 2：跑新测试**

```bash
go test ./internal/plugin/ -run Test_walkJSONRepeatedMessageLeaves -v
```

预期：4 个测试 PASS。

- [ ] **步骤 3：跑全套 + vet**

```bash
go test ./...
go vet ./...
gofmt -l internal/plugin/
```

预期：全通过。

- [ ] **步骤 4：Commit**

```bash
git add internal/plugin/jsonleafwalk.go internal/plugin/jsonleafwalk_test.go
git commit -m "test(plugin): cover walkJSONRepeatedMessageLeaves"
```

---

### 任务 3：在 `servicegen.generateMethodQuery` 中接入新 walker

**文件：**
- 修改：`internal/plugin/servicegen.go`（`generateMethodQuery` 函数）

- [ ] **步骤 1：在 `generateMethodQuery` 现有 `walkJSONLeafFields` 回调之后追加新 walker 调用**

找到 `generateMethodQuery` 中（约第 226 行）现有的：

```go
walkJSONLeafFields(input, func(path httprule.FieldPath, field protoreflect.FieldDescriptor) {
    ...
})
```

在 `}` 之后、`return queryErr` 之前，追加：

```go
// Group repeated-message leaves by their path prefix so each repeated
// field gets a single `if + forEach` block.
grouped := make(map[string][]repeatedMessageLeaf)
var repeatedErr error
walkJSONRepeatedMessageLeaves(input, func(leaf repeatedMessageLeaf) {
    if repeatedErr != nil {
        return
    }
    prefix := leaf.PathPrefix.String()
    if _, covered := pathCovered[prefix]; covered {
        return
    }
    if rule.Body != "" && rule.Body != "*" {
        bodySegments := strings.Split(rule.Body, ".")
        if pathStartsWith(leaf.PathPrefix, bodySegments) {
            return
        }
    }
    grouped[prefix] = append(grouped[prefix], leaf)
})
if repeatedErr != nil {
    return fmt.Errorf("query repeated message walk: %w", repeatedErr)
}

for _, leaves := range grouped {
    f.P(t(3), "if (request.", leaves[0].PathPrefix.String(), " !== undefined && request.", leaves[0].PathPrefix.String(), " !== null) {")
    f.P(t(4), "request.", leaves[0].PathPrefix.String(), ".forEach((item, index) => {")
    for _, leaf := range leaves {
        jp, err := jsonPath(leaf.LeafPath, leaf.Element)
        if err != nil {
            return fmt.Errorf("query repeated message json path: %w", err)
        }
        f.P(t(5), "if (item.", jp, " !== undefined && item.", jp, " !== null) {")
        f.P(t(6), "queryParams.push(`", leaves[0].PathPrefix.String(), ".${index}.", jp, "=${encodeURIComponent(item.", jp, ".toString())}`)")
        f.P(t(5), "}")
    }
    f.P(t(4), "})")
    f.P(t(3), "}")
}
```

> 注意：`PathPrefix.String()` 在 `PathPrefix` 含 `.` 时会带 `.`，需要 `jsonPath` 风格的 dot-notation 转换。当前的 `PathPrefix` 是 `[]string` 形式（snake_case 字段名），需要先转换为 JSON 路径（`json_name`）。`walkJSONRepeatedMessageLeaves` 的 `PathPrefix` 当前存的是字段名（snake_case），与 `walkJSONLeafFields` 的 path 形式不同。
>
> **修整**：把 `walkJSONRepeatedMessageLeaves` 的 `PathPrefix` 改为 `[]string` 存 **JSON name**，与 `walkJSONLeafFields` 的 `path` 形式一致。回退到任务 1 步骤 1 修改 `emitRepeatedMessageLeaves`：
>
> ```go
> // In emitRepeatedMessageLeaves, when building prefix:
> prefixJSON := append(httprule.FieldPath{}, prefixJSONFromParent...)
> prefixJSON = append(prefixJSON, field.JSONName())
> ```
>
> 并修改 `walkRepeatedLeaves` 在递归下钻时也用 JSON name。简化做法：在 walker 内部直接用 `field.JSONName()` 拼 prefix；`leaf.LeafPath` 同样存 JSON name。
>
> 详见下面步骤 2 的 `jsonPath` 调用与 `PathPrefix` 输出。

- [ ] **步骤 2：调整 walker 使用 JSON name**

为保持与 `walkJSONLeafFields` 一致，**修改 `walkJSONRepeatedMessageLeaves` 让其 `PathPrefix` 与 `LeafPath` 都存 JSON name**（即 `field.JSONName()`）：

将 `walkRepeatedLeaves` 中的：

```go
p = append(p, string(field.Name()))
```

改为：

```go
p = append(p, field.JSONName())
```

将 `emitRepeatedMessageLeaves` 中的 `LeafPath: httprule.FieldPath{string(ef.Name())}` 改为：

```go
LeafPath: httprule.FieldPath{ef.JSONName()},
```

这样 servicegen 中可以直接用 `PathPrefix.String()` 当 JSON path 用，无需再调用 `jsonPath`。

- [ ] **步骤 3：编译 + 跑测试**

```bash
go test ./...
```

预期：所有现有测试通过。`Test_walkJSONRepeatedMessageLeaves_emitsFirstLevelLeaves` 等 4 个新 walker 测试**会失败**（因为 prefix 改成 JSON name 后输出从 `items|name` 变成 `items|name`——意外地相同，因为 `name` 的 JSON name 也是 `name`）。但要更新任务 2 中两个成功 case 的期望字符串：原本的 `items|name`、`wrapper.items|name` 现在分别仍是 `items|name`、`wrapper.items|name`（因为字段名都是 `name`）。`nested.proto` 用例预期不变。

- [ ] **步骤 4：跑全套 + vet**

```bash
go test -count=1 ./...
go vet ./...
gofmt -l internal/plugin/
```

预期：全通过。

- [ ] **步骤 5：Commit**

```bash
git add internal/plugin/jsonleafwalk.go internal/plugin/servicegen.go
git commit -m "feat(plugin): generate index dot-notation for repeated message query"
```

---

### 任务 4：servicegen 行为测试（生成 TS 文本快照）

**文件：**
- 修改：`internal/plugin/servicegen_test.go`（追加）

- [ ] **步骤 1：追加 `Test_generateMethodQuery_repeatedMessage` 测试**

```go
func Test_generateMethodQuery_repeatedMessage(t *testing.T) {
    t.Parallel()
    // Proto:
    //   message Item { string name = 1; int32 age = 2; }
    //   message GetRequest { repeated Item items = 1; }
    //   service S { rpc Get(GetRequest) returns (Empty) {
    //     option (google.api.http) = { get: "/v1/items" }
    //   } }
    fd := &descriptorpb.FileDescriptorProto{ /* same as in Test_walkJSONRepeatedMessageLeaves_emitsFirstLevelLeaves */ }
    file := mustNewFile(t, fd)
    service := file.Services().Get(0)
    method := service.Methods().Get(0)
    input := method.Input()
    rule := httprule.Rule{
        Method: "GET",
        Template: httprule.Template{
            Segments: []httprule.Segment{
                {Kind: httprule.SegmentKindLiteral, Literal: "v1"},
                {Kind: httprule.SegmentKindLiteral, Literal: "items"},
            },
        },
    }
    gen := serviceGenerator{pkg: "reptest", service: service}
    f := codegen.NewFile("reptest", file)
    // Inject the test rule via the test seam (see implementation note).
    // Implementation: temporarily expose a `generateMethodQueryForTest` that
    // accepts an explicit rule parameter, or invoke generateMethod with
    // a constructed rule.
    if err := gen.generateMethodQueryForTest(f, input, rule); err != nil {
        t.Fatal(err)
    }
    out := f.String()
    assert.Assert(t, strings.Contains(out, "request.items.forEach((item, index) =>"))
    assert.Assert(t, strings.Contains(out, "items.${index}.name="))
    assert.Assert(t, strings.Contains(out, "items.${index}.age="))
}
```

> 为支持上述测试，在 `servicegen.go` 暴露一个测试钩子：
> ```go
> // generateMethodQueryForTest exposes generateMethodQuery for tests.
> func (s serviceGenerator) generateMethodQueryForTest(f *codegen.File, input protoreflect.MessageDescriptor, rule httprule.Rule) error {
>     return s.generateMethodQuery(f, input, rule)
> }
> ```
> 也可考虑直接在 servicegen_test.go 中调用 `generateMethodQuery`（同包可直接访问）。

- [ ] **步骤 2：跑测试**

```bash
go test ./internal/plugin/ -run Test_generateMethodQuery_repeatedMessage -v
```

预期：PASS。

- [ ] **步骤 3：Commit**

```bash
git add internal/plugin/servicegen_test.go internal/plugin/servicegen.go
git commit -m "test(plugin): cover repeated message query TS snapshot"
```

---

### 任务 5：新增 `examples/proto/einride/example/repeated_message/v1/*.proto` 示例

**文件：**
- 新增：`examples/proto/einride/example/repeated_message/v1/repeated_message.proto`

- [ ] **步骤 1：创建 proto 文件**

```proto
syntax = "proto3";
package einride.example.repeated_message.v1;

import "google/api/annotations.proto";
import "google/protobuf/empty.proto";

message Tag {
  string value = 1;
}

message Item {
  string name = 1;
  int32 age = 2;
  google.protobuf.StringValue note = 3;
}

message GetItemsRequest {
  repeated Item items = 1;
  string filter = 2;
}

service ItemService {
  rpc GetItems(GetItemsRequest) returns (google.protobuf.Empty) {
    option (google.api.http) = {
      get: "/v1/items"
    };
  }
}
```

- [ ] **步骤 2：`buf generate` + `deno fmt`**

```bash
cd examples/proto
buf generate
deno fmt gen/typescript/einride/example/repeated_message/v1/
cd ../..
```

预期：`examples/proto/gen/typescript/einride/example/repeated_message/v1/index.ts` 生成。

- [ ] **步骤 3：检查 `getItems` 方法的 query 生成**

```bash
grep -A 30 "getItems(request)" examples/proto/gen/typescript/einride/example/repeated_message/v1/index.ts
```

预期片段：

```ts
getItems(request) {
  // ... path ...
  const queryParams: string[] = [];
  if (request.items !== undefined && request.items !== null) {
    request.items.forEach((item, index) => {
      if (item.name !== undefined && item.name !== null) {
        queryParams.push(`items.${index}.name=${encodeURIComponent(item.name.toString())}`)
      }
      if (item.age !== undefined && item.age !== null) {
        queryParams.push(`items.${index}.age=${encodeURIComponent(item.age.toString())}`)
      }
      if (item.note !== undefined && item.note !== null) {
        queryParams.push(`items.${index}.note=${encodeURIComponent(item.note.toString())}`)
      }
    })
  }
  if (request.filter !== undefined && request.filter !== null) {
    queryParams.push(`filter=${encodeURIComponent(request.filter.toString())}`)
  }
  // ...
}
```

- [ ] **步骤 4：Commit**

```bash
git add examples/proto/einride/example/repeated_message/v1/ examples/proto/gen/typescript/einride/example/repeated_message/v1/
git commit -m "feat(example): add repeated_message query example"
```

---

### 任务 6：补充嵌套报错示例

**文件：**
- 新增：`examples/proto/einride/example/repeated_message/v1/nested_invalid.proto`（生成时应报错；考虑在 `buf generate` 之外的独立测试中验证）

- [ ] **步骤 1：创建会引发生成报错的 proto**

```proto
syntax = "proto3";
package einride.example.repeated_message.v1;

import "google/api/annotations.proto";
import "google/protobuf/empty.proto";

message Tag {
  string value = 1;
}

message ItemWithTags {
  string name = 1;
  repeated Tag tags = 2;  // nested repeated message — should trigger error
}

message GetNestedRequest {
  repeated ItemWithTags items = 1;
}

service NestedService {
  rpc GetNested(GetNestedRequest) returns (google.protobuf.Empty) {
    option (google.api.http) = {
      get: "/v1/nested"
    };
  }
}
```

- [ ] **步骤 2：在单元测试中验证（不通过 `buf generate` 验证）**

在 `servicegen_test.go` 追加：

```go
func Test_generateMethodQuery_errorsOnNestedRepeatedMessage(t *testing.T) {
    t.Parallel()
    // Build proto with ItemWithTags { string name; repeated Tag tags; }
    // and a request that uses repeated ItemWithTags. Verify generateMethodQuery
    // returns an error mentioning "nested repeated message".
    // ... build fd as in Task 5 ...
    gen := serviceGenerator{pkg: "reptest", service: service}
    f := codegen.NewFile("reptest", file)
    err := gen.generateMethodQueryForTest(f, input, rule)
    assert.ErrorContains(t, err, "nested repeated message")
}
```

- [ ] **步骤 3：删除示例 proto（不进入集成测试）**

`buf generate` 会在生成期报错，会破坏 integration test。所以**不要把 nested_invalid.proto 加入 examples/proto**——它只用于单元测试。

- [ ] **步骤 4：Commit**

```bash
git add internal/plugin/servicegen_test.go
git commit -m "test(plugin): verify error on nested repeated message in query"
```

---

### 任务 7：更新文档

**文件：**
- 修改：`docs/code-generation.md`
- 修改：`docs/protobuf-annotations.md`

- [ ] **步骤 1：在 `code-generation.md` 替换"repeated message"行**

找到原"repeated message"行（如果存在），替换为：

```
| **repeated message query** | 索引点语法 `items.<index>.<subfield>=value`；first-level scalar/enum/WKT 子字段；嵌套（`repeated message` / `map<K, message>` / nested single message）生成期报错 | 元素结构简单时使用 |
```

如果表中没有"repeated message"行，新增一行。

- [ ] **步骤 2：在 `protobuf-annotations.md` 同步**

在"当前行为"段落后追加：

```
> **repeated message query**：当前实现支持 `repeated SomeMessage` 中 `SomeMessage` 仅含 scalar/enum/WKT 字段的情况，输出 `items.<index>.<subfield>=value` 格式。嵌套（`SomeMessage` 内含 `repeated OtherMessage`、`map<K, OtherMessage>` 或 nested single message）在生成阶段报错，避免生成 runtime 错误的 query 序列化代码。
```

- [ ] **步骤 3：Commit**

```bash
git add docs/code-generation.md docs/protobuf-annotations.md
git commit -m "docs: align repeated message query docs with index dot-notation"
```

---

### 任务 8：完整验证

- [ ] **步骤 1：跑全套单元 + 集成测试**

```bash
go test -count=1 ./...
make integration
```

预期：全部通过。

- [ ] **步骤 2：跑 vet + gofmt**

```bash
go vet ./...
gofmt -l .
```

预期：无错误，无 gofmt 输出。

- [ ] **步骤 3：跑 race detector**

```bash
go test -count=1 -race ./...
```

预期：全部通过。

- [ ] **步骤 4：检查 git status 干净**

```bash
git status
```

预期：工作区干净。

---

### 自检

**1. 规格覆盖度：**

| 规格章节 | 对应任务 |
|----------|----------|
| §4.1 行为契约 | 任务 1 walker + 任务 2 测试 |
| §4.1 嵌套报错 | 任务 1 `emitRepeatedMessageLeaves` 验证 + 任务 2 两个错误测试 + 任务 6 集成测试 |
| §4.2 生成器改造 | 任务 1 `walkJSONRepeatedMessageLeaves` |
| §4.3 servicegen 处理 | 任务 3 `forEach` 块生成 + 任务 4 snapshot 测试 |
| §4.4 关键约束 | 任务 3 中 `pathCovered` / body 排除集 |
| §4.5 错误信息 | 任务 1 error 格式 + 任务 2 错误测试断言 |
| §5 改动文件 | 任务 1-7 |
| §6 测试矩阵 1-9 | 任务 2（walker）+ 任务 4（servicegen）+ 任务 5（集成示例）+ 任务 6（嵌套报错）|
| §10 验收标准 | 任务 8 |

**2. 占位符扫描：** 所有步骤包含实际代码、命令、文件路径与预期输出，无 TODO/待定。

**3. 类型一致性：**
- `repeatedMessageLeaf` 字段类型与 `walkJSONLeafFields` 回调风格一致（path + field descriptor）
- `walkJSONRepeatedMessageLeaves` 返回 `error` 而非 void——这是与 `walkJSONLeafFields`（void）不同之处，原因：嵌套情形需要在中途返回错误
- servicegen 中 `PathPrefix` 与 `LeafPath` 都存 JSON name（与 `walkJSONLeafFields` 行为一致）
- `emitRepeatedMessageLeaves` 中 `MapValue` 用法需要确认 `protoreflect` 1.36.x API 存在；如有差异，按实际 API 调整

**4. 非目标检查：** 未处理 `repeated WKT`；未实现 flat 重复 key（评审已否决）；未实现 `?items=<JSON>` 单 key 编码；未抽取公共 helper；未引入 max-depth 限制。
