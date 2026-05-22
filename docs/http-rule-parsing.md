# HTTP Rule Parsing

Parses `google.api.http` annotation patterns (e.g., `/v1/{name=shippers/*}`) into structured URL templates. This is the foundation for generating correct HTTP method bindings and URL construction in the TypeScript client.

## Package: `internal/httprule`

### Core types

```go
// A parsed URL path template.
// Example: `/v1/{name=books/*}:publish`
type Template struct {
    Segments []Segment   // Path segments
    Verb     string      // Custom verb (e.g., "publish")
}

// A single path segment.
type Segment struct {
    Kind     SegmentKind      // Literal, MatchSingle, MatchMultiple, Variable
    Literal  string           // For literal segments
    Variable VariableSegment  // For variable segments
}

// A {field.path=sub-template} variable binding.
type VariableSegment struct {
    FieldPath FieldPath  // The protobuf field path (e.g., ["name"])
    Segments  []Segment  // The sub-template for matching (default: [*])
}
```

### Grammar

The parser implements the standard `google.api.http` path template grammar:

```
Template = "/" Segments [ Verb ]    // always starts with /
Segments = Segment { "/" Segment }
Segment  = "*"                      // match single path segment
         | "**"                     // match remaining path (last segment only)
         | LITERAL                  // exact match
         | Variable                 // field variable binding

Variable = "{" FieldPath [ "=" Segments ] "}"
FieldPath = IDENT { "." IDENT }
Verb     = ":" LITERAL
```

### Examples

| Pattern | Segments | Verb |
|---------|----------|------|
| `/v1/shippers` | `["v1", "shippers"]` (all literal) | — |
| `/v1/{name=shippers/*}` | `["v1", variable(name → [*])]` | — |
| `/v1/{parent=shippers/*}/sites` | `["v1", variable(parent → [*]), "sites"]` | — |
| `/v1/{string}:path` | `["v1", variable(string)]` | `"path"` |
| `/{name=shippers/*}/shipments/{shipment}` | `[variable(name → [*]), "shipments", variable(shipment)]` | — |

### Validation

After parsing, the validator checks:

1. **No nested variables**: `{a={b}}` is rejected
2. **`**` only as last segment**: `**/foo` is rejected
3. **No bare `*` or `**` at top level**: `*` and `**` must appear inside `{variable=**}`
4. **No duplicate variable bindings**: `{a}/{a}` is rejected

### API

```go
// Extract the HTTP annotation from a protobuf method descriptor.
rule, ok := httprule.Get(methodDescriptor)

// Parse the annotation into a structured Rule.
parsed, err := httprule.ParseRule(rule)
// parsed.Method     → "GET", "POST", etc.
// parsed.Template   → the URL Template
// parsed.Body       → the body field selector ("*", field name, or "")
// parsed.AdditionalRules → additional_bindings
```

### Relation to Code Generation

In `servicegen.go`, parsed rules drive three generation phases:

1. **Path validation** — generates `if (!request.field) throw new Error(...)` for path variables
2. **URL construction** — generates template literal strings using `request.field` values
3. **Body serialization** — generates `JSON.stringify()` calls based on the body selector
4. **Query parameters** — generates query string construction for all fields not covered by path or body
