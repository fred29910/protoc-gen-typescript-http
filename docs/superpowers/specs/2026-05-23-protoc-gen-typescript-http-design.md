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
Instead of blindly doing `request.name`, we will apply URL component encoding while preserving structural slashes (needed for Google AIP sub-templates like `{name=shippers/*}`).
Generated output will resemble:
```typescript
const path_name = request.name.split('/').map(p => encodeURIComponent(p)).join('/');
const path = `shippers/${path_name}`;
```

### 2. Query Parameters (Task 3 & 4)
- **Repeated Fields (Task 4):** Standardized as `key=value1&key=value2`.
  ```typescript
  request.tags.forEach((x) => queryParams.push(`tags=${encodeURIComponent(x.toString())}`));
  ```
- **Map Fields (Task 3):** Encode both key and value.
  ```typescript
  Object.entries(request.labels).forEach(([key, value]) => {
    queryParams.push(`labels[${encodeURIComponent(key)}]=${encodeURIComponent(value.toString())}`);
  });
  ```

### 3. Body Parsing (Task 9)
When `google.api.http` has `body: "nested.field"`, the path will be evaluated iteratively through the message structures to safely build `const body = JSON.stringify(request.nested.field);`. The string matching `path[0] == rule.Body` will be refactored to support deep paths.

### 4. Robustness (Task 7 & 8)
- **Streaming (Task 7):** Ensure `generateClient` uses `supportedMethod(method)` to completely skip `IsStreamingClient()` and `IsStreamingServer()`.
- **Panic Protection (Task 8):** Refactor `jsonPathSegments` to return `([]string, error)`. If a field is `nil`, bubble up the error so the generator can fail gracefully instead of crashing with nil-pointer dereferences.

## Conclusion
This refactor improves the TypeScript HTTP client generator to better support edge cases in Google AIP, ensuring safety, correctness, and generating idiomatic TypeScript code.
