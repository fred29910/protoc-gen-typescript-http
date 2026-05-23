# Protoc Gen TypeScript HTTP Refactor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix 7 known issues (tasks 3-9) in the `protoc-gen-typescript-http` Go plugin that generates TypeScript HTTP clients from protobuf definitions.

**Architecture:** Completely refactor `internal/plugin/servicegen.go` to properly handle URL path encoding (preserving slashes for wildcard sub-templates, full encoding for standard vars), query parameter encoding (map key/value both encoded, deterministic sort), nested body field extraction, streaming RPC exclusion, and add nil-safety to `jsonPathSegments`.

**Tech Stack:** Go 1.21+, protobuf reflect (protoreflect), protoc plugin framework, TypeScript output targeting ES2020+.

---

### Task 1: Fix `jsonPathSegments` nil-safety (Task 8)

**Files:**
- Modify: `internal/plugin/servicegen.go:247-257`
- Modify: `internal/plugin/servicegen.go:240-245` (callers)
- Test: No direct unit test — covered by integration test + manual proto compilation

**Step 1: Change `jsonPathSegments` signature to return `([]string, error)`**

Current signature:
```go
func jsonPathSegments(path httprule.FieldPath, message protoreflect.MessageDescriptor) []string {
```

New signature:
```go
func jsonPathSegments(path httprule.FieldPath, message protoreflect.MessageDescriptor) ([]string, error) {
```

Implementation with nil check:
```go
func jsonPathSegments(path httprule.FieldPath, message protoreflect.MessageDescriptor) ([]string, error) {
    segs := make([]string, len(path))
    for i, p := range path {
        field := message.Fields().ByName(protoreflect.Name(p))
        if field == nil {
            return nil, fmt.Errorf("field %q not found in message %s", p, message.FullName())
        }
        segs[i] = field.JSONName()
        if i < len(path) {
            message = field.Message()
        }
    }
    return segs, nil
}
```

**Step 2: Update `jsonPath` to propagate error**

```go
func jsonPath(path httprule.FieldPath, message protoreflect.MessageDescriptor) (string, error) {
    segs, err := jsonPathSegments(path, message)
    if err != nil {
        return "", err
    }
    return strings.Join(segs, "."), nil
}
```

**Step 3: Update `nullPropagationPath` to propagate error**

```go
func nullPropagationPath(path httprule.FieldPath, message protoreflect.MessageDescriptor) (string, error) {
    segs, err := jsonPathSegments(path, message)
    if err != nil {
        return "", err
    }
    return strings.Join(segs, "?."), nil
}
```

**Step 4: Update all callers to handle errors**

Callers that need updating:
- `generateMethodPathValidation` (line 123): `nullPropagationPath(fp, input)` → return error if err
- `generateMethodPath` (line 141): `jsonPath(seg.Variable.FieldPath, input)` → propagate error
- `generateMethodBody` (line 170): `nullPropagationPath(httprule.FieldPath{rule.Body}, input)` → propagate error (this will also be fixed for nested body in Task 5)
- `generateMethodQuery` (line 200): `jsonPath(path, input)` → need new approach since it's in a callback
- `queryPresenceExpr` (line 231): `nullPropagationPath(path, message)` → return error

Since many of these are used in `walkJSONLeafFields` callback which doesn't return error, we need a pragmatic approach:
- Add a `syncErr` field or return accumulated errors through the existing `methodErr` mechanism
- Best approach: Change `generateMethodQuery` to build errors and return from `generateMethod` instead

**Step 5: Run integration test to verify it builds**

