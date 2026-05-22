# Architecture

## Overview

```
┌─────────────┐     stdin      ┌───────────────────┐     stdout      ┌──────────────┐
│   protoc    │ ──────────────▶│  protoc-gen-ts-http│ ──────────────▶│  index.ts    │
│   + buf     │  CodeGenerator  │                   │  CodeGenerator  │  (generated) │
│             │  Request        │     main.go       │  Response       │              │
└─────────────┘                 └───────────────────┘                 └──────────────┘
                                        │
                          ┌─────────────┼─────────────┐
                          │             │             │
                          ▼             ▼             ▼
                   ┌──────────┐  ┌──────────┐  ┌──────────┐
                   │  plugin  │  │ httprule │  │ codegen  │
                   │ (core)   │  │ (parser) │  │ (writer) │
                   └──────────┘  └──────────┘  └──────────┘
                          │                          ┌──────────┐
                          └─────────────────────────▶│ protowalk│
                                                     │ (walker) │
                                                     └──────────┘
```

## Entry point — `main.go`

The binary is a standard protoc plugin that communicates via protobuf serialized over stdin/stdout:

1. Reads a `CodeGeneratorRequest` from stdin
2. Delegates to `plugin.Generate()`
3. Writes the `CodeGeneratorResponse` to stdout

## Package: `internal/plugin`

The core generation engine. It:

- Builds a proto registry from the request's file descriptors
- Groups files by protobuf package
- For each package, generates a single `index.ts` output file containing:
  - TypeScript type definitions for all messages and enums
  - Service interfaces
  - A request handler type
  - Client factory functions with per-method HTTP routing logic

**Key files:**

| File | Purpose |
|------|---------|
| `generate.go` | Entry point — builds registry, groups files by package, orchestrates generation |
| `packagegen.go` | Package-level generator — walks descriptors and dispatches to specialized generators |
| `messagegen.go` | Generates TypeScript type aliases for protobuf messages |
| `enumgen.go` | Generates TypeScript union types for protobuf enums |
| `servicegen.go` | Generates service interfaces, request handler type, and client implementations |
| `commentgen.go` | Extracts and emits protobuf source comments and field behavior annotations |
| `type.go` | Maps protobuf field types to TypeScript types |
| `wellknown.go` | Handles Google Well-Known Types (Any, Duration, Timestamp, Struct, etc.) |
| `jsonleafwalk.go` | Walks message fields recursively to discover query-parameter candidates |
| `helpers.go` | Utility functions for type naming, field iteration, indentation |

## Package: `internal/httprule`

Parses `google.api.http` annotation patterns into structured URL templates.

**Key types:**

- `Template` — a parsed URL template with segments and optional verb
- `Segment` — a single path segment (literal, wildcard, variable)
- `VariableSegment` — a named variable with optional sub-segments

The parser implements the grammar defined in `google.api.http`:
```
Template = "/" Segments [ Verb ]
Segments = Segment { "/" Segment }
Segment  = "*" | "**" | LITERAL | Variable
Variable = "{" FieldPath [ "=" Segments ] "}"
FieldPath = IDENT { "." IDENT }
Verb     = ":" LITERAL
```

## Package: `internal/codegen`

A minimal code generation utility providing a buffered writer with a `P(v ...interface{})` method for emitting formatted lines.

## Package: `internal/protowalk`

A generic protobuf descriptor tree walker. It recursively visits files, messages, enums, services, and fields with cycle detection. Used by `packagegen.go` to traverse descriptors in the correct order.
