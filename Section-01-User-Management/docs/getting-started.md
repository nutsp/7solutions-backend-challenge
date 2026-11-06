# Getting started

[Documentation index](README.md) · [Project overview](../README.md)

## Prerequisites

- Docker with Compose v2, Make and OpenSSL.
- curl and jq for the working API example in the root README. Development prerequisites are listed in each service README.
- The independent `ensure-index` repository available at `<repository-root>/ensure-index`. Compose builds `../ensure-index` relative to `deployment/compose.yaml`; this directory has its own Git repository and is ignored by the application repository. Obtain it before a fresh setup. Its own README covers tool-specific commands.

All commands in this guide run from the application repository root.

## First startup

```sh
make dev-env
make ensure-index
make up
curl --fail http://127.0.0.1:8080/readyz
```

The generator creates `.env` with random development credentials and three independent data/audit/lookup keys, using mode 600. If `.env` already exists, it skips generation and exits successfully without changing the file.

`ensure-index` starts the required MongoDB initialization dependencies and prepares collections, strict validators and indexes with administrative credentials. Normal application startup does not initialize or migrate the schema. See the separate [ensure-index README](../ensure-index/README.md) for tool details.

`up` builds the three application images, starts their dependencies and waits for health checks. MongoDB runs as a single-member replica set for transactions; Redis persists sessions. This local topology is not highly available. REST is published on `127.0.0.1:8080`. Mongo Express (`127.0.0.1:8081`) and Redis Insight (`127.0.0.1:5540`) provide local data viewers; database and gRPC ports remain internal.

## Existing workspace

With an existing `.env` and initialized schema:

```sh
make up
```

Keep the same encryption keys when reusing database volumes. Do not regenerate them to fix a startup failure.

## Data viewers

- **Mongo Express:** http://127.0.0.1:8081 — open directly; no browser login, localhost only.
- **Redis Insight:** http://127.0.0.1:5540 — complete the first-run terms/setup screen, then open **Auth sessions** (Redis DB 0).
- Override ports with `MONGO_EXPRESS_PORT` and `REDIS_INSIGHT_PORT` in `.env`.

## Inspect and stop

```sh
docker compose --env-file .env -f deployment/compose.yaml ps
docker compose --env-file .env -f deployment/compose.yaml logs --tail 100 orch-user-access
make down
```

`down` preserves named database volumes. The configured stop grace period is 20 seconds; services handle SIGINT/SIGTERM, drain requests and close clients. Core User stops and joins its worker before disconnecting MongoDB.

## Troubleshooting

| Symptom | Check |
|---|---|
| Missing `ensure-index` build context | Put the independent tool repository at `./ensure-index` |
| Missing required Compose variables | Generate `.env` for a fresh workspace, or restore the existing private file |
| Missing collections/indexes or invalid storage schema | Run `make ensure-index`; it does not convert legacy data |
| `/readyz` returns 503 | Inspect both Core health statuses, MongoDB and Redis connectivity |
| MongoDB decryption fails | Restore the original keyring/key IDs and lookup key; see [Core User README](../core-user-manage/README.md#configuration) |
| Port 8080 is occupied | Set `GATEWAY_PORT` in `.env` and use that port in requests |
| An old route returns 404 | Use the versioned endpoints in the [API reference](api.md) |

Continue with [registration and JWT examples](api.md).

## Reset local state

From the repository root:

```sh
make clean
```

This removes Compose containers (including tool profiles/orphans), networks, named volumes and service images, plus local `bin`, coverage outputs and generated code-graph artifacts. MongoDB users/audits, Redis sessions and auxiliary UI volume data are deleted. It preserves `.env`, configuration, source code, generated protobuf bindings and mocks. It does not prune unrelated Docker resources or global Go caches. Cleanup can parse the Compose project without an `.env` file; it never starts a service.

To initialize an empty database afterwards, keep the existing `.env` and run:

```sh
make ensure-index
make up
```

Run `make clean` from an individual service directory to remove only that service's build/coverage artifacts; it does not stop the stack.