Run: `go build ./...` then `go test ./...`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/plugin/servicegen.go
git commit -m "fix: add nil-safety to jsonPathSegments with error propagation"
```

---

### Task 2: Skip streaming RPCs in client generation (Task 7)

**Files:**
- Modify: `internal/plugin/servicegen.go:82-87`

**Problem:** `generateMethod` (line 82) only checks `httprule.Get(method)` — it doesn't check `supportedMethod(method)` which also checks `!method.IsStreamingClient() && !method.IsStreamingServer()`. So streaming methods with HTTP annotations get generated as client methods, but the interface drops them.

**Fix:** Change the guard in `generateMethod`:

Current:
```go
r, ok := httprule.Get(method)
if !ok {
    return nil
}
```

Replace with:
```go
if !supportedMethod(method) {
    return nil
}
r, ok := httprule.Get(method)
if !ok {
    return nil
}
```

Note: `supportedMethod` already checks `httprule.Get(method)`, so we could simplify to just `if !supportedMethod(method) { return nil }` — but keeping the current structure is clearer.

**Step 1: Apply the fix**

**Step 2: Verify with `go build ./...`**

Expected: PASS

**Step 3: Commit**

```bash
git add internal/plugin/servicegen.go
git commit -m "fix: skip streaming RPCs in generateClient like generateInterface does"
```

---

### Task 3: Fix map query key/value encoding (Task 3)

**Files:**
- Modify: `internal/plugin/servicegen.go:207-210`

**Problem:** Map query parameters currently encode only the value, not the key:
```typescript
Object.entries(request.labels).forEach(([key, value]) => {
    queryParams.push(`labels[${key}]=${encodeURIComponent(value.toString())}`)
})
```

**Fix:** Encode both key and value, and sort keys for determinism:

```typescript
Object.keys(request.labels).sort().forEach((key) => {
    const value = request.labels[key];
    queryParams.push(`labels[${encodeURIComponent(key)}]=${encodeURIComponent(value.toString())}`)
})
```

**Step 1: Update the Go template code generation**

Change line 208-210 in `servicegen.go` from:
```go
case field.IsMap():
    f.P(t(4), "Object.entries(request.", jp, ").forEach(([key, value]) => {")
    f.P(t(5), "queryParams.push(`", jp, "[${key}]=${encodeURIComponent(value.toString())}`)")
    f.P(t(4), "})")
```

To:
```go
case field.IsMap():
    f.P(t(4), "Object.keys(request.", jp, ").sort().forEach((key) => {")
    f.P(t(5), "const value = request.", jp, "[key];")
    f.P(t(5), "queryParams.push(`", jp, "[${encodeURIComponent(key)}]=${encodeURIComponent(value.toString())}`)")
    f.P(t(4), "})")
```

**Step 2: Regenerate example output**

Run: `go build -o bin/protoc-gen-typescript-http . && cd examples/proto && buf generate && cd ../..`
Expected: Generated TS changes to use `Object.keys().sort()` + encodeURIComponent on key

**Step 3: Verify integration test**

Run: `cd tests/integration && go test -tags=integration -v . && cd ../..`
Or if that requires too much: `go build ./...` 
Expected: PASS

**Step 4: Commit**

```bash
git add internal/plugin/servicegen.go
git commit -m "fix: encode map query key/value, sort keys for determinism"
```

---

### Task 4: Path variable encoding (Tasks 5 & 6)

**Files:**
- Modify: `internal/plugin/servicegen.go:132-156` (`generateMethodPath`)
- Potentially: `internal/httprule/template.go` (understanding segment types)

**Problem:** Path variables are interpolated directly into the template string:
```typescript
const path = `v1/${request.name}`;
```
This breaks when `name` contains special URL characters like spaces, `?`, `#`, etc.

For Google AIP sub-templates like `{name=shippers/*}`, the slashes must be preserved (the variable value is a structured resource name). For standard variables like `{id}`, everything should be encoded.

**Detection:** A variable is a wildcard sub-template IF `seg.Variable.Segments` has more than a single `SegmentKindMatchSingle`. Specifically:
- `{id}` → `Segments: [{Kind: SegmentKindMatchSingle}]` → standard (full encodeURIComponent)
- `{name=shippers/*}` → `Segments: [{Kind: SegmentKindLiteral, Literal: "shippers"}, {Kind: SegmentKindMatchSingle}]` → wildcard (split/encode/join)
- `{name=**}` → `Segments: [{Kind: SegmentKindMatchMultiple}]` → wildcard (split/encode/join)

**Step 1: Add helper to detect wildcard variable**

In `servicegen.go`, add:
```go
// isWildcardVariable returns true if the variable segment uses a sub-template
// pattern (e.g., {name=shippers/*} or {name=**}), which means slashes within
// the value are semantically significant and must be preserved.
func isWildcardVariable(seg httprule.Segment) bool {
    if seg.Kind != httprule.SegmentKindVariable {
        return false
    }
    // If the variable has more than just a single * segment, it's a wildcard
    if len(seg.Variable.Segments) == 1 && seg.Variable.Segments[0].Kind == httprule.SegmentKindMatchSingle {
        return false // simple {id} — no sub-template
    }
    return true // has sub-template like {name=shippers/*} or {name=**}
}
```

**Step 2: Modify `generateMethodPath` to apply encoding**

In the `SegmentKindVariable` case in `generateMethodPath`:

Current:
```go
case httprule.SegmentKindVariable:
    fieldPath := jsonPath(seg.Variable.FieldPath, input)
    pathParts = append(pathParts, "${request."+fieldPath+"}")
```

