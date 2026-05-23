# Protoc Gen TypeScript HTTP Refactor Design

## Background
The `protoc-gen-typescript-http` plugin generates TypeScript HTTP clients from Protocol Buffer definitions. However, it currently suffers from several missing edge-cases and bugs related to URL path construction, query parameter encoding, map/array type handling, and streaming RPC filtering.

## Objectives
Fix tasks 3-9 from the project issues:
- Task 3: Fix map query key/value encoding (`Object.entries()`).
- Task 4: Clarify and test repeated query formats (`field=value`).
- Task 5 & 6: Fix path variable encoding and strict sub-template semantics.
- Task 7: Skip streaming RPCs in client generation.
- Task 8: Prevent panics in `jsonPathSegments` when fields are invalid.
- Task 9: Support nested body fields (e.g., `body: "nested.field"`).

## Architecture & Refactor
We will completely refactor the generation code in `internal/plugin/servicegen.go` by adopting a builder approach for the Request building blocks:
- Path Builder
- Query Builder
- Body Builder

### 1. Path Variable Encoding (Task 5 & 6)
Instead of blindly doing `request.name`, we will apply URL component encoding. Crucially, we must distinguish between standard path variables and wildcard sub-templates:
- **Standard Variable (`{id}`):** Strictly URL encode the entire value (`encodeURIComponent(request.id)`).
- **Wildcard Sub-template (`{name=shippers/*}`):** Retain structural slashes by splitting, encoding segments, and joining.

Generated output logic will conditionally choose the encoding method in Go:
```typescript
// If the variable rule is a wildcard sub-template:
const path_name = request.name.split('/').map(p => encodeURIComponent(p)).join('/');
const path = `shippers/${path_name}`;

// If the variable rule is standard:
const path = `shippers/${encodeURIComponent(request.id)}`;
```

### 2. Query Parameters (Task 3 & 4)
We will maintain a deterministic and performant query string builder.
- **Repeated Fields (Task 4):** Standardized as `key=value1&key=value2` following gRPC-Gateway conventions.
  ```typescript
  request.tags.forEach((x) => queryParams.push(`tags=${encodeURIComponent(x.toString())}`));
  ```
- **Map Fields (Task 3):** Iterate through keys deterministically (sorted) and encode both key and value.
  ```typescript
  Object.keys(request.labels).sort().forEach((key) => {
    const value = request.labels[key];
    queryParams.push(`labels[${encodeURIComponent(key)}]=${encodeURIComponent(value.toString())}`);
  });
  ```
- **Performance:** `queryParams` will be a `string[]` that is `.join('&')` at the end to minimize allocation overhead.

### 3. Body Parsing (Task 9)
When `google.api.http` has `body: "nested.field"`, the path will be evaluated iteratively through the message structures to safely build `const body = JSON.stringify(request.nested.field);`. The string matching `path[0] == rule.Body` will be refactored to support deep paths.

### 4. Robustness & Error Handling (Task 7 & 8)
- **Streaming (Task 7):** Ensure `generateClient` uses `supportedMethod(method)` to completely skip `IsStreamingClient()` and `IsStreamingServer()`.
- **Panic Protection in Go (Task 8):** Refactor `jsonPathSegments` to return `([]string, error)`. If a proto field is not found (nil), it will return a detailed Go error (`fmt.Errorf("field %q not found in message %s", p, message.FullName())`). The generator will bubble this up and fail the compilation gracefully rather than crashing with a nil-pointer dereference.

## Conclusion
This refactor improves the TypeScript HTTP client generator to better support edge cases in Google AIP, ensuring safety, correctness, determinism, and generating idiomatic TypeScript code.
