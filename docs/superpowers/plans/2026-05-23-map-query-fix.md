# Map Query 生成修复 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 修复 `internal/plugin/servicegen.go` 中 map 字段 query 参数生成时错误调用 `.forEach()` 导致运行时崩溃的 bug，改为使用 `Object.entries().forEach()`。

**架构：** 生成器（Go）直接修改两行字符串模板，生成的 TypeScript 从调用 ES6 Map 的 `.forEach()` 改为调用 plain object 的 `Object.entries().forEach()`。同时在 proto 示例中补充 map 字段的测试 RPC，覆盖集成测试盲区。

**技术栈：** Go 1.25+、protobuf/protoreflect、buf v1.69+、mage v1.17+

---

## 文件清单

| 文件 | 操作 | 说明 |
|---|---|---|
| `internal/plugin/servicegen.go` | 修改第 208-209 行 | 核心 bug 修复 |
| `examples/proto/einride/example/syntax/v1/syntax_service.proto` | 修改 | 新增 `MapQuery` RPC 和 map 字段到 `Request` |
| `examples/proto/gen/typescript/einride/example/syntax/v1/index.ts` | 重新生成 | golden 文件，由 `mage integration` 自动更新 |

---

## 任务 1：修复 `servicegen.go` 的 `IsMap()` 分支

**文件：**
- 修改：`internal/plugin/servicegen.go:207-210`

### 背景

`generateMethodQuery` 函数在 `field.IsMap()` 分支生成如下 TypeScript：

```typescript
// 当前（错误）
request.annotations.forEach((value, key) => {
  queryParams.push(`annotations[key]=${encodeURIComponent(value.toString())}`)
})
```

这有两个 bug：
1. plain object `{ [key: string]: V }` 没有 `.forEach()` 方法 → `TypeError` 崩溃
2. 模板中 `[key]` 是字面量字符串，不是变量 → 永远输出 `annotations[key]=foo`

- [ ] **步骤 1：打开并确认当前代码**

  打开 `internal/plugin/servicegen.go`，定位到第 207-210 行，确认当前内容为：

  ```go
  case field.IsMap():
      f.P(t(4), "request.", jp, ".forEach((value, key) => {")
      f.P(t(5), "queryParams.push(`", jp, "[key]=${encodeURIComponent(value.toString())}`)")
      f.P(t(4), "})")
  ```

- [ ] **步骤 2：应用修复**

  将上述代码替换为：

  ```go
  case field.IsMap():
      f.P(t(4), "Object.entries(request.", jp, ").forEach(([key, value]) => {")
      f.P(t(5), "queryParams.push(`", jp, "[${key}]=${encodeURIComponent(value.toString())}`)")
      f.P(t(4), "})")
  ```

  注意两处改动：
  - `"request.", jp, ".forEach((value, key) => {"` → `"Object.entries(request.", jp, ").forEach(([key, value]) => {"`
  - `"[key]=${..."` → `"[${key}]=${..."` （`key` 改为模板变量）

- [ ] **步骤 3：确认文件无语法错误**

  运行：
  ```bash
  go build ./...
  ```
  预期：无输出，exit code 0

- [ ] **步骤 4：运行单元测试（已有测试）**

  运行：
  ```bash
  go test ./...
  ```
  预期输出示例：
  ```
  ok   github.com/go-kratos/protoc-gen-typescript-http/internal/httprule   (cached)
  ```
  预期：所有已有测试通过，exit code 0

- [ ] **步骤 5：commit**

  ```bash
  git add internal/plugin/servicegen.go
  git commit -m "fix: use Object.entries for map query params instead of .forEach"
  ```

---

## 任务 2：在 proto 示例中添加 map 字段和测试 RPC

**目的：** 当前集成测试从未覆盖 `IsMap()` 分支（`Request` 消息中没有 map 字段）。添加一个含 map 字段的 `MapQuery` RPC，让集成测试覆盖刚修复的代码路径。

**文件：**
- 修改：`examples/proto/einride/example/syntax/v1/syntax_service.proto`
- 重新生成（由 `mage integration` 自动完成）：`examples/proto/gen/typescript/einride/example/syntax/v1/index.ts`

- [ ] **步骤 1：在 `Request` 消息中添加 map 字段**

  打开 `examples/proto/einride/example/syntax/v1/syntax_service.proto`，在 `message Request` 末尾追加一个 map 字段：

  当前 `Request`（第 44-51 行）：
  ```protobuf
  message Request {
    string string = 1;
    repeated string repeated_string = 2;
    message Nested {
      string string = 1;
    }
    Nested nested = 3;
  }
  ```

  修改为：
  ```protobuf
  message Request {
    string string = 1;
    repeated string repeated_string = 2;
    message Nested {
      string string = 1;
    }
    Nested nested = 3;
    map<string, string> labels = 4;
  }
  ```

