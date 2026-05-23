# 示例

## 概述

`examples/proto/` 中的示例 proto 演示了两种 API 模式：

1. **Freight 服务**，一个遵循 Google API 改进提案（AIP）的完整 REST API
2. **语法覆盖**，演示了所有支持的 protobuf 特性与 HTTP 注解模式

## Freight 服务 (`einride/example/freight/v1/`)

一个用于货运系统的完整 REST API，支持完整的 CRUD 操作。

### 资源模型

```
Shipper (shippers/{shipper})
├── Site (shippers/{shipper}/sites/{site})
└── Shipment (shippers/{shipper}/shipments/{shipment})
```

### 服务：`FreightService`

| 方法 | HTTP 绑定 | 描述 |
|--------|-------------|-------------|
| `GetShipper` | `GET /v1/{name=shippers/*}` | 获取 Shipper |
| `ListShippers` | `GET /v1/shippers` | 列出 Shipper（支持分页） |
| `CreateShipper` | `POST /v1/shippers` body: "shipper" | 创建 Shipper |
| `UpdateShipper` | `PATCH /v1/{shipper.name=shippers/*}` body: "shipper" | 更新 Shipper |
| `DeleteShipper` | `DELETE /v1/{name=shippers/*}` | 删除 Shipper |
| `GetSite` | `GET /v1/{name=shippers/*/sites/*}` | 获取 Site |
| `ListSites` | `GET /v1/{parent=shippers/*}/sites` | 列出 Shipper 的 Site |
| `CreateSite` | `POST /v1/{parent=shippers/*}/sites` body: "site" | 创建 Site |
| `UpdateSite` | `PATCH /v1/{site.name=shippers/*/sites/*}` body: "site" | 更新 Site |
| `DeleteSite` | `DELETE /v1/{name=shippers/*/sites/*}` | 删除 Site |
| `GetShipment` | `GET /v1/{name=shippers/*/shipments/*}` | 获取 Shipment |
| `ListShipments` | `GET /v1/{parent=shippers/*}/shipments` | 列出 Shipment |
| `CreateShipment` | `POST /v1/{parent=shippers/*}/shipments` body: "shipment" | 创建 Shipment |
| `UpdateShipment` | `PATCH /v1/{shipment.name=shippers/*/shipments/*}` body: "shipment" | 更新 Shipment |
| `DeleteShipment` | `DELETE /v1/{name=shippers/*/shipments/*}` | 删除 Shipment |

### 消息

- **Shipper**：name、create_time、update_time、delete_time、display_name
- **Site**：name、时间戳、display_name、lat_lng
- **Shipment**：name、时间戳、origin_site、destination_site、取货/送货时间、line_items、annotations
- **LineItem**：title、quantity、weight_kg、volume_m3

### 生成的客户端用法

> **注意**：`RequestHandler` 和 `RequestType` 是生成代码中**未导出的类型**，无法直接 import。以下示例展示了如何在调用方自行声明等效类型。

```typescript
import { createFreightServiceClient } from "./einride/example/freight/v1/index";
import type { FreightService } from "./einride/example/freight/v1/index";

// 自行声明 RequestHandler 类型（当前生成代码不导出该类型）
type RequestType = {
  path: string;
  method: string;
  body: string | null;
};
type RequestHandler = (request: RequestType, meta: { service: string, method: string }) => Promise<unknown>;

const rootUrl = "https://freight-example.einride.tech";

// 将 fetch 适配为 RequestHandler 接口
const handler: RequestHandler = (request, meta) => {
  const url = `${rootUrl}/${request.path}`;
  return fetch(url, {
    method: request.method,
    headers: request.body ? { "Content-Type": "application/json" } : undefined,
    body: request.body ?? undefined,
  }).then((res) => res.json());
};

// 创建类型安全的 client
const client: FreightService = createFreightServiceClient(handler);

// Typed API calls — full autocompletion and type safety
const shipper = await client.GetShipper({ name: "shippers/abc123" });
const shippers = await client.ListShippers({ pageSize: 10 });
const newShipper = await client.CreateShipper({
  shipper: { displayName: "Acme Corp" },
});
```

## 语法覆盖 (`einride/example/syntax/v1/`)

演示了所有 protobuf 类型和 HTTP 注解变体。

### 消息特性

- **所有标量类型**：double、float、int32、int64、uint32、uint64、sint32、sint64、fixed32、fixed64、sfixed32、sfixed64、bool、string、bytes
- **可选字段**：带有 `optional` 关键字的每个标量类型
- **重复字段**：作为 `repeated` 的每个标量类型
- **Map 字段**：`map<string, string>`、`map<string, Message>`
- **Oneof 字段**：oneof string、enum、message
- **常用类型（Well-Known Types）**：Any、Duration、Empty、FieldMask、Struct、Value、NullValue、ListValue、包装类型
- **嵌套类型**：嵌套消息和枚举
- **跨包引用**：`syntax.v2` 引用 `syntax.v1` 类型

### 服务：`SyntaxService`

| 方法 | 模式 | 请求体 | 演示内容 |
|--------|---------|------|-------------|
| `QueryOnly` | `GET /v1` | — | 简单的 GET 请求，所有字段均作为查询参数 |
| `EmptyVerb` | `GET /v1:emptyVerb` | — | 空请求上的自定义动词 |
| `StarBody` | `POST /v1:starBody` | `*` | 整个消息作为请求体 |
| `Body` | `POST /v1:body` | `nested` | 特定字段作为请求体 |
| `Path` | `POST /v1/{string}:path` | — | 路径变量 |
| `PathBody` | `POST /v1/{string}:pathBody` | `nested` | 路径变量 + 请求体字段 |

### 转发类型 (`einride/example/syntax/v2/`)

通过重用 `syntax.v1` 中的类型来演示跨包类型引用。生成的 TypeScript 类型名会包含扁平化的包名前缀以避免冲突：

```protobuf
message Message {
  einride.example.syntax.v1.Message forwarded_message = 1;
  einride.example.syntax.v1.Enum forwarded_enum = 2;
}
```

生成代码中，跨包类型会被引用为：

```typescript
export type Message = {
  forwardedMessage: einrideexamplesyntaxv1_Message | undefined;
  forwardedEnum: einrideexamplesyntaxv1_Enum | undefined;
};
```

## 生成示例

```bash
cd examples/proto
buf generate
```

这将从 `.proto` 源文件重新构建所有生成的 `.ts` 文件。这需要将插件二进制文件添加到您的 `PATH` 中。
