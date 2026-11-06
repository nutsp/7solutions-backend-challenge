# HTTP API tests with httpyac

These files target **VS Code + httpyac**. Named responses are parsed JSON bodies: use `{{register.data.user.id}}`, not REST Client's `{{register.response.body.$.data.user.id}}`. `# @ref register` makes that request's response available and executes it first if it is not cached. See the [httpyac named-response documentation](https://httpyac.github.io/guide/metaData.html#name).

Start the stack using the [project README](../../../README.md). Open a file in VS Code and run **all requests in the file**, or send them individually from top to bottom. Change `@baseUrl` if needed. Each workflow is self-contained and reads its tokens/IDs directly from named responses; no response-dependent aliases are evaluated at file startup.

| File | Coverage |
|---|---|
| [01-users.http](01-users.http) | Register, profile, get, list, update name/email, delete, rejection after deletion |
| [02-sessions.http](02-sessions.http) | Login, refresh rotation, reuse rejection, logout and account cleanup |
| [03-health.http](03-health.http) | Liveness, legacy alias and readiness |
| [04-errors.http](04-errors.http) | Validation, duplicate email, authentication, ownership, missing user and method/route errors |

Every request includes a native `?? status == ...` assertion. Expected error responses are successful tests when their status matches. These assertions check HTTP status; the Go API integration suite also validates response content and concurrency.

## Run with the real httpyac engine

From the repository root, with Node.js/npm installed:

```sh
npx --yes httpyac@6.16.7 send 'orch-user-access/tests/http/*.http' --all --output none --output-failed none --no-color
```

This pins the engine version used by the installed VS Code extension at verification time. Output suppression keeps JWT response bodies out of console output. This command runs the actual `.http` parser, variable references, request dependencies and assertions.

Refresh replaces its token, so keep the request order. Registration uses GUID-based emails. The final requests delete only accounts created by their file; the sessions workflow uses a separate initial session for cleanup after logout.

If interrupted, finish cleanup while tokens are valid, or log in using the account's latest email and the development password declared in the file. Do not rerun registration before cleaning up an earlier account: its cached response will be replaced. Clearing request history/cache may cause `@ref` to create a new fixture. Never commit response history, real credentials or tokens.

For the broader automated suite, run `make integration` from the Orch directory. See the [API specification](../../../docs/api.md) for response envelopes, field rules and sequences.
