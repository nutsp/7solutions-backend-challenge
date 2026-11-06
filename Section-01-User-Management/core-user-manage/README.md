# core-user-manage

gRPC service for User CRUD, encrypted MongoDB records, transactional audit and the user-count worker.

## Configuration

Dependency: MongoDB replica set. Prepare collections and indexes with the separate `ensure-index` tool before starting the service. Settings: [config/config.yaml](config/config.yaml). Environment overrides YAML; `CONFIG_FILE` selects another file.

| Setting | Environment |
|---|---|
| RPC listener | `GRPC_SERVER_ADDRESS` |
| Database connection | `MONGO_URI`, `MONGO_DATABASE`, `MONGO_USERNAME`, `MONGO_PASSWORD`, `MONGO_AUTH_SOURCE` |
| Encryption keyrings | `USER_DATA_KEYS`, `USER_AUDIT_KEYS` |
| Email lookup key | `EMAIL_LOOKUP_KEY` |

Keyrings are JSON maps of key IDs to base64-encoded 32-byte keys. The lookup key is an independent base64-encoded 32-byte key. Keep existing key IDs/bytes and the lookup key when reusing data. Compose supplies runtime credentials; administrative credentials belong to `ensure-index`.

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

Owns `pkg/proto/core-user-manage/user-service.proto`. Update copies in all three modules and run `make generate` to rebuild this module’s bindings. Use `make -C .. generate` to rebuild all service copies. A working `protoc` is required. From `pkg`, use `make gen-proto` and `make runtest`; service-level `make generate` delegates to that Makefile.

`pkg` is copied across the three services. After changing reusable code, synchronize the copies and run `make check` for shared-package and RPC-contract consistency. Run `make -C .. unit-test` for package/contract consistency and all service unit tests. The root `make test` runs the HTTP collections against the live stack.

Keep tests focused on important application logic and failure paths; avoid tests of thin wrappers or framework behavior.

## Storage and integration

Repositories and encrypted document mapping live in `internal/repository/mongodb`. User writes and audit inserts share one transaction. A normalized no-op update preserves the timestamp and creates no audit event. Passwords remain bcrypt hashes.

After starting the stack, run `make integration` (or `make integration COMPOSE_PROJECT_NAME=your-project` for another Compose project). It builds the external `ensure-index` command and checks unique indexes, encrypted storage, audit rollback and concurrent updates in temporary databases.

For key rotation, keep old keys for retained ciphertext and select a new active key ID in config. Do not change the email lookup key without separate rotation tooling.

[Run the full system](../README.md) · [Architecture](../docs/architecture.md) · [Package APIs](pkg/README.md)

Use `make pkg-test` for shared-package race tests and `make clean` to remove local build/coverage artifacts. A full local reset is documented in the [setup guide](../docs/getting-started.md#reset-local-state).
