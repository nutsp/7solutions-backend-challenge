# core-authentication

gRPC service for credential verification, JWT issuance and Redis sessions. Reads credentials from Core User over RPC.

## Configuration

Dependencies: Core User and Redis. Settings: [config/config.yaml](config/config.yaml). Environment overrides YAML; `CONFIG_FILE` selects another file.

| Setting | Environment |
|---|---|
| RPC listener | `GRPC_SERVER_ADDRESS` |
| User RPC endpoint | `CORE_USER_GRPC_ADDRESS` |
| Redis connection | `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD` |
| JWT signing and lifetime | `JWT_SECRET`, `ACCESS_TOKEN_SECONDS` |

Match Redis credentials to the Redis server. Server, health-client and downstream-client plaintext flags are independent; Compose sets them for local use. Set TLS fields in the YAML for other environments.

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

Owns `pkg/proto/core-authentication/authentication-service.proto`. Update copies in all three modules and run `make generate` to rebuild this module’s bindings. Use `make -C .. generate` to rebuild all service copies. A working `protoc` is required. From `pkg`, use `make gen-proto` and `make runtest`; service-level `make generate` delegates to that Makefile.

`pkg` is copied across the three services. After changing reusable code, synchronize the copies and run `make check` for shared-package and RPC-contract consistency. Run `make -C .. unit-test` for package/contract consistency and all service unit tests. The root `make test` runs the HTTP collections against the live stack.

Keep tests focused on important application logic and failure paths; avoid tests of thin wrappers or framework behavior.

## Important logic

Authentication logic lives in `internal/services`; Redis session operations and atomic refresh rotation live in `internal/repository/redis`. Tests focus on credential failures, expired/revoked sessions and refresh behavior. Keep plaintext passwords and tokens out of logs.

[Run the full system](../README.md) · [Architecture](../docs/architecture.md) · [Package APIs](pkg/README.md)

Use `make pkg-test` for shared-package race tests and `make clean` to remove local build/coverage artifacts. A full local reset is documented in the [setup guide](../docs/getting-started.md#reset-local-state).
