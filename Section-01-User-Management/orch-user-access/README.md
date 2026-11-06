# orch-user-access

REST gateway for registration, login and user operations. Coordinates the Core User and Core Authentication gRPC services.

## Configuration

Dependencies: both Core services. Settings: [config/config.yaml](config/config.yaml). Environment overrides YAML; `CONFIG_FILE` selects another file.

| Setting | Environment |
|---|---|
| HTTP listener | `HTTP_ADDRESS` |
| User RPC endpoint | `CORE_USER_GRPC_ADDRESS` |
| Authentication RPC endpoint | `CORE_AUTH_GRPC_ADDRESS` |

Compose supplies dependency addresses and local plaintext gRPC flags. Running `go run ./cmd` directly requires separately reachable Core endpoints; private Compose service names are resolved inside containers.

## Development

Requires Go 1.26+ and Make; `make test` uses the race detector. Run from this service directory:

```sh
make test
make vet
make build
```

`make run` starts the service directly with its configured dependencies. After code changes, rebuild only this service in the running Compose stack:

```sh
make rebuild
```

## Code and generation

Application logic: `internal/services`; models/ports: `internal/domain`; inbound RPC/HTTP: `internal/handlers`; configuration: `internal/appconfig`.

Regenerate local mocks after changing domain ports:

```sh
make mocks
```

After changing downstream RPC contracts, update their copies under `pkg/proto` and run `make generate` with a working `protoc`. Use `make -C .. generate` for all service copies. The Makefile installs the pinned Go plugins.

`pkg` is copied across the three services. After changing reusable code, synchronize the copies and run `make check` for shared-package and RPC-contract consistency. Run `make -C .. unit-test` for package/contract consistency and all service unit tests. The root `make test` runs the HTTP collections against the live stack.

Keep tests focused on important application logic and failure paths; avoid tests of thin wrappers or framework behavior.

## API integration

For interactive requests covering every REST endpoint, see [tests/http](tests/http/README.md). The `.http` files use httpyac named responses, request dependencies and native status assertions.

After starting the stack, run `make integration`. The target runs httpyac directly, then the Go integration tests in `tests/integration` for response validation, CRUD, ownership and concurrent registration/refresh. It uses temporary accounts and deletes them afterwards. Node.js/npm is required for httpyac; set `TEST_GATEWAY_URL` for the Go suite if using another URL.

Routes and DTOs live in `internal/handlers/http`; downstream calls live in `internal/adapter`. Update the [API reference](../docs/api.md) when changing public behavior.

[Run the full system](../README.md) · [Architecture](../docs/architecture.md) · [Package APIs](pkg/README.md)

From `pkg`, run `make gen-proto` to generate both domain bindings or `make runtest` to run shared-package race tests. From the service directory, use `make pkg-test` and `make clean` for package tests and local build/coverage cleanup.
