# API specification

- Base URL: `http://127.0.0.1:8080`
- JSON requests: `Content-Type: application/json`
- Authenticated requests: `Authorization: Bearer <access_token>`

Interactive request files: [Orch HTTP tests](../orch-user-access/tests/http/README.md).

## Endpoints

| Method | Path | Auth | Request | Success: `data` |
|---|---|---|---|---|
| POST | [`/api/v1/users`](#register) | Public | `name`, `email`, `password` | 201: `{user: User, tokens: Tokens}` |
| POST | [`/api/v1/sessions`](#login) | Public | `email`, `password` | 201: `Tokens` |
| GET | [`/api/v1/users/me`](#current-profile) | Bearer | — | 200: `User` |
| GET | [`/api/v1/users/{id}`](#get-user) | Bearer | — | 200: `User` |
| GET | [`/api/v1/users`](#list-users) | Bearer | Query: `limit`, `after` | 200: `{users: User[], next_cursor?: string}` |
| PATCH | [`/api/v1/users/{id}`](#update-user) | Owner | `name` and/or `email` | 200: `User` |
| DELETE | [`/api/v1/users/{id}`](#delete-user) | Owner | — | 204: no body |
| POST | [`/api/v1/sessions/current/tokens`](#refresh) | Refresh token | `refresh_token` | 200: `Tokens` |
| DELETE | [`/api/v1/sessions/current`](#logout) | Bearer | — | 204: no body |
| GET | [`/healthzc`](#liveness) | Public | — | 200: `{status: "alive"}` |
| GET | [`/readyz`](#readiness) | Public | — | 200: `{status: "ready"}`; 503 if unavailable |

`/healthz` aliases `/healthzc`. Registration sets `Location: /api/v1/users/{id}`; login sets `Location: /api/v1/sessions/current`.

## Fields

| Field | Rule |
|---|---|
| `id`, `after` | 32 hexadecimal characters |
| `name` | Required on registration; 1–100 characters; trimmed; no control characters |
| `email` | Required on registration/login; valid email; at most 254 UTF-8 bytes; trimmed and lowercased |
| `password` | Registration: 12–72 UTF-8 bytes; login: required, at most 72 bytes |
| `limit` | 1–100; default 20 |
| PATCH body | At least one of `name`, `email`; missing fields stay unchanged; null-only patches are invalid |

JSON body limit: 16 KiB. Unknown fields and trailing JSON are rejected. `next_cursor` is omitted on the final page.

## Response examples

Registration, HTTP 201:

```json
{
  "data": {
    "user": {
      "id": "0123456789abcdef0123456789abcdef",
      "name": "Alice",
      "email": "alice@example.com",
      "created_at": "2026-11-06T00:00:00Z",
      "updated_at": "2026-11-06T00:00:00Z"
    },
    "tokens": {
      "access_token": "<access_token>",
      "refresh_token": "<refresh_token>",
      "token_type": "Bearer",
      "expires_at": "2026-11-06T00:15:00Z",
      "user_id": "0123456789abcdef0123456789abcdef"
    }
  }
}
```

`User` is the object under `user`; `Tokens` is the object under `tokens`. Get/PATCH return `{"data": User}`; login/refresh return `{"data": Tokens}`.

Empty list, HTTP 200:

```json
{"data":{"users":[]}}
```

Error, HTTP 401:

```json
{"error":{"code":"unauthorized","message":"invalid credentials or token"}}
```

| HTTP | Error code | Meaning |
|---|---|---|
| 400 | `invalid_request` | Invalid input |
| 401 | `unauthorized` | Invalid credentials or token |
| 403 | `forbidden` | Ownership violation |
| 404 | `not_found` | User or route missing |
| 405 | `method_not_allowed` | Unsupported method |
| 409 | `conflict` | Email already registered |
| 429 | `session_limit` | Active session limit reached |
| 500 | `internal_error` | Internal failure |
| 503 | `service_unavailable` | Dependency unavailable |
| 504 | `timeout` | Request deadline exceeded |

## Sequence diagrams

`Orch` = orch-user-access; `Auth` = core-authentication; `User` = core-user-manage. All calls are synchronous. Error responses stop the flow; later messages are not executed. Unless a branch is shown, arrows describe the success path. Dependency failures return 503; deadlines return 504.

Payload validation finishes before business RPCs. Protected GET/PATCH routes authenticate before validating handler fields; DELETE validates its target ID before owner authorization. Every REST request carries request/trace IDs and an eight-second request deadline.

Examples use these values from a registration/login response:

```sh
BASE=http://127.0.0.1:8080
ACCESS_TOKEN='<access_token>'
REFRESH_TOKEN='<refresh_token>'
USER_ID='<user_id>'
```

### Shared authentication for protected GET/PATCH

Used by profile, get-user, list and update. A missing/malformed Bearer header returns 401 before the Auth RPC.

```mermaid
sequenceDiagram
    autonumber
    participant O as Orch
    participant A as Auth
    participant R as Redis
    participant U as User
    participant M as MongoDB
    O->>A: Authenticate(access JWT)
    A->>A: Verify HS256, issuer, audience and expiry
    A->>R: GetSession(user ID, session ID)
    R-->>A: Session record
    A->>A: Check session owner and absolute expiry
    A->>U: GetInternal(authenticated user ID)
    U->>M: Read encrypted authenticated account
    M-->>U: Encrypted User
    Note over U: Decrypt public fields<br/>Verify email lookup integrity
    U-->>A: Account identity
    A-->>O: Principal(user ID)
```

| Failure | Result |
|---|---|
| Invalid/expired JWT; missing, expired or mismatched session | 401; stop before account/profile lookup |
| Authenticated account deleted | 401; stop before the requested user operation |
| Redis/User RPC failure or deadline | 503/504; no operation is executed |

<a id="register"></a>

### POST /api/v1/users — Register

**Request:** `{name, email, password}`. **Response:** 201, `data: {user: User, tokens: Tokens}`; `Location: /api/v1/users/{id}`. **Errors:** 400 input, 409 duplicate email, 429 session limit, 503/504 dependency failure.

```sh
curl -i "$BASE/api/v1/users" -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","password":"correct-horse-battery"}'
```

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orch
    participant U as User
    participant A as Auth
    participant R as Redis
    C->>O: POST /api/v1/users
    O->>U: CreateUser(normalized registration)
    U->>U: Validate, bcrypt hash, generate user and audit IDs
    Note over U: MongoDB transaction:<br/>encrypted User + CREATE audit<br/>Unique email index rejects<br/>duplicates<br/>Audit failure rolls back User
    U-->>O: Public User without password hash
    O->>A: Login(saved email, original password)
    A->>U: FindCredential(email)
    U-->>A: User ID and password hash
    A->>A: Compare bcrypt<br/>generate session, refresh token and JWT
    A->>R: Atomic CreateSession with refresh hash and expiry
    R-->>A: Session created, below 100 active-session limit
    A-->>O: Tokens
    O-->>C: 201, Location, data.user and data.tokens
```

If login/session creation fails after the User transaction commits, the account remains. Use the login endpoint; repeating registration returns 409. Tokens are returned only after Redis accepts the session.

<a id="login"></a>

### POST /api/v1/sessions — Login

**Request:** `{email, password}`. **Response:** 201, `data: Tokens`; `Location: /api/v1/sessions/current`. **Errors:** 400 malformed input, 401 credentials, 429 active-session limit, 503/504 dependency failure.

```sh
curl -i "$BASE/api/v1/sessions" -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"correct-horse-battery"}'
```

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orch
    participant A as Auth
    participant U as User
    participant R as Redis
    C->>O: POST /api/v1/sessions
    O->>A: Login(normalized email, password)
    A->>U: FindCredential(email)
    Note over U: MongoDB lookup by normalized<br/>email HMAC<br/>Returns password hash only to<br/>Auth
    U-->>A: Credential or NotFound
    A->>A: Bcrypt comparison<br/>dummy hash if account is unknown
    alt Missing account or wrong password
        A-->>O: Unauthorized
        O-->>C: 401 unauthorized
    else Credentials valid
        A->>A: Generate session ID, refresh token and HS256 JWT
        A->>R: Atomic CreateSession with fixed 30-day expiry
        R-->>A: Session persisted
        A-->>O: Tokens
        O-->>C: 201 and session Location
    end
```

Access JWT lifetime defaults to 15 minutes. Sessions expire after 30 days and are limited to 100 active sessions per user. Redis rejection/failure returns no tokens.

<a id="current-profile"></a>

### GET /api/v1/users/me — Current profile

**Auth:** Bearer. **Response:** 200, `data: User`. **Errors:** 401 authentication, 404 account removed between checks, 503/504 dependency failure.

```sh
curl -i "$BASE/api/v1/users/me" -H "Authorization: Bearer $ACCESS_TOKEN"
```

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orch
    participant A as Auth
    participant U as User
    participant M as MongoDB
    C->>O: GET /api/v1/users/me with Bearer JWT
    O->>A: Authenticate(JWT), shared authentication flow
    A-->>O: Principal(user ID)
    O->>U: GetUser(principal ID), forwarded actor ID
    U->>M: Read profile by authenticated user ID
    M-->>U: Encrypted User
    U->>U: Decrypt name and email
    U-->>O: Public User
    O-->>C: 200, data containing User
```

The account read during authentication and the profile read are separate. Public responses never contain `password_hash`.

<a id="get-user"></a>

### GET /api/v1/users/{id} — Get user

**Auth:** Bearer; authenticated users may read another user’s public profile. **Path:** 32-character hexadecimal ID. **Response:** 200, `data: User`. **Errors:** 400 ID, 401 authentication, 404 target missing, 503/504 dependency failure.

```sh
curl -i "$BASE/api/v1/users/$USER_ID" -H "Authorization: Bearer $ACCESS_TOKEN"
```

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orch
    participant A as Auth
    participant U as User
    participant M as MongoDB
    C->>O: GET /api/v1/users/id with Bearer JWT
    O->>A: Authenticate(JWT), shared authentication flow
    A-->>O: Principal(actor ID)
    O->>O: Validate target ID
    O->>U: GetUser(target ID), forwarded actor ID
    U->>M: Read target User by ID
    M-->>U: Encrypted User or NotFound
    U->>U: Decrypt public fields
    U-->>O: Public User
    O-->>C: 200, data containing User
```

<a id="list-users"></a>

### GET /api/v1/users — List users

**Auth:** Bearer. **Query:** `limit` 1–100 (default 20), optional hexadecimal `after` cursor. **Response:** 200, `data: {users: User[], next_cursor?: string}`. **Errors:** 400 query, 401 authentication, 503/504 dependency failure.

```sh
curl -i "$BASE/api/v1/users?limit=20" -H "Authorization: Bearer $ACCESS_TOKEN"
curl -i "$BASE/api/v1/users?limit=20&after=$USER_ID" -H "Authorization: Bearer $ACCESS_TOKEN"
```

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orch
    participant A as Auth
    participant U as User
    participant M as MongoDB
    C->>O: GET /api/v1/users with Bearer JWT and query
    O->>A: Authenticate(JWT), shared authentication flow
    A-->>O: Principal(actor ID)
    O->>O: Validate limit and cursor
    O->>U: ListUsers(limit, after), forwarded actor ID
    U->>M: Find ID greater than cursor, sort ID ascending, fetch limit plus one
    M-->>U: Encrypted records
    U->>U: Decrypt page and compute next cursor
    U-->>O: Users and optional next cursor
    O-->>C: 200, data containing users and next_cursor
```

`next_cursor` is the last returned ID when another page exists. It is omitted on the final page. An empty result returns `{"data":{"users":[]}}`.

<a id="update-user"></a>

### PATCH /api/v1/users/{id} — Update own user

**Auth:** Bearer and owner. **Request:** at least one of `name`, `email`. **Response:** 200, `data: User`. **Errors:** 400 ID/body, 401 authentication, 403 owner mismatch, 404 target missing, 409 duplicate email, 503/504 dependency or transaction failure.

```sh
curl -i -X PATCH "$BASE/api/v1/users/$USER_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Alice Updated","email":"alice.updated@example.com"}'
```

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orch
    participant A as Auth
    participant U as User
    participant M as MongoDB
    C->>O: PATCH /api/v1/users/id with Bearer JWT and patch
    O->>A: Authenticate(JWT), shared authentication flow
    A-->>O: Principal(actor ID)
    O->>U: UpdateUser(validated ID and patch), forwarded actor ID
    U->>U: Require owner<br/>normalize changed values
    U->>M: Begin transaction and read current encrypted User
    M-->>U: Current encrypted User
    Note over U: Decrypt before image<br/>Compare normalized values
    alt Normalized values unchanged
        U-->>O: Current User, original UpdatedAt, no audit
    else Values changed
        U->>M: Update encrypted fields and insert encrypted UPDATE audit
        M-->>U: Commit, or rollback on any write or audit error
        U-->>O: Updated public User after successful commit
    end
    O-->>C: 200, data containing User
```

Missing fields remain unchanged. A real change advances `updated_at`; `created_at` is preserved. Audit failure rolls back the User write. MongoDB transaction retries re-read the current record.

<a id="delete-user"></a>

### DELETE /api/v1/users/{id} — Delete own user

**Auth:** signed, unexpired Bearer JWT with owner ID. **Response:** 204, empty body. **Errors:** 400 ID, 401 JWT/session when account exists, 403 owner mismatch, 503/504 transaction or revocation failure.

```sh
curl -i -X DELETE "$BASE/api/v1/users/$USER_ID" -H "Authorization: Bearer $ACCESS_TOKEN"
```

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orch
    participant A as Auth
    participant U as User
    participant R as Redis
    C->>O: DELETE /api/v1/users/id with Bearer JWT
    O->>A: AuthorizeDelete(JWT, validated target ID)
    A->>A: Verify JWT and require subject equals target
    A->>U: GetInternal(target ID)
    U-->>A: Account exists or NotFound
    Note over A: Account exists: shared<br/>authentication checks<br/>Account missing: owner cleanup<br/>allowed<br/>without a live session
    A-->>O: Authorized owner principal
    O->>U: DeleteUser(target ID), forwarded owner ID
    Note over U: MongoDB transaction: read,<br/>delete User,<br/>append DELETE audit<br/>Missing User: no-op<br/>Audit failure rolls back<br/>deletion
    U-->>O: Deleted or already absent
    O->>A: RevokeUser(owner ID), forwarded owner ID
    A->>R: Read user session index and delete all indexed sessions
    A-->>O: Revocation completed
    O-->>C: 204, empty body
```

User deletion and Redis cleanup are separate operations. If cleanup fails after deletion, retry with the same unexpired owner JWT. The retry creates no duplicate DELETE audit; an expired JWT returns 401.

<a id="refresh"></a>

### POST /api/v1/sessions/current/tokens — Refresh

**Request:** `{refresh_token}`; no Bearer header. **Response:** 200, `data: Tokens`. **Errors:** 400 body, 401 invalid/reused/expired token or deleted account, 503/504 dependency failure.

```sh
curl -i "$BASE/api/v1/sessions/current/tokens" -H 'Content-Type: application/json' \
  -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}"
```

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orch
    participant A as Auth
    participant R as Redis
    participant U as User
    C->>O: POST refresh_token, required and at most 256 bytes
    O->>A: Refresh(raw token)
    Note over A: Parse user ID, session ID and<br/>random secret
    A->>R: GetSession(user ID, session ID)
    R-->>A: Refresh hash and original expiry
    A->>A: Constant-time hash comparison and expiry check
    A->>U: GetInternal(session user ID)
    U-->>A: Account still exists
    A->>A: Generate replacement refresh token and access JWT
    A->>R: Lua compare-and-swap old hash to replacement hash
    R-->>A: Rotation won, otherwise Unauthorized
    A-->>O: Replacement Tokens after successful rotation
    O-->>C: 200, data containing Tokens
```

Replace both stored tokens with the response values. Concurrent reuse has one winner; other requests return 401. Rotation preserves the original session ID and 30-day expiry.

<a id="logout"></a>

### DELETE /api/v1/sessions/current — Logout

**Auth:** signed, unexpired Bearer JWT. **Response:** 204, empty body. **Errors:** 401 missing/invalid/expired JWT, 503/504 Redis failure.

```sh
curl -i -X DELETE "$BASE/api/v1/sessions/current" -H "Authorization: Bearer $ACCESS_TOKEN"
```

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orch
    participant A as Auth
    participant R as Redis
    C->>O: DELETE /api/v1/sessions/current with Bearer JWT
    O->>A: Logout(JWT)
    A->>A: Verify signature, issuer, audience and expiry
    A->>R: Lua delete session and remove session-index member
    R-->>A: Session removed or already absent
    A-->>O: Logout completed
    O-->>C: 204, empty body
```

Logout does not require a profile lookup or an existing live session. A repeat logout with a still-valid signed JWT returns 204; later protected requests with that session return 401.

<a id="liveness"></a>

### GET /healthzc — Liveness

**Auth:** public. **Alias:** `/healthz`. **Response:** 200, `{"data":{"status":"alive"}}`. No dependency checks.

```sh
curl -i "$BASE/healthzc"
curl -i "$BASE/healthz"
```

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orch
    C->>O: GET /healthzc or /healthz
    O-->>C: 200, data status alive
```

<a id="readiness"></a>

### GET /readyz — Readiness

**Auth:** public. **Response:** 200, `{"data":{"status":"ready"}}`; 503, `service_unavailable` if a Core probe fails or does not report SERVING.

```sh
curl -i "$BASE/readyz"
```

```mermaid
sequenceDiagram
    autonumber
    actor C as Client
    participant O as Orch
    participant U as User health service
    participant A as Auth health service
    Note over U,A: Cached SERVING state from<br/>background probes<br/>User: MongoDB ping<br/>Auth: Redis ping
    C->>O: GET /readyz, bounded by readiness timeout
    O->>U: gRPC Health.Check
    U-->>O: SERVING or failed status
    alt User health fails or is not SERVING
        O-->>C: 503 service_unavailable, skip Auth probe
    else User is SERVING
        O->>A: gRPC Health.Check
        A-->>O: SERVING or failed status
        O-->>C: 200 ready if SERVING, otherwise 503 unavailable
    end
```

Readiness uses the gRPC health state maintained by each Core’s background probe; it does not query MongoDB/Redis directly per HTTP request. HTTP and gRPC health probes are excluded from access logs.
