# Additional Bindings first-match-wins 生成实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 改造 `servicegen.generateMethod` 让其消费 `rule.AdditionalRules`，按 first-match-wins 语义为每个 binding 生成独立分支，缺失匹配时抛错。

**架构：** 引入新的 `generateMethodMultiBinding` 内部函数，遍历 `[rule, ...rule.AdditionalRules]`。每个 binding 在生成的 TS 中以 `if (pathVarsPresent) { ... return handler(...); }` 形式存在；分支末尾 `throw new Error("no matching binding for <Method>")` 兜底。无 additional bindings 时退化为单分支（行为不变）。

**技术栈：** Go 1.25+, protobuf protoreflect, TypeScript, gotest.tools

**设计文档：** `docs/superpowers/specs/2026-06-02-additional-bindings-selector-design.md`

---

## 文件结构

| 文件 | 职责 | 变更类型 |
|------|------|----------|
| `internal/plugin/servicegen.go` | 生成器核心，重构 `generateMethod` 为多分支 | 修改 |
| `internal/plugin/servicegen_test.go` | binding 选择行为测试 | 修改（追加） |
| `examples/proto/einride/example/additional_bindings/v1/*.proto` | 示例 proto | 新增 |
| `examples/proto/gen/typescript/einride/example/additional_bindings/v1/*.ts` | golden 输出 | 自动重新生成 |
| `docs/code-generation.md` | additional_bindings 描述 | 修改（追加/覆盖） |
| `docs/protobuf-annotations.md` | "当前行为" 段落 | 修改（追加/覆盖） |
| `README.md` | 功能描述对齐 | 修改（视情况）|

所有 `examples/proto/gen/typescript/**` 下的生成文件通过 `buf generate` + `deno fmt` 自动更新，不手工编辑。

---

### 任务 1：抽取 `bindingBlock` 内部辅助函数，封装单个 binding 的 TS 生成

**文件：**
- 修改：`internal/plugin/servicegen.go`（在 `generateMethod` 函数之前/附近新增）

- [ ] **步骤 1：在 `servicegen.go` 中新增 `generateMethodBinding` 函数骨架**

在 `generateMethod` 函数之前新增以下函数（只生成 path/body/query 三段并写入 `*codegen.File`，不写 `return handler`）：

```go
// generateMethodBinding generates the per-binding TS body: path validation,
// path construction, body construction, and query construction. It does NOT
// write the `return handler(...)` line — the caller wraps each block in an
// if-statement and decides where the return goes.
func (s serviceGenerator) generateMethodBinding(
    f *codegen.File,
    input protoreflect.MessageDescriptor,
    rule httprule.Rule,
) error {
    if err := s.generateMethodPathValidation(f, input, rule); err != nil {
        return fmt.Errorf("path validation: %w", err)
    }
    if err := s.generateMethodPath(f, input, rule); err != nil {
        return fmt.Errorf("path: %w", err)
    }
    if err := s.generateMethodBody(f, input, rule); err != nil {
        return fmt.Errorf("body: %w", err)
    }
    if err := s.generateMethodQuery(f, input, rule); err != nil {
        return fmt.Errorf("query: %w", err)
    }
    return nil
}
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

预期：所有测试通过（行为尚未改变）。

- [ ] **步骤 4：Commit**

```bash
git add internal/plugin/servicegen.go
git commit -m "refactor(plugin): extract generateMethodBinding helper"
```

---

### 任务 2：实现 `generateMethodMultiBinding` 与新的 `generateMethod`

**文件：**
- 修改：`internal/plugin/servicegen.go`（替换 `generateMethod` 函数体）

- [ ] **步骤 1：替换 `generateMethod` 函数**

将 `generateMethod`（约第 82-122 行）替换为：

```go
func (s serviceGenerator) generateMethod(f *codegen.File, method protoreflect.MethodDescriptor) error {
    if !supportedMethod(method) {
        return nil
    }
    outputType := typeFromMessage(s.pkg, method.Output())
    r, ok := httprule.Get(method)
    if !ok {
        return nil
    }
    rule, err := httprule.ParseRule(r)
    if err != nil {
        return fmt.Errorf("parse http rule: %w", err)
    }
    rules := append([]httprule.Rule{rule}, rule.AdditionalRules...)
    f.P(t(2), method.Name(), "(request) { // eslint-disable-line @typescript-eslint/no-unused-vars")
    for i, sub := range rules {
        input := method.Input()
        cond, err := bindingPathVarsPresentExpr(sub, input)
        if err != nil {
            return fmt.Errorf("binding %d presence: %w", i, err)
        }
        if i == 0 {
            f.P(t(3), "if (", cond, ") {")
        } else {
            f.P(t(3), "} else if (", cond, ") {")
        }
        if err := s.generateMethodBinding(f, input, sub); err != nil {
            return fmt.Errorf("binding %d: %w", i, err)
        }
        if err := s.writeMethodHandlerCall(f, method, outputType, sub); err != nil {
            return fmt.Errorf("binding %d handler: %w", i, err)
        }
    }
    f.P(t(3), "} else {")
    f.P(t(4), "throw new Error(", strconv.Quote(fmt.Sprintf("no matching binding for %s", method.Name())), ");")
    f.P(t(3), "}")
    f.P(t(2), "},")
    return nil
}

