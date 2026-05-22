protoc-gen-typescript-http
==========================

Generates Typescript types and service clients from protobuf definitions annotated with [http rules](https://github.com/googleapis/googleapis/blob/master/google/api/http.proto). The generated types follow the [canonical JSON encoding](https://developers.google.com/protocol-buffers/docs/proto3#json).

**Experimental**: This library is under active development and breaking changes to config files, APIs and generated code are expected between releases.

Using the plugin
----------------

For examples of correctly annotated protobuf defintions and the generated code, look at [examples](./examples).

### Install the plugin

```bash
go install github.com/go-kratos/protoc-gen-typescript-http@latest
```

Or download a prebuilt binary from [releases](./releases).

### Invocation

```bash
protoc 
  --typescript-http_out [OUTPUT DIR] \
  [.proto files ...]
```

---

The generated clients can be used with any HTTP client that returns a Promise containing JSON data.

```typescript
const rootUrl = "...";

type Request = {
  path: string,
  method: string,
  body: string | null
}

function fetchRequestHandler({path, method, body}: Request) {
  return fetch(rootUrl + path, {method, body}).then(response => response.json())
}

export function siteClient() {
  return createShipperServiceClient(fetchRequestHandler);
}

### Development

We use [Mage](https://github.com/magefile/mage) to manage project tasks.

#### Prerequisites

- Go 1.25.7+
- [buf](https://buf.build/docs/installation)
- [mage](https://magefile.org/) (optional, can also be run via `go run github.com/magefile/mage`)

#### Available Tasks

- `mage build`: Builds the plugin binary to the `bin/` directory.
- `mage test`: Runs unit tests.
- `mage integration`: Runs integration tests (builds plugin, generates code in `examples/`, and verifies there are no changes).
- `mage clean`: Cleans build artifacts.

You can also use the traditional `Makefile` as a shortcut:
- `make build`
- `make test`
- `make integration`
- `make clean`
```
