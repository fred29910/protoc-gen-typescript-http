# Development

## Prerequisites

- **Go** 1.25.7+
- **[buf](https://buf.build/docs/installation)** — for building and linting example protos
- **[mage](https://magefile.org/)** — task runner (optional, can be run via `go run github.com/magefile/mage`)

## Project structure

```
.
├── main.go                    # Protoc plugin entry point
├── internal/
│   ├── codegen/               # Simple code generation buffer
│   │   └── file.go
│   ├── httprule/              # HTTP annotation parser
│   │   ├── rule.go
│   │   ├── template.go
│   │   ├── fieldpath.go
│   │   └── template_test.go
│   ├── plugin/                # Core generation engine
│   │   ├── generate.go
│   │   ├── packagegen.go
│   │   ├── messagegen.go
│   │   ├── enumgen.go
│   │   ├── servicegen.go
│   │   ├── commentgen.go
│   │   ├── type.go
│   │   ├── wellknown.go
│   │   ├── jsonleafwalk.go
│   │   └── helpers.go
│   └── protowalk/             # Protobuf descriptor walker
│       └── walk.go
├── tests/
│   └── integration/           # Integration tests (build tag: integration)
├── examples/
│   └── proto/                 # Example proto definitions and generated code
├── magefile.go                # Mage task definitions
└── Makefile                   # Optional Makefile wrapper
```

## Available tasks

All project tasks are managed by [Mage](https://magefile.org/):

| Command | Description |
|---|---|
| `mage build` | Build the plugin binary to `bin/protoc-gen-typescript-http` |
| `mage test` | Run unit tests |
| `mage integration` | Run integration tests (full build + code generation + diff verification) |
| `mage clean` | Clean build artifacts |

Or using the Makefile wrapper:

```bash
make build
make test
make integration
make clean
```

## Workflow

### Building

```bash
mage build
```

The binary is output to `bin/protoc-gen-typescript-http`.

### Running unit tests

```bash
mage test
```

### Running integration tests

Integration tests:
1. Build the plugin binary
2. Run `buf generate` in `examples/proto/` using the built plugin
3. Verify generated code matches committed code via `git diff --exit-code`

```bash
mage integration
```

### Using the plugin directly

```bash
protoc --typescript-http_out=./output --proto_path=./protos ./protos/*.proto
```

Or with buf:

```bash
cd examples/proto
buf generate
```

### Adding new examples

1. Add your `.proto` files under `examples/proto/`
2. Generate TypeScript code: `buf generate` (from `examples/proto/`)
3. Commit both `.proto` and generated `.ts` files

## Code conventions

- Standard Go formatting (`gofmt`)
- Protobuf follows [Google AIP](https://google.aip.dev/) conventions in examples
- Generated TypeScript uses `@ts-nocheck` and `eslint-disable camelcase` to avoid warnings on proto-idiomatic naming
- Error handling uses `fmt.Errorf("context: %w", err)` for proper error wrapping

## Release process

Releases are automated via [GoReleaser](https://goreleaser.com/) configuration in `.goreleaser.yml`:

```bash
goreleaser release
```

Builds for linux, windows, and darwin (amd64) with CGO disabled.
