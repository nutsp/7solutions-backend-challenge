# Shared packages

Each service imports its own copy of these packages. Pass configuration explicitly; `pkg/config` loads YAML and environment overrides.

| Package | Entry point |
|---|---|
| redisclient | New(ctx, cfg) |
| mongoclient | New(ctx, cfg) |
| grpcclient | Dial(cfg), Healthcheck(cfg) |
| grpcserver | New(cfg, log, guards...), Run(ctx, cfg, ...) |
| httpserver | Run(ctx, cfg, handler, log) |
| logger | New(cfg) |
| config | Load(cfg, &settings) using Viper; environment > YAML > existing typed defaults |
| validator | New(cfg) returns a generic validator/v10 engine; callers own struct tags and custom field rules |
| errors | ErrInvalid/ErrNotFound/ErrConflict/ErrUnauthorized/ErrForbidden/ErrSessionLimit/ErrUnavailable/ErrTimeout; Wrap(operation, err); FromDependency(operation, err) |
| errors/grpc | FromDownstream(operation, err), ToStatus(err, logger) |
| errors/http | Write(responseWriter, err) |
| errors/mongodb | From(err) |
| httpresponse | Success(w, status, data), Failure(w, status, code, message), NoContent(w) |
| hash | New(hash.Config), Hash(password), Compare(encoded, password) |
| jwt | New(jwt.Config), Issue(Claims), Parse(token) with HS256 only |
| proto | core-authentication/authentication-service.proto and core-user-manage/user-service.proto |
| requestcontext | New(ctx, candidate), ID(ctx); validated X-Request-ID propagation |
| tracing | Setup(cfg) initializes OpenTelemetry/W3C; returned provider.Shutdown participates in graceful shutdown |
| healthcheck | New(cfg, probe), Liveness/Readiness HTTP handlers, IsHTTP(path), IsGRPC(method) |
| cryptography | New(cfg), Encrypt/Decrypt with associated data; NewLookup(cfg), Lookup(message); ParseKeys(encoded keyring) |
| lifecycle | SignalContext(cfg), Cleanup(cfg, ...) |

See [configuration](../README.md#configuration) for settings and [architecture](../../docs/architecture.md) for service boundaries.

## Generate and test

Run from this `pkg` directory:

```sh
make gen-proto
make runtest
```

`generate` aliases `gen-proto`; `test` aliases `runtest`. Proto generation installs pinned Go plugins, validates the contract copies and runs `protoc` from the owning service module. It generates both domain contracts with that module's import path. Select another compiler with `make gen-proto PROTOC=/path/to/protoc`.

Package tests run `go test -race -count=1 ./...` within `pkg`. From the service directory, `make generate` and `make pkg-test` delegate here. Synchronize shared source changes across all three services before checking contracts or generating bindings.