New: (with error propagation, assuming `jsonPath` returns error):
```go
case httprule.SegmentKindVariable:
    fieldPath, err := jsonPath(seg.Variable.FieldPath, input)
    if err != nil {
        return err
    }
    if isWildcardVariable(seg) {
        // Preserve structural slashes — split, encode each segment, join
        // e.g., request.name.split('/').map(p => encodeURIComponent(p)).join('/')
        pathParts = append(pathParts,
            "${request."+fieldPath+".split('/').map(p => encodeURIComponent(p)).join('/')}")
    } else {
        // Standard variable — full encodeURIComponent
        pathParts = append(pathParts,
            "${encodeURIComponent(request."+fieldPath+")}")
    }
```

**Step 3: Update `generateMethodPath` return type**

Change `generateMethodPath` to return `error`:
```go
func (s serviceGenerator) generateMethodPath(
    f *codegen.File,
    input protoreflect.MessageDescriptor,
    rule httprule.Rule,
) error {
```

And update the call site in `generateMethod` to handle the error.

**Step 4: Regenerate example output**

Run: `go build ./...`
Expected: PASS. Generated output for methods like `GetShipper` (uses `v1/{name}` where `name` is `shippers/{shipper}`) will change from `${request.name}` to `${encodeURIComponent(request.name)}` since `{name}` is simple.

Note: For the existing example proto, `{name}` is a plain variable (no sub-template), so it will get `encodeURIComponent`. The wildcard case will be tested by adding a new proto in future.

**Step 5: Commit**

```bash
git add internal/plugin/servicegen.go
git commit -m "fix: encode path variables, preserve slashes for wildcard sub-templates"
```

---

### Task 5: Support nested body fields (Task 9)

**Files:**
- Modify: `internal/plugin/servicegen.go:163-171` (`generateMethodBody`)
- Modify: `internal/plugin/servicegen.go:190-198` (`generateMethodQuery` body-skip check)

**Problem 1 — body generation (line 170):**
```go
nullPath := nullPropagationPath(httprule.FieldPath{rule.Body}, input)
```
When `rule.Body` is `"nested.field"`, this creates a 1-element FieldPath `["nested.field"]` — it should be `["nested", "field"]`.

**Fix:** Split `rule.Body` by `.` to create the FieldPath correctly:
```go
// Support nested body: body: "nested.field" → access request.nested.field
var bodyPath httprule.FieldPath
if rule.Body != "" && rule.Body != "*" {
    bodyPath = strings.Split(rule.Body, ".")
}
```

Then use:
```go
nullPath, err := nullPropagationPath(bodyPath, input)
```

**Step 1: Fix body expression in `generateMethodBody`**

```go
func (s serviceGenerator) generateMethodBody(
    f *codegen.File,
    input protoreflect.MessageDescriptor,
    rule httprule.Rule,
) error {
    switch {
    case rule.Body == "":
        f.P(t(3), "const body = null;")
    case rule.Body == "*":
        f.P(t(3), "const body = JSON.stringify(request);")
    default:
        bodyPath := httprule.FieldPath(strings.Split(rule.Body, "."))
        nullPath, err := nullPropagationPath(bodyPath, input)
        if err != nil {
            return err
        }
        f.P(t(3), "const body = JSON.stringify(request?.", nullPath, " ?? {});")
    }
    return nil
}
```

**Step 2: Fix body skip check in `generateMethodQuery`**

Current:
```go
if rule.Body != "" && path[0] == rule.Body {
    return
}
```

Problem: `rule.Body` is `"nested.field"` but `path[0]` is `"nested"` → never skips.

Fix: Split into segments and compare:
```go
if rule.Body != "" && rule.Body != "*" {
    bodySegments := strings.Split(rule.Body, ".")
    if pathStartsWith(path, bodySegments) {
        return
    }
}
```

Add helper:
```go
// pathStartsWith returns true if path starts with the given prefix segments.
func pathStartsWith(path httprule.FieldPath, prefix []string) bool {
    if len(path) < len(prefix) {
        return false
    }
    for i, seg := range prefix {
        if string(path[i]) != seg {
            return false
        }
    }
    return true
}
```

**Step 3: Update `generateMethodQuery` return type**

Add `error` return and propagate the `jsonPath` error.

**Step 4: Update `generateMethod` to chain errors**

`generateMethod` calls `generateMethodBody` and `generateMethodQuery` — add error handling.

**Step 5: Verify with `go build ./...`**

Expected: PASS

**Step 6: Commit**

```bash
git add internal/plugin/servicegen.go
git commit -m "feat: support nested body fields (body: 'nested.field')"
```

---

### Task 6: Regenerate golden files and fix integration test

**Files:**
- Modify: `examples/proto/gen/typescript/einride/example/freight/v1/index.ts`
- Modify: `examples/proto/gen/typescript/einride/example/syntax/v1/index.ts`
- Modify: `examples/proto/gen/typescript/einride/example/syntax/v2/index.ts`

