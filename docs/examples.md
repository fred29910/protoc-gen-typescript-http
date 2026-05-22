# Examples

## Overview

The example protos in `examples/proto/` demonstrate two API patterns:

1. **Freight service** — a full REST API following Google API Improvement Proposals (AIPs)
2. **Syntax coverage** — demonstrates all supported protobuf features and HTTP annotation patterns

## Freight Service (`einride/example/freight/v1/`)

A complete REST API for a freight transport system with full CRUD operations.

### Resource model

```
Shipper (shippers/{shipper})
├── Site (shippers/{shipper}/sites/{site})
└── Shipment (shippers/{shipper}/shipments/{shipment})
```

### Service: `FreightService`

| Method | HTTP binding | Description |
|--------|-------------|-------------|
| `GetShipper` | `GET /v1/{name=shippers/*}` | Get a shipper |
| `ListShippers` | `GET /v1/shippers` | List shippers (with pagination) |
| `CreateShipper` | `POST /v1/shippers` body: "shipper" | Create a shipper |
| `UpdateShipper` | `PATCH /v1/{shipper.name=shippers/*}` body: "shipper" | Update a shipper |
| `DeleteShipper` | `DELETE /v1/{name=shippers/*}` | Delete a shipper |
| `GetSite` | `GET /v1/{name=shippers/*/sites/*}` | Get a site |
| `ListSites` | `GET /v1/{parent=shippers/*}/sites` | List sites for a shipper |
| `CreateSite` | `POST /v1/{parent=shippers/*}/sites` body: "site" | Create a site |
| `UpdateSite` | `PATCH /v1/{site.name=shippers/*/sites/*}` body: "site" | Update a site |
| `DeleteSite` | `DELETE /v1/{name=shippers/*/sites/*}` | Delete a site |
| `GetShipment` | `GET /v1/{name=shippers/*/shipments/*}` | Get a shipment |
| `ListShipments` | `GET /v1/{parent=shippers/*}/shipments` | List shipments |
| `CreateShipment` | `POST /v1/{parent=shippers/*}/shipments` body: "shipment" | Create a shipment |
| `UpdateShipment` | `PATCH /v1/{shipment.name=shippers/*/shipments/*}` body: "shipment" | Update a shipment |
| `DeleteShipment` | `DELETE /v1/{name=shippers/*/shipments/*}` | Delete a shipment |

### Messages

- **Shipper** — name, create_time, update_time, delete_time, display_name
- **Site** — name, timestamps, display_name, lat_lng
- **Shipment** — name, timestamps, origin_site, destination_site, pickup/delivery times, line_items, annotations
- **LineItem** — title, quantity, weight_kg, volume_m3

### Generated client usage

```typescript
import { createFreightServiceClient } from "./einride/example/freight/v1";

// Create a handler using any HTTP client
const handler: RequestHandler = (request, meta) => {
  return fetch(`https://freight-example.einride.tech${request.path}`, {
    method: request.method,
    body: request.body ?? undefined,
  }).then((res) => res.json());
};

// Use the generated client
const client = createFreightServiceClient(handler);

// Typed API calls
const shipper = await client.GetShipper({ name: "shippers/abc123" });
const shippers = await client.ListShippers({ pageSize: 10 });
const newShipper = await client.CreateShipper({ shipper: { displayName: "Acme Corp" } });
```

## Syntax Coverage (`einride/example/syntax/v1/`)

Demonstrates all protobuf types and HTTP annotation variations.

### Message features

- **All scalar types**: double, float, int32, int64, uint32, uint64, sint32, sint64, fixed32, fixed64, sfixed32, sfixed64, bool, string, bytes
- **Optional fields**: every scalar type with `optional` keyword
- **Repeated fields**: every scalar type as `repeated`
- **Map fields**: `map<string, string>`, `map<string, Message>`
- **Oneof fields**: oneof string, enum, message
- **Well-Known Types**: Any, Duration, Empty, FieldMask, Struct, Value, NullValue, ListValue, wrapper types
- **Nested types**: nested messages and enums
- **Cross-package references**: `syntax.v2` references `syntax.v1` types

### Service: `SyntaxService`

| Method | Pattern | Body | Demonstrates |
|--------|---------|------|-------------|
| `QueryOnly` | `GET /v1` | — | Simple GET, all fields as query params |
| `EmptyVerb` | `GET /v1:emptyVerb` | — | Custom verb on empty request |
| `StarBody` | `POST /v1:starBody` | `*` | Whole message as body |
| `Body` | `POST /v1:body` | `nested` | Specific field as body |
| `Path` | `POST /v1/{string}:path` | — | Path variable |
| `PathBody` | `POST /v1/{string}:pathBody` | `nested` | Path variable + body field |

### Forward types (`einride/example/syntax/v2/`)

Demonstrates cross-package type references by reusing types from `syntax.v1`:

```protobuf
message Message {
  einride.example.syntax.v1.Message forwarded_message = 1;
  einride.example.syntax.v1.Enum forwarded_enum = 2;
}
```

## Generating examples

```bash
cd examples/proto
buf generate
```

This rebuilds all generated `.ts` files from the `.proto` sources. Requires the plugin binary in your `PATH`.
