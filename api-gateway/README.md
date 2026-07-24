# BeeSter API Gateway

BeeSter is a Go API Gateway built around the standard library and gRPC-Go. It exposes an HTTP reverse proxy and an optional native gRPC passthrough proxy, with explicit configuration and predictable lifecycle behavior.

## Features

- Config-driven HTTP routing with `net/http` + `httputil.ReverseProxy`
- Native gRPC passthrough routing by full-method prefix
- Named HTTP and gRPC upstream registries
- Standard gRPC health checks for readiness
- UUIDv7 request IDs propagated to HTTP headers and gRPC metadata
- Structured `log/slog` access logging
- HS256 JWT validation, trusted user-ID context, and header/metadata injection
- Redis-backed token-bucket rate limiting keyed by authenticated user ID or client IP
- Request-body limit, upstream request timeout, panic recovery, and safe proxy forwarding headers
- Liveness (`/healthz`) and readiness (`/readyz`)
- Graceful HTTP/gRPC shutdown

## Protocol model

BeeSter deliberately does **not** guess how to convert arbitrary HTTP/JSON requests into arbitrary gRPC messages. That requires protobuf descriptors or service-specific adapters. Instead:

- HTTP clients are proxied to HTTP upstreams.
- Native gRPC clients are transparently proxied to gRPC upstreams.

When a product needs HTTP-to-gRPC transcoding, add generated protobuf contracts or an explicit transcoding layer rather than inventing a generic envelope protocol.

## Configuration

Copy `.env.example` to `.env` for local development. Production should inject environment variables from the runtime/orchestrator rather than baking `.env` into the image.

### HTTP routing

```env
HTTP_UPSTREAMS=users=http://users:8080,auth=http://auth:8080
HTTP_ROUTES=GET:/api/users/{id}=users,POST:/api/auth/login=auth
```

Route patterns use Go `http.ServeMux` method/path syntax. The original request path/query are preserved when proxying. A target may include a base path.

### gRPC routing

```env
GRPC_ADDR=:9090
GRPC_TRANSPORT_SECURITY=insecure
GRPC_UPSTREAMS=users=dns:///users:50051,auth=dns:///auth:50051
GRPC_ROUTES=/users.v1.UserService/=users,/auth.v1.AuthService/=auth
```

Longest matching gRPC method prefix wins. Internal gRPC upstreams are expected to expose the standard `grpc.health.v1.Health` service with whole-server status for `service=""` so `/readyz` can verify them.

Use `GRPC_TRANSPORT_SECURITY=tls` in production unless the network security model explicitly provides equivalent transport protection.

### JWT

BeeSter v1 supports HS256 JWT verification:

```env
JWT_ENABLED=true
JWT_HMAC_SECRET=replace-me
JWT_ISSUER=my-issuer
JWT_AUDIENCE=my-api
JWT_USER_ID_CLAIM=sub
JWT_USER_ID_HEADER=X-User-ID
JWT_PUBLIC_PATH_PREFIXES=/healthz,/readyz,/api/auth/
```

Client-supplied `X-User-ID` is removed. After successful validation, BeeSter injects the trusted user ID into HTTP upstream headers and gRPC metadata. When gateway JWT validation is enabled, `Authorization` is stripped by default; set `JWT_FORWARD_AUTHORIZATION=true` only when downstream services genuinely need the original token. When JWT validation is disabled, the header is preserved for downstream-owned authentication.

For larger deployments, prefer asymmetric JWT/JWKS validation in a later version so signing keys do not need to be shared with the gateway.

### Rate limiting

```env
RATE_LIMIT_ENABLED=true
REDIS_ADDR=redis:6379
RATE_LIMIT_CAPACITY=100
RATE_LIMIT_REFILL_PER_SECOND=50
RATE_LIMIT_FAIL_OPEN=false
```

The limiter uses an atomic Redis Lua token bucket and Redis server time. Authenticated traffic is keyed by user ID; unauthenticated/public traffic falls back to client IP. Keys are SHA-256 hashed before storage.

`TRUST_PROXY_HEADERS=false` is the safe default. Enable it only behind a trusted reverse proxy/load balancer that overwrites `X-Forwarded-For` / `X-Real-IP`.

## Local run

```bash
cp .env.example .env
go run ./cmd/beester
```

Checks:

```bash
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
```

## Validation

```bash
gofmt -w .
go vet ./...
go test -race ./...
go build ./cmd/beester
```

Redis integration tests run when `REDIS_TEST_ADDR` is set.