**Step 1: Build and regenerate**

```bash
go build -o bin/protoc-gen-typescript-http .
cd examples/proto && buf generate && cd ../..
```

**Step 2: Check git diff to verify generated changes**

Run: `git diff examples/proto/gen/typescript/`
Expected: Changes reflecting:
- `encodeURIComponent()` added around simple path variables
- Map queries use `Object.keys().sort()` + both key/value encoded
- Repeated queries unchanged (already correct `field=value` format)

**Step 3: Commit generated files**

```bash
git add examples/proto/gen/typescript/
git add -A
git commit -m "chore: regenerate golden files after refactor"
```

---

### Task 7: Add unit tests for path and query generation

**Files:**
- Create: `internal/plugin/servicegen_test.go`
- Modify: `tests/integration/integration_test.go` (if needed)

**Step 1: Create unit test file**

Create `internal/plugin/servicegen_test.go` with tests for:
- `isWildcardVariable` — verify detection of sub-template variables
- `jsonPathSegments` — verify nil handling returns error
- `jsonPath` — verify correct JSON path construction
- `pathStartsWith` — verify nested body path prefix matching

```go
package plugin

import (
    "testing"
    
    "github.com/go-kratos/protoc-gen-typescript-http/internal/httprule"
    "gotest.tools/v3/assert"
)

func TestIsWildcardVariable(t *testing.T) {
    tests := []struct {
        seg      httprule.Segment
        expected bool
    }{
        { // {id} — simple variable, no wildcard
            Kind: httprule.SegmentKindVariable,
            Variable: httprule.VariableSegment{
                FieldPath: []string{"id"},
                Segments:  []httprule.Segment{{Kind: httprule.SegmentKindMatchSingle}},
            },
            expected: false,
        },
        { // {name=shippers/*} — sub-template, wildcard
            Kind: httprule.SegmentKindVariable,
            Variable: httprule.VariableSegment{
                FieldPath: []string{"name"},
                Segments: []httprule.Segment{
                    {Kind: httprule.SegmentKindLiteral, Literal: "shippers"},
                    {Kind: httprule.SegmentKindMatchSingle},
                },
            },
            expected: true,
        },
        { // {name=**} — catch-all, wildcard
            Kind: httprule.SegmentKindVariable,
            Variable: httprule.VariableSegment{
                FieldPath: []string{"name"},
                Segments:  []httprule.Segment{{Kind: httprule.SegmentKindMatchMultiple}},
            },
            expected: true,
        },
        { // literal segment → not a variable
            Kind:    httprule.SegmentKindLiteral,
            Literal: "v1",
            expected: false,
        },
    }
    for _, tt := range tests {
        assert.Equal(t, tt.expected, isWildcardVariable(tt.seg))
    }
}

func TestPathStartsWith(t *testing.T) {
    assert.Assert(t, pathStartsWith(
        httprule.FieldPath{"nested", "field", "sub"},
        []string{"nested", "field"},
    ))
    assert.Assert(t, !pathStartsWith(
        httprule.FieldPath{"other", "field"},
        []string{"nested", "field"},
    ))
    assert.Assert(t, !pathStartsWith(
        httprule.FieldPath{"nested"},
        []string{"nested", "field"},
    ))
    assert.Assert(t, pathStartsWith(
        httprule.FieldPath{"nested"},
        []string{"nested"},
    ))
}
```

**Step 2: Run unit tests**

Run: `go test ./internal/plugin/ -v`
Expected: PASS (all new tests pass + no existing tests fail)

**Step 3: Commit**

```bash
git add internal/plugin/servicegen_test.go
git commit -m "test: add unit tests for wildcard variable detection and path prefix matching"
```

---

## Summary of Changes

| File | Change |
|---|---|
| `internal/plugin/servicegen.go` | Refactor `jsonPathSegments` → return error |
| `internal/plugin/servicegen.go` | Add `isWildcardVariable()` helper |
| `internal/plugin/servicegen.go` | Add `pathStartsWith()` helper |
| `internal/plugin/servicegen.go` | Add path encoding (encodeURIComponent + split/join) |
| `internal/plugin/servicegen.go` | Map query: encode key+value, sort keys |
| `internal/plugin/servicegen.go` | Streaming: use `supportedMethod()` in client |
| `internal/plugin/servicegen.go` | Nested body: split on `.`, use correct FieldPath |
| `internal/plugin/servicegen.go` | Propagate errors through all generators |
| `internal/plugin/servicegen_test.go` | Unit tests for new helpers |
| `examples/proto/gen/typescript/.../*.ts` | Regenerated golden files |
