# GoLearn URL Shortener

A small URL shortening service written in Go.

## Features

- Short links with random or custom codes
- Redirects with visit counting
- Optional link expiration (TTL) and a background cleanup worker
- SQLite storage with versioned migrations, plus an in-memory store
- Per-IP rate limiting on link creation
- Structured JSON logging, request logging and panic recovery
- Environment-based configuration and graceful shutdown
- Multi-stage Docker image (~22 MB, distroless, non-root)

## Tech stack

- Go 1.27, `net/http` routing (no framework)
- `log/slog` for logging
- `database/sql` with [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO)
- [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate) for rate limiting

## Quick start

### Locally

Requires Go 1.27+.

```bash
make run
```

The service listens on `http://localhost:8080` and stores data in `shortener.db`.

### Docker

```bash
make docker-build
make docker-run
```

Data is kept in the `shortener-data` Docker volume.

## API

### Create a short link

`POST /api/links`

| Field | Type | Required | Description |
|---|---|---|---|
| `url` | string | yes | Target URL, `http` or `https` |
| `code` | string | no | Custom code, 3-32 characters of `A-Z a-z 0-9 - _` |
| `ttl_seconds` | integer | no | Lifetime in seconds, up to one year. Omit for a link that never expires |

```bash
curl -X POST localhost:8080/api/links \
  -d '{"url":"https://go.dev","code":"go-home","ttl_seconds":86400}'
```

```json
{
  "code": "go-home",
  "short_url": "http://localhost:8080/go-home",
  "url": "https://go.dev",
  "created_at": "2026-10-05T10:00:00Z",
  "expires_at": "2026-10-06T10:00:00Z"
}
```

### Follow a short link

`GET /{code}` returns `302 Found` with a `Location` header and increments the visit counter.

### Get link stats

`GET /api/links/{code}`

```json
{
  "code": "go-home",
  "short_url": "http://localhost:8080/go-home",
  "url": "https://go.dev",
  "created_at": "2026-10-05T10:00:00Z",
  "expires_at": "2026-10-06T10:00:00Z",
  "visits": 42
}
```

### Health check

`GET /healthz` returns `200 ok`.

### Errors

Errors come back as JSON in the form `{"error": "message"}`.

| Status | When |
|---|---|
| `400` | Invalid JSON, URL, code or TTL |
| `404` | Link does not exist or has expired |
| `405` | Wrong HTTP method |
| `409` | Custom code is already taken |
| `429` | Rate limit exceeded; see the `Retry-After` header |
| `500` | Unexpected server error |

## Configuration

All settings come from environment variables.

| Variable | Default | Description |
|---|---|---|
| `ADDR` | `:8080` | Listen address |
| `BASE_URL` | `http://localhost:8080` | Public base used to build `short_url` |
| `STORAGE` | `sqlite` | `sqlite` or `memory` |
| `DB_PATH` | `shortener.db` | SQLite database file |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |
| `SHUTDOWN_TIMEOUT` | `10s` | How long to wait for in-flight requests on shutdown |
| `CLEANUP_INTERVAL` | `1m` | How often expired links are deleted |
| `RATE_LIMIT_RPS` | `1` | Link creations per second per IP; `0` disables the limit |
| `RATE_LIMIT_BURST` | `5` | Requests allowed in a burst before limiting starts |

Example:

```bash
BASE_URL=https://sho.rt LOG_LEVEL=debug RATE_LIMIT_RPS=0 make run
```

## Project structure

```
cmd/shortener/        entry point: config, wiring, server lifecycle
internal/config/      environment configuration
internal/link/        domain model, service logic, cleanup worker
internal/httpapi/     HTTP handlers, middleware, rate limiting
internal/memstore/    in-memory store
internal/sqlitestore/ SQLite store and migrations
internal/storetest/   shared contract tests for every store
```

Dependencies flow inward. `httpapi` depends on `link`, and both stores implement the `link.Store` interface. The service never imports a concrete store, so swapping storage only changes `main.go`.

## Development

```bash
make test    # tests with the race detector
make cover   # coverage report
make lint    # gofmt check and go vet
make build   # static binary in bin/
```

Both store implementations run the same contract test suite from `internal/storetest`. Time-dependent code (TTL cleanup, rate limiting) is tested with `testing/synctest`, so the tests do not sleep.

## Limitations

- The rate limiter keeps its state in process memory, so each instance has its own limits.
- The client IP comes from the TCP connection. Behind a reverse proxy, all requests share the proxy's IP until trusted `X-Forwarded-For` handling is added.
- SQLite fits a single instance. Running several instances needs a shared database such as PostgreSQL.
