# Observability examples

Run from the repository root. Set `ACCESS_TOKEN` from a login response; sample IDs and log values below are illustrative.

## Follow service logs

```sh
docker compose --env-file .env -f deployment/compose.yaml logs -f --tail 50 \
  orch-user-access core-authentication core-user-manage
```

## Find a request across services

```sh
curl -i http://127.0.0.1:8080/api/v1/users/me \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'X-Request-ID: debug-request-1'

docker compose --env-file .env -f deployment/compose.yaml logs --no-color --no-log-prefix \
  orch-user-access core-authentication core-user-manage | grep 'debug-request-1'
```

Example response headers:

```text
X-Request-ID: debug-request-1
X-Trace-ID: 0123456789abcdef0123456789abcdef
traceparent: 00-0123456789abcdef0123456789abcdef-fedcba9876543210-01
```

Example HTTP and downstream RPC logs:

```json
{"level":"INFO","msg":"HTTP request","service":"orch-user-access","request_id":"debug-request-1","trace_id":"0123456789abcdef0123456789abcdef","method":"GET","path":"/api/v1/users/me","status":200,"duration_ms":8}
{"level":"INFO","msg":"RPC","service":"core-authentication","request_id":"debug-request-1","trace_id":"0123456789abcdef0123456789abcdef","method":"/platform.authentication.v1.AuthenticationService/Authenticate","code":"OK","duration_ms":3}
```

## Continue a trace

```sh
curl -i http://127.0.0.1:8080/api/v1/users/me \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'traceparent: 00-0123456789abcdef0123456789abcdef-0123456789abcdef-01'

docker compose --env-file .env -f deployment/compose.yaml logs --no-color --no-log-prefix \
  orch-user-access core-authentication core-user-manage | grep '0123456789abcdef0123456789abcdef'
```

Trace correlation is available in headers and logs. No trace collector or viewer is configured.

## Find errors and worker counts

```sh
docker compose --env-file .env -f deployment/compose.yaml logs --since 10m --no-color --no-log-prefix \
  orch-user-access core-authentication core-user-manage | grep '"level":"ERROR"'

docker compose --env-file .env -f deployment/compose.yaml logs --tail 30 --no-color --no-log-prefix \
  core-user-manage | grep '"msg":"user count"'
```

Example worker log, emitted every ten seconds:

```json
{"level":"INFO","msg":"user count","service":"core-user-manage","count":3,"duration_ms":1}
```

## Check health

```sh
curl -i http://127.0.0.1:8080/healthzc
# 200: {"data":{"status":"alive"}}

curl -i http://127.0.0.1:8080/readyz
# 200: {"data":{"status":"ready"}}
# 503: {"error":{"code":"service_unavailable","message":"service temporarily unavailable"}}

docker compose --env-file .env -f deployment/compose.yaml ps
```

Health probes are excluded from request access logs.