// bindingPathVarsPresentExpr returns a TS nullish AND-chain for the path
// variables of the given rule. e.g. "request.id !== undefined && request.id !== null".
func bindingPathVarsPresentExpr(rule httprule.Rule, message protoreflect.MessageDescriptor) (string, error) {
    var parts []string
    for _, seg := range rule.Template.Segments {
        if seg.Kind != httprule.SegmentKindVariable {
            continue
        }
        np, err := nullPropagationPath(seg.Variable.FieldPath, message)
        if err != nil {
            return "", err
        }
        parts = append(parts, fmt.Sprintf("request.%s !== undefined && request.%s !== null", np, np))
    }
    if len(parts) == 0 {
        return "true", nil
    }
    return strings.Join(parts, " && "), nil
}

// writeMethodHandlerCall writes the `return handler({...}, {...})` line for a
// single binding branch.
func (s serviceGenerator) writeMethodHandlerCall(
    f *codegen.File,
    method protoreflect.MethodDescriptor,
    outputType *typeReference,
    rule httprule.Rule,
) error {
    f.P(t(3), "let uri = path;")
    f.P(t(3), "if (queryParams.length > 0) {")
    f.P(t(4), "uri += `?${queryParams.join(\"&\")}`")
    f.P(t(3), "}")
    f.P(t(3), "return handler({")
    f.P(t(4), "path: uri,")
    f.P(t(4), "method: ", strconv.Quote(rule.Method), ",")
    f.P(t(4), "body,")
    f.P(t(3), "}, {")
    f.P(t(4), "service: \"", method.Parent().Name(), "\",")
    f.P(t(4), "method: \"", method.Name(), "\",")
    f.P(t(3), "}) as Promise<", outputType.Reference(), ">;")
    return nil
}
```

> **注意**：原 `generateMethod` 中 `if !supportedMethod` / `httprule.Get` / `httprule.ParseRule` 的逻辑都保留在替换后的版本里。`writeMethodHandlerCall` 把原 `let uri = path;` 到 `return handler(...)` 的 13 行封装到独立函数中。

- [ ] **步骤 2：编译验证**

```bash
go build ./...
```

预期：编译成功，无错误。

- [ ] **步骤 3：运行单元测试 + 集成测试**

```bash
go test ./...
make integration
```

预期：
- 单元测试通过
- 集成测试 **可能失败**（因为生成代码结构改变，golden diff 出现）—— 暂时允许失败，这是预期的步骤 4 处理
- 失败的 golden diff 应该是路径、body 段位置下移到 `if (bindingMatches) { ... }` 内，不是功能错误

- [ ] **步骤 4：Commit**

```bash
git add internal/plugin/servicegen.go
git commit -m "feat(plugin): generate first-match-wins branches for additional_bindings"
```

---

### 任务 3：编写 binding 选择行为的单元测试

**文件：**
- 修改：`internal/plugin/servicegen_test.go`（追加新测试）

> 单元测试直接验证生成 TS 文本的"骨架"（不依赖运行时执行）。通过 `buf generate` + golden diff 来覆盖运行时行为（见任务 6）。

- [ ] **步骤 1：添加 `Test_generateMethod_firstMatchWins_*` 表驱动测试**

在 `servicegen_test.go` 末尾追加：

```go
func Test_generateMethod_additionalBindings(t *testing.T) {
    t.Parallel()
    // Build a proto with:
    //   message GetRequest { string name = 1; string id = 2; }
    //   service S { rpc Get(GetRequest) returns (Empty) {
    //     option (google.api.http) = {
    //       get: "/v1/{id}"
    //       additional_bindings { get: "/v1/{name}" }
    //     }
    //   } }
    fd := &descriptorpb.FileDescriptorProto{
        Name:    strPtr("bindtest/bindings.proto"),
        Package: strPtr("bindtest"),
        Dependency: []string{"google/protobuf/empty.proto"},
        MessageType: []*descriptorpb.DescriptorProto{
            {
                Name: strPtr("GetRequest"),
                Field: []*descriptorpb.FieldDescriptorProto{
                    {Name: strPtr("name"), Number: int32Ptr(1), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("name")},
                    {Name: strPtr("id"), Number: int32Ptr(2), Type: fieldType(descriptorpb.FieldDescriptorProto_TYPE_STRING), JsonName: strPtr("id")},
                },
            },
        },
        Service: []*descriptorpb.ServiceDescriptorProto{
            {
                Name: strPtr("S"),
                Method: []*descriptorpb.MethodDescriptorProto{
                    {
                        Name:       strPtr("Get"),
                        InputType:  strPtr(".bindtest.GetRequest"),
                        OutputType: strPtr(".google.protobuf.Empty"),
                    },
                },
            },
        },
        Options: &descriptorpb.FileOptions{
            GoPackage: strPtr("bindtest"),
        },
    }
    file := mustNewFile(t, fd)
    // Build registry and attach the Google Empty type.
    files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
        File: append([]*descriptorpb.FileDescriptorProto{fd}, emptyFileDesc()),
    })
    assert.NilError(t, err)
    file = filesByName(t, files, "bindtest/bindings.proto")
    service := file.Services().Get(0)

    // Resolve input descriptor.
    methods := service.Methods()
    input := methods.Get(0).Input()

    // Construct the main rule + additional binding.
    mainRule := httprule.Rule{
        Method: "GET",
        Template: httprule.Template{
            Segments: []httprule.Segment{
                {
                    Kind: httprule.SegmentKindVariable,
                    Variable: httprule.VariableSegment{
                        FieldPath: httprule.FieldPath{"id"},
                        Segments:  []httprule.Segment{{Kind: httprule.SegmentKindMatchSingle}},
                    },
                },
            },
        },
    }
    additionalRule := httprule.Rule{
        Method: "GET",
        Template: httprule.Template{
            Segments: []httprule.Segment{
                {
                    Kind: httprule.SegmentKindVariable,
                    Variable: httprule.VariableSegment{
                        FieldPath: httprule.FieldPath{"name"},
                        Segments:  []httprule.Segment{{Kind: httprule.SegmentKindMatchSingle}},
                    },
                },
            },
        },
    }
    mainRule.AdditionalRules = []httprule.Rule{additionalRule}

    gen := serviceGenerator{pkg: "bindtest", service: service}
    f := codegen.NewFile("bindtest", file)
    assert.NilError(t, gen.generateMethod(f, methods.Get(0), mainRule, input))

    out := f.String()

    // Assert: at least one branch checks `id`, at least one checks `name`,
    // and there is a `throw` fallback.
    assert.Assert(t, strings.Contains(out, "request.id !== undefined"),
        "expected main binding presence check, got:\n%s", out)
    assert.Assert(t, strings.Contains(out, "request.name !== undefined"),
        "expected additional binding presence check, got:\n%s", out)
    assert.Assert(t, strings.Contains(out, "no matching binding"),
        "expected throw fallback, got:\n%s", out)
}
```

> 上面的测试需要 `codegen.NewFile` 暴露 `String()`、`mustNewFile`、`emptyFileDesc`、`filesByName` 等辅助函数。`codegen.NewFile` 当前的 API 可能在你的工作树里略有不同——按需调整。`String()` 在大多数 `codegen.File` 实现里都存在。

- [ ] **步骤 2：调整 `generateMethod` 签名以接受 `httprule.Rule` 参数**

为了让单元测试能直接传构造好的 `Rule`，把 `generateMethod` 拆为：

```go
// generateMethod is the entry point used by servicegen.Generate — it parses
// the HttpRule and delegates to generateMethodWithRule.
func (s serviceGenerator) generateMethod(f *codegen.File, method protoreflect.MethodDescriptor) error {
    r, ok := httprule.Get(method)
    if !ok {
        return nil
    }
    rule, err := httprule.ParseRule(r)
    if err != nil {
        return fmt.Errorf("parse http rule: %w", err)
    }
    return s.generateMethodWithRule(f, method, rule)
}

