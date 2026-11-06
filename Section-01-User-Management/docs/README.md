# Operations and overview

[Project README](../README.md) contains startup and a working API example. Commands run from the repository root unless stated otherwise.

- [Setup and troubleshooting](getting-started.md)
- [API specification and sequence diagrams](api.md)
- [Service architecture and dependency direction](architecture.md)
- [Logs, traces and health](observability.md)

Configuration, development, code generation and tests belong to the owning service README: [Orch](../orch-user-access/README.md), [Authentication](../core-authentication/README.md), [User](../core-user-manage/README.md).

## Root Make targets

| Target | Action |
|---|---|
| `make dev-env` | Generate private development credentials when missing; skip existing `.env` successfully |
| `make clean` | Remove project containers/images/networks/volumes and local build artifacts; deletes MongoDB/Redis data, preserves `.env` and source |
| `make pkg-test` | Run the shared package race tests in all three modules |
| `make build` / `make vet` | Call the corresponding target in all three service modules |
| `make check` | Compare normalized shared package sources and protobuf contracts |
| `make unit-test` | Run consistency checks and all service race tests |
| `make test` | Run all `.http` collections directly with httpyac; requires a live API and Node.js/npm |
| `make integration` | Run httpyac and the Go API integration suite, including concurrency |
| `make storage-integration` | Build and run tagged Go storage/schema tests in temporary databases using Docker |

Make recipes invoke the native tools directly; there is no `scripts/` directory or Python runtime requirement.
