# Protobuf Annotations

This plugin generates TypeScript code from protobuf files annotated with Google's HTTP annotation rules. This guide explains how to properly annotate your `.proto` files.

## Dependencies

Add the required imports to your proto files:

```protobuf
import "google/api/annotations.proto";
import "google/api/field_behavior.proto";
import "google/api/client.proto";
import "google/api/resource.proto";
```

## HTTP Annotations (`google.api.http`)

The `google.api.http` annotation on RPC methods drives URL construction, HTTP method selection, and body serialization.

### Basic patterns

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

### Body binding rules

| Body value | Effect |
|---|---|
| Not specified | No body (GET/DELETE) — `null` |
| `"*"` | Entire request message is serialized as JSON body |
| `"field_name"` | Only `request.field_name` is serialized as JSON body |

### Path variables

Variables in the URL path are denoted by `{field.path}`:

```protobuf
rpc GetSite(GetSiteRequest) returns (Site) {
  option (google.api.http) = {get: "/v1/{name=shippers/*/sites/*}"};
}
```

The plugin:
1. Validates the field is non-null before making the request
2. Constructs the URL using `request.name` (with proto field names converted to JSON camelCase names)

### Custom verbs

```protobuf
rpc QueryOnly(Request) returns (Message) {
  option (google.api.http) = {get: "/v1:query"};
}
```

### Additional bindings

A single RPC can have multiple HTTP bindings:

```protobuf
rpc CreateShipper(CreateShipperRequest) returns (Shipper) {
  option (google.api.http) = {
    post: "/v1/shippers"
    body: "shipper"
  };
  option (google.api.method_signature) = "shipper";
}
```

Note: Additional bindings in `google.api.http.additional_bindings` are parsed and available but each method currently generates a single implementation path.

## Field Behaviors (`google.api.field_behavior`)

Document field semantics:

```protobuf
string name = 1 [(google.api.field_behavior) = REQUIRED];
google.protobuf.Timestamp create_time = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
```

These are propagated as comments in the generated TypeScript:

```typescript
// Behaviors: REQUIRED
name?: string;

// Behaviors: OUTPUT_ONLY
createTime?: string;
```

## Resource Annotations (`google.api.resource`)

Document the resource model:

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

These are informational (not used in code generation) but help with API consistency.

## Example: Full service

See [examples/proto/einride/example/freight/v1/freight_service.proto](../examples/proto/einride/example/freight/v1/freight_service.proto) for a complete annotated service.

## Supported HTTP methods

| Annotation | Generated method |
|---|---|
| `get` | GET |
| `post` | POST |
| `put` | PUT |
| `patch` | PATCH |
| `delete` | DELETE |
| `custom` | Custom method (from `custom.kind`) |

## Unsupported RPC patterns

The plugin skips methods that are:
- Client-streaming (`stream` on request)
- Server-streaming (`stream` on response)
- Missing `google.api.http` annotation
