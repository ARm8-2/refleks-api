# refleks-api

Production-oriented Go monolith API foundation.

This repository starts with a clean baseline:

- Opinionated project layout for long-term maintainability.
- Structured logging and middleware-ready HTTP stack.
- Graceful shutdown and timeout configuration.
- A first operational endpoint: status.

## Project Layout

```text
.
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── httpapi/
│   │   ├── middleware.go
│   │   └── router.go
│   ├── httpserver/
│   │   └── server.go
│   └── status/
│       ├── handler.go
│       ├── handler_test.go
│       └── service.go
├── go.mod
└── README.md
```

## Run

```bash
go run ./cmd/api
```

Server defaults to port `8080`.

## Docker

Build the image:

```bash
docker build \
	--build-arg VERSION=$(git rev-parse --short HEAD) \
	-t refleks-api:local .
```

Run the container:

```bash
docker run --rm -p 8080:8080 --env-file .env refleks-api:local
```

The image is multi-stage and optimized for runtime size:

- Build stage: Go toolchain with BuildKit cache mounts for faster rebuilds.
- Runtime stage: distroless non-root image with only the compiled binary.

## Status Endpoint

Request:

```http
GET /v1/status
```

Example response:

```json
{
	"status": "ok",
	"service": "refleks-api",
	"environment": "development",
	"version": "dev",
	"timestamp": "2026-03-22T11:00:00.000000000Z",
	"uptime_seconds": 12
}
```

## Configuration

Create a local env file from the template:

```bash
cp .env.example .env
```

Then export the values in your shell before running the API (or use your preferred env loader).

Environment variables:

- `APP_NAME` (default: `refleks-api`)
- `APP_ENV` (default: `development`)
- `APP_VERSION` (default: build-time version or `dev`)
- `APP_PORT` (default: `8080`)
- `LOG_LEVEL` (one of: `debug`, `info`, `warn`, `error`)
- `HTTP_READ_TIMEOUT` (default: `15s`)
- `HTTP_WRITE_TIMEOUT` (default: `15s`)
- `HTTP_IDLE_TIMEOUT` (default: `60s`)
- `HTTP_READ_HEADER_TIMEOUT` (default: `5s`)
- `HTTP_SHUTDOWN_TIMEOUT` (default: `10s`)

## Test

```bash
go test ./...
```