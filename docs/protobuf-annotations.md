# Protobuf 注解

本插件可以从带有 Google HTTP 注解规则的 protobuf 文件中生成 TypeScript 代码。这篇指南将介绍如何正确地为 `.proto` 文件添加注解。

## 依赖项

在您的 proto 文件中添加所需的导入：

```protobuf
import "google/api/annotations.proto";
import "google/api/field_behavior.proto";
import "google/api/client.proto";
import "google/api/resource.proto";
```

## HTTP 注解 (`google.api.http`)

RPC 方法上的 `google.api.http` 注解决定了 URL 的构建、HTTP 方法的选择以及 body 的序列化方式。

### 基本模式

```protobuf
// GET request — path variable {name} maps to request.name
rpc GetShipper(GetShipperRequest) returns (Shipper) {
  option (google.api.http) = {get: "/v1/{name=shippers/*}"};
}

// POST with body field
rpc CreateShipper(CreateShipperRequest) returns (Shipper) {
  option (google.api.http) = {
    post: "/v1/shippers"
    body: "shipper"            // request.shipper is serialized as JSON body
  };
}

// PATCH with body
rpc UpdateShipper(UpdateShipperRequest) returns (Shipper) {
  option (google.api.http) = {
    patch: "/v1/{shipper.name=shippers/*}"
    body: "shipper"
  };
}

// DELETE
rpc DeleteShipper(DeleteShipperRequest) returns (Shipper) {
  option (google.api.http) = {delete: "/v1/{name=shippers/*}"};
}
```

### Body 绑定规则

| Body 值 | 效果 | 生成代码 |
|---|---|---|
| 未指定 | 无 body (GET/DELETE)，为 `null` | `const body = null;` |
| `"*"` | 整个请求消息都会被序列化为 JSON body | `const body = JSON.stringify(request);` |
| `"field_name"` | 仅将 `request.field_name` 序列化为 JSON body | `const body = JSON.stringify(request?.fieldName ?? {});` |
| `"nested.field"` | 嵌套字段序列化为 JSON body | `const body = JSON.stringify(request?.nested?.field ?? {});` |

### 路径变量

URL 路径中的变量用 `{field.path}` 表示：

```protobuf
rpc GetSite(GetSiteRequest) returns (Site) {
  option (google.api.http) = {get: "/v1/{name=shippers/*/sites/*}"};
}
```

本插件会：
1. 在发起请求前验证该字段是否非空
2. 使用 `request.name` 构建 URL（其中 proto 字段名会转换为 JSON 的 camelCase 驼峰命名）
3. 路径变量值会经过 `encodeURIComponent` 编码；对于包含子模板的变量（如 `{name=shippers/*}`），会保留语义性的斜杠

### 自定义动词

自定义动词通过冒号（`:`）追加到 URL 路径后：

```protobuf
rpc QueryOnly(Request) returns (Message) {
  option (google.api.http) = {get: "/v1:query"};
}
```

这会生成一个方法为 GET 且 URL 路径为 `v1:query` 的请求。

### 自定义 HTTP 方法 (`custom`)

当需要非标准 HTTP 方法时，使用 `custom` 字段：

```protobuf
rpc CustomMethod(Request) returns (Message) {
  option (google.api.http) = {
    custom: {
      kind: "FETCH"     // 自定义 HTTP 方法名
      path: "/v1/action"
    }
  };
}
```

生成的客户端会使用 `custom.kind` 中指定的字符串作为 HTTP 方法。

### 额外绑定

单个 RPC 可以使用 `additional_bindings` 来配置多个 HTTP 绑定：

```protobuf
rpc LegacyCreateShipper(CreateShipperRequest) returns (Shipper) {
  option (google.api.http) = {
    post: "/v1/shippers"
    body: "shipper"
    additional_bindings: {
      post: "/v1/shippers/create"
      body: "shipper"
    }
  };
}
```

> **当前行为**：自 2026-06-02 起，`google.api.http.additional_bindings` 中的额外绑定会按 first-match-wins 语义参与生成。每个 binding 编译为独立 TS 分支：第一个所有 path 变量都满足的 binding 被选中；无任何 binding 匹配时生成 `throw new Error("no matching binding for <Method>")` 兜底。历史行为（仅生成主绑定）在评审 `2026-06-02-golang-tool-project-review.md` 中标记为 P0 #1，已通过 spec/plan 闭环。

## 字段行为 (`google.api.field_behavior`)

用于声明字段的语义：

```protobuf
string name = 1 [(google.api.field_behavior) = REQUIRED];
google.protobuf.Timestamp create_time = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
```

这些行为会作为注释传播到生成的 TypeScript 代码中：

```typescript
// Behaviors: REQUIRED
name: string | undefined;

// Behaviors: OUTPUT_ONLY
createTime: wellKnownTimestamp | undefined;
```

> 注意：`REQUIRED` 字段当前生成 `: T | undefined` 类型而非 `?: T`（即非 optional），以符合 proto3 的 zero-value 语义。调用方仍需在运行时处理字段可能为 `undefined` 的情况。

## 资源注解 (`google.api.resource`)

用于声明资源模型：

```protobuf
message Shipper {
  option (google.api.resource) = {
    type: "freight-example.einride.tech/Shipper"
    pattern: "shippers/{shipper}"
    singular: "shipper"
    plural: "shippers"
  };
  string name = 1;
}
```

这些注解仅用于提供信息（不用于代码生成），但有助于保持 API 的一致性。

## 示例：完整服务

完整的注解服务示例请参阅 [examples/proto/einride/example/freight/v1/freight_service.proto](../examples/proto/einride/example/freight/v1/freight_service.proto)。

## 支持的 HTTP 方法

| 注解字段 | 生成的 HTTP 方法 |
|---|---|
| `get` | `GET` |
| `post` | `POST` |
| `put` | `PUT` |
| `patch` | `PATCH` |
| `delete` | `DELETE` |
| `custom` | `custom.kind` 中的值 |

## 不支持的 RPC 模式

本插件会跳过以下类型的方法：
- 客户端流式传输（请求中包含 `stream`）
- 服务端流式传输（响应中包含 `stream`）
- 缺少 `google.api.http` 注解
