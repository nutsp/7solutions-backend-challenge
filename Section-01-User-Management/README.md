# User Management API

Go implementation of the [7Solutions backend challenge](https://github.com/7-solutions/backend-challenge), using REST, gRPC, MongoDB and Redis.

| Service | Responsibility |
|---|---|
| [orch-user-access](orch-user-access/README.md) | REST API and orchestration |
| [core-authentication](core-authentication/README.md) | JWT authentication and Redis sessions |
| [core-user-manage](core-user-manage/README.md) | User CRUD and MongoDB audit |

## Start

Requires Docker Compose v2, Make, OpenSSL, curl and jq. Place the separate `ensure-index` repository at `./ensure-index`.

Run from the repository root:

```sh
make dev-env  # Creates .env if missing; keeps existing secrets
make ensure-index
make up
curl --fail http://127.0.0.1:8080/readyz
```

Wait for readiness to return HTTP 200. The API is available at `http://127.0.0.1:8080`; MongoDB, Redis and gRPC remain internal. Keep the existing `.env` when reusing database volumes.

## Data viewers

Both UIs start with `make up` and connect to the existing databases.

| UI | URL | Login |
|---|---|---|
| Mongo Express | http://127.0.0.1:8081 | Open directly; localhost only |
| Redis Insight | http://127.0.0.1:5540 | Complete first-run setup, then select **Auth sessions** |

## Use the API

Register a user and use its access token to read the profile:

```sh
EMAIL="demo-$(date +%s)@example.com"
REGISTER=$(curl --fail-with-body -s http://127.0.0.1:8080/api/v1/users \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"Demo\",\"email\":\"$EMAIL\",\"password\":\"correct-horse-battery\"}")
TOKEN=$(printf '%s' "$REGISTER" | jq -er '.data.tokens.access_token')
curl --fail-with-body http://127.0.0.1:8080/api/v1/users/me \
  -H "Authorization: Bearer $TOKEN"
```

Registration returns HTTP 201; the profile returns HTTP 200 with the created user. See the [API reference](docs/api.md) for login, update, deletion and logout.

## Stop or restart

`make down` stops the stack and preserves database volumes. Run `make up` to start it again with the same `.env`. For a full local reset, `make clean` removes project containers, images, volumes and build artifacts, including all MongoDB users/audits and Redis sessions; see [reset local state](docs/getting-started.md#reset-local-state).

[Setup and troubleshooting](docs/getting-started.md) · [Architecture](docs/architecture.md) · [Operations](docs/README.md)

Development, configuration and test instructions are in each service's README above.