- [ ] **步骤 2：在 `SyntaxService` 中添加 `MapQuery` RPC**

  在 `service SyntaxService` 的末尾（`PathBody` RPC 之后，`}` 之前）追加：

  ```protobuf
  rpc MapQuery(Request) returns (Message) {
    option (google.api.http) = {get: "/v1:mapQuery"};
  }
  ```

  最终 `syntax_service.proto` 完整内容如下：

  ```protobuf
  syntax = "proto3";

  package einride.example.syntax.v1;

  import "einride/example/syntax/v1/syntax.proto";
  import "google/api/annotations.proto";
  import "google/protobuf/empty.proto";

  service SyntaxService {
    rpc QueryOnly(Request) returns (Message) {
      option (google.api.http) = {get: "/v1"};
    }

    rpc EmptyVerb(google.protobuf.Empty) returns (google.protobuf.Empty) {
      option (google.api.http) = {get: "/v1:emptyVerb"};
    }

    rpc StarBody(Request) returns (Message) {
      option (google.api.http) = {
        post: "/v1:starBody"
        body: "*"
      };
    }

    rpc Body(Request) returns (Message) {
      option (google.api.http) = {
        post: "/v1:body"
        body: "nested"
      };
    }

    rpc Path(Request) returns (Message) {
      option (google.api.http) = {post: "/v1/{string}:path"};
    }

    rpc PathBody(Request) returns (Message) {
      option (google.api.http) = {
        post: "/v1/{string}:pathBody"
        body: "nested"
      };
    }

    rpc MapQuery(Request) returns (Message) {
      option (google.api.http) = {get: "/v1:mapQuery"};
    }
  }

  message Request {
    string string = 1;
    repeated string repeated_string = 2;
    message Nested {
      string string = 1;
    }
    Nested nested = 3;
    map<string, string> labels = 4;
  }
  ```

- [ ] **步骤 3：构建插件**

  ```bash
  mage build
  ```
  预期：`bin/protoc-gen-typescript-http` 生成，exit code 0

- [ ] **步骤 4：重新生成 golden 文件**

  ```bash
  cd examples/proto && buf generate
  ```
  预期：`examples/proto/gen/typescript/einride/example/syntax/v1/index.ts` 更新，exit code 0

  > 如果 `buf` 未在 PATH 中，可用 `.tools/buf generate`（参见 Makefile）。

- [ ] **步骤 5：手动验证生成代码中 MapQuery 的 map 处理**

  查看生成的 `index.ts`，找到 `MapQuery` 方法，确认 `labels` 字段的 query 参数生成代码为：

  ```typescript
  if (request.labels !== undefined && request.labels !== null) {
    Object.entries(request.labels).forEach(([key, value]) => {
      queryParams.push(`labels[${key}]=${encodeURIComponent(value.toString())}`)
    })
  }
  ```

  关键点：
  - 使用 `Object.entries(request.labels)`，而非 `request.labels.forEach`
  - 模板中使用 `[${key}]`（变量），而非 `[key]`（字面量）

  运行验证命令：
  ```bash
  grep -A4 "Object.entries(request.labels)" examples/proto/gen/typescript/einride/example/syntax/v1/index.ts
  ```
  预期：输出包含上述代码块

- [ ] **步骤 6：运行集成测试**

  ```bash
  go test -tags integration ./tests/integration/...
  ```
  预期：`PASS`，exit code 0

  > 集成测试通过 `buf generate` + `git diff` 校验 golden 文件无意外变更，需要先完成步骤 4 的 golden 更新。

- [ ] **步骤 7：commit**

  ```bash
  git add examples/proto/einride/example/syntax/v1/syntax_service.proto \
          examples/proto/gen/typescript/einride/example/syntax/v1/index.ts
  git commit -m "test: add MapQuery RPC with map field to cover IsMap() query branch"
  ```

---

## 任务 3：最终验证

- [ ] **步骤 1：运行全量测试**

  ```bash
  go test ./...
  ```
  预期：所有包通过，exit code 0

- [ ] **步骤 2：运行集成测试**

  ```bash
  mage build && go test -tags integration ./tests/integration/...
  ```
  预期：`PASS`，exit code 0

- [ ] **步骤 3：确认 git 状态干净**

  ```bash
  git status
  ```
  预期：`nothing to commit, working tree clean`

---

## 成功标准

1. `go build ./...` 无错误
2. `go test ./...` 全部通过
3. 生成的 `index.ts` 中 `MapQuery` 方法使用 `Object.entries(request.labels).forEach(([key, value]) => {...})`
4. 模板字符串中使用 `[${key}]` 而非 `[key]` 字面量
5. 集成测试通过，golden 文件无意外 diff