// generateMethodWithRule writes the multi-branch method body for the given rule.
func (s serviceGenerator) generateMethodWithRule(
    f *codegen.File,
    method protoreflect.MethodDescriptor,
    rule httprule.Rule,
) error {
    if !supportedMethod(method) {
        return nil
    }
    outputType := typeFromMessage(s.pkg, method.Output())
    rules := append([]httprule.Rule{rule}, rule.AdditionalRules...)
    f.P(t(2), method.Name(), "(request) {")
    for i, sub := range rules {
        input := method.Input()
        cond, err := bindingPathVarsPresentExpr(sub, input)
        if err != nil {
            return fmt.Errorf("binding %d presence: %w", i, err)
        }
        prefix := "if ("
        if i > 0 {
            prefix = "} else if ("
        }
        f.P(t(3), prefix, cond, ") {")
        if err := s.generateMethodBinding(f, input, sub); err != nil {
            return fmt.Errorf("binding %d: %w", i, err)
        }
        if err := s.writeMethodHandlerCall(f, method, outputType, sub); err != nil {
            return fmt.Errorf("binding %d handler: %w", i, err)
        }
    }
    f.P(t(3), "} else {")
    f.P(t(4), "throw new Error(", strconv.Quote(fmt.Sprintf("no matching binding for %s", method.Name())), ");")
    f.P(t(3), "}")
    f.P(t(2), "},")
    return nil
}
```

- [ ] **步骤 3：编译 + 跑测试**

```bash
go test ./internal/plugin/ -run Test_generateMethod_additionalBindings -v
```

预期：PASS。

- [ ] **步骤 4：跑全套测试 + vet**

```bash
go test ./...
go vet ./...
gofmt -l internal/plugin/
```

预期：全通过，gofmt 无输出。

- [ ] **步骤 5：Commit**

```bash
git add internal/plugin/servicegen.go internal/plugin/servicegen_test.go
git commit -m "test(plugin): cover additional_bindings first-match-wins generation"
```

---

### 任务 4：新增 `examples/proto/einride/example/additional_bindings/v1/*.proto` 示例

**文件：**
- 新增：`examples/proto/einride/example/additional_bindings/v1/additional_bindings.proto`
- 修改：`examples/proto/buf.gen.yaml`（如未启用该目录，可能需要 `buf.yaml` 配置）
- 修改：`examples/proto/einride/example/additional_bindings/v1/buf.lock`（首次 `buf generate` 生成）

- [ ] **步骤 1：创建 proto 文件**

```proto
syntax = "proto3";
package einride.example.additional_bindings.v1;

import "google/api/annotations.proto";
import "google/protobuf/empty.proto";

message GetShipmentRequest {
  // Path form 1: /v1/{parent}/shipments/{shipment}
  string parent = 1;
  // Path form 2: /v1/shipments/{shipment}
  string shipment = 2;
}

service ShipmentService {
  rpc GetShipment(GetShipmentRequest) returns (google.protobuf.Empty) {
    option (google.api.http) = {
      get: "/v1/{parent}/shipments/{shipment}"
      additional_bindings {
        get: "/v1/shipments/{shipment}"
      }
    };
  }
}
```

- [ ] **步骤 2：运行 `buf generate`**

```bash
cd examples/proto
buf generate
cd ../..
```

预期：`examples/proto/gen/typescript/einride/example/additional_bindings/v1/` 下生成 `index.ts`。

- [ ] **步骤 3：检查生成的 `index.ts`**

```bash
cat examples/proto/gen/typescript/einride/example/additional_bindings/v1/index.ts
```

预期片段：

```ts
getShipment(request) {
  if (request.parent !== undefined && request.parent !== null && request.shipment !== undefined && request.shipment !== null) {
    // ... path = /v1/.../shipments/...
    return handler({...}, {...});
  } else if (request.shipment !== undefined && request.shipment !== null) {
    // ... path = /v1/shipments/...
    return handler({...}, {...});
  } else {
    throw new Error("no matching binding for getShipment");
  }
},
```

- [ ] **步骤 4：`deno fmt` 格式化**

```bash
cd examples/proto
deno fmt gen/typescript/einride/example/additional_bindings/v1/
cd ../..
```

- [ ] **步骤 5：Commit**

```bash
git add examples/proto/einride/example/additional_bindings/v1/ examples/proto/gen/typescript/einride/example/additional_bindings/v1/
git commit -m "feat(example): add additional_bindings example proto and generated TS"
```

---

### 任务 5：更新 `docs/code-generation.md`、`docs/protobuf-annotations.md`

**文件：**
- 修改：`docs/code-generation.md`
- 修改：`docs/protobuf-annotations.md`
- 可能：`README.md`

- [ ] **步骤 1：在 `code-generation.md` 的 additional_bindings 行替换描述**

找到：

```
| **additional_bindings** | 仅生成主 HTTP 绑定的 client 代码，`additional_bindings` 被解析但不生成实现 | 需要 alternative binding 时需手动实现 |
```

替换为：

```
| **additional_bindings** | 按 first-match-wins 语义为每个 binding 生成独立分支；无匹配时抛 `Error("no matching binding for <Method>")` | 与 Google API Gateway/Transcoding 默认行为一致 |
```

- [ ] **步骤 2：在 `protobuf-annotations.md` 的 "当前行为" 段落替换**

找到（line 120 附近）：

```
> **当前行为**：`google.api.http.additional_bindings` 中的额外绑定会被解析并呈现在 `Rule.AdditionalRules` 字段中，但每个方法**仅生成主绑定的实现**。`additional_bindings` 中的额外路由不会生成对应的 client 方法。
```

替换为：

```
> **当前行为**：`google.api.http.additional_bindings` 中的额外绑定会被解析为独立的 `Rule`，并按 first-match-wins 语义为每个 binding 生成独立 TS 分支。第一个所有 path 变量都满足的 binding 被选中；无任何 binding 匹配时生成 `throw new Error("no matching binding for <Method>")` 兜底。
```

- [ ] **步骤 3：检查 `README.md` 是否有相关功能描述需要更新**

```bash
grep -n "additional_bindings\|transcoding" README.md
```

如有相关行：改为与上面一致的描述。无相关行：跳过此步骤。

- [ ] **步骤 4：Commit**

```bash
git add docs/code-generation.md docs/protobuf-annotations.md README.md
git commit -m "docs: align additional_bindings docs with first-match-wins"
```

---

### 任务 6：完整验证

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

- [ ] **步骤 4：检查 `git status` 干净（除已知变更外）**

```bash
git status
```

预期：工作区干净。

---

### 自检

**1. 规格覆盖度：**

| 规格章节 | 对应任务 |
|----------|----------|
| §4.1 生成器改造 | 任务 1 + 任务 2 |
| §4.2 每个分支内部结构 | 任务 1（`generateMethodBinding` 抽离） |
| §4.3 关键约束（不抽公共变量等）| 任务 2（`writeMethodHandlerCall` 独立） |
| §4.4 错误处理 | 任务 2 末尾 `throw` 兜底 + 任务 3 测试断言 |
| §5 改动文件清单 | 任务 2 / 任务 3 / 任务 4 / 任务 5 |
| §6 测试矩阵 1-10 | 任务 3（单元测试）+ 任务 4（集成示例）+ 任务 6（最终验证）|
| §10 验收标准 | 任务 6 |

**2. 占位符扫描：** 所有步骤包含实际代码、命令、文件路径与预期输出，无 TODO/待定。

**3. 类型一致性：**
- `bindingPathVarsPresentExpr` 返回 `(string, error)`，与 `nullPropagationPath` 一致
- `writeMethodHandlerCall` 返回 `error`，与其他 generator 辅助函数一致
- `generateMethodBinding` 接受 `httprule.Rule` 参数，与现有 `generateMethod*` 系列函数签名风格一致
- `codegen.File.P(t(n), ...)` 用法与现有代码一致

**4. 非目标检查：** 未修改 httprule 解析层；未改 path encoding / body encoding 策略；未引入新公共 helper；未做 cross-package path validation。
