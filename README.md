# refleks-api

Production-oriented Go monolith API foundation.

This repository now includes:

- Opinionated project layout for long-term maintainability.
- Structured logging and middleware-ready HTTP stack.
- Graceful shutdown and timeout configuration.
- A first operational endpoint: status.
- Run sync API for .refleks uploads and deduplication.
- Supabase-backed metadata persistence.
- Cloudflare R2 raw file storage.

## Project Layout

```text
.
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   ├── auth/
│   │   ├── handler.go
│   │   └── service.go
│   ├── benchmarks/
│   │   ├── adapters/
│   │   │   └── supabase_repository.go
│   │   ├── contracts.go
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── types.go
│   ├── config/
│   │   └── config.go
│   ├── httpapi/
│   │   ├── middleware.go
│   │   └── router.go
│   ├── httpserver/
│   │   └── server.go
│   ├── leaderboards/
│   │   ├── adapters/
│   │   │   └── supabase_repository.go
│   │   ├── contracts.go
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── types.go
│   ├── runsync/
│   │   ├── adapters/
│   │   │   ├── r2_store.go
│   │   │   └── supabase_repository.go
│   │   ├── contracts.go
│   │   ├── handler.go
│   │   ├── refleks_format.go
│   │   ├── service.go
│   │   └── types.go
│   ├── supabase/
│   │   └── client.go
│   └── status/
│       ├── handler.go
│       ├── handler_test.go
│       └── service.go
├── testdata/
│   ├── client-reference/
│   └── refleks-samples/
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

- Build stage: Go toolchain compiling a static Linux binary.
- Runtime stage: distroless non-root image with only the compiled binary.

### Run Alongside Worker (VPS)

If you run API and worker as separate containers on the same Ubuntu VPS, keep them on one Docker network and use restart policies.

```bash
docker network create refleks-net

docker run -d \
	--name refleks-api \
	--restart unless-stopped \
	--network refleks-net \
	-p 8080:8080 \
	--env-file .env \
	refleks-api:local

docker run -d \
	--name refleks-worker \
	--restart unless-stopped \
	--network refleks-net \
	--env-file worker.env \
	refleks-worker:latest
```

Notes:

- The worker normally does not expose an HTTP port.
- Both services should point to the same Supabase Postgres and R2 credentials.

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

## Auth Stub Endpoints

Request:

```http
GET /v1/auth/session
```

Response:

```json
{
	"status": "stub",
	"message": "auth integration pending",
	"supabase_configured": true,
	"supabase_reachable": true,
	"authenticated": false
}
```

This endpoint is intentionally a stable scaffold for future JWT/session auth integration.

### Steam Login Stub (Premium Scope Scaffold)

Request:

```http
POST /v1/auth/steam/login
```

Response:

```json
{
	"status": "stub",
	"message": "steam login integration pending",
	"provider": "steam",
	"premium_scope": "parquet_lab_private",
	"authenticated": false,
	"token_issued": false,
	"requires_steam_oidc": true
}
```

This endpoint is a stable contract placeholder for premium/private access flows (for example, private parquet access).

## Benchmark Endpoints

Benchmark endpoints are read-only and are enabled when `SUPABASE_DB_URL` is configured.

### List Benchmarks

Request:

```http
GET /v1/benchmarks?q=voltaic
```

Optional compact response mode:

```http
GET /v1/benchmarks?view=progress
```

Supported query params:

- `q` optional benchmark text filter (matches benchmark name or abbreviation)
- `view` optional response mode: `full` (default) or `progress`
	- `full`: returns the complete benchmark hierarchy including ordered `ranks` plus scenario links and rank thresholds when available
	- `progress`: returns the same hierarchy layout used by app `benchmarks_data.json` with ordered `ranks` (no scenario link arrays)

Responses include an `ETag`; clients should send `If-None-Match` on repeat requests to receive `304 Not Modified` when the benchmark payload has not changed.

Response:

```json
{
	"benchmarks": [
		{
			"benchmarkName": "Voltaic Intermediate S5",
			"rankCalculation": "voltaic_energy",
			"abbreviation": "VT Int S5",
			"color": "#6D8CFF",
			"spreadsheetURL": "https://example.test/sheet",
			"dateAdded": "2025-01-22",
			"difficulties": [
				{
					"difficultyName": "Intermediate",
					"kovaaksBenchmarkId": 12345,
					"sharecode": "KOVAAKSXYZ",
					"ranks": [
						{
							"name": "Silver",
							"color": "#CBD9E6"
						},
						{
							"name": "Gold",
							"color": "#CAB148"
						}
					],
					"scenarios": [
						{
							"scenarioName": "VT 1w3ts",
							"rankThresholds": [
								5000,
								6000,
								7000
							]
						}
					],
					"categories": [
						{
							"categoryName": "Static",
							"color": "#1D3557",
							"scenarios": [
								{
									"scenarioName": "VT 1w3ts",
									"rankThresholds": [
										5000,
										6000,
										7000
									]
								}
							],
							"subcategories": [
								{
									"subcategoryName": "1w3ts",
									"scenarioCount": 3,
									"color": "#457B9D",
									"scenarios": [
										{
											"scenarioName": "VT 1w3ts",
											"rankThresholds": [
												5000,
												6000,
												7000
											]
										}
									]
								}
							]
						}
					]
				}
			]
		}
	],
	"count": 1
}
```

## Leaderboard Endpoints

Leaderboard endpoints are read-only and are enabled when `SUPABASE_DB_URL` is configured.

### Scenario Leaderboard

Request:

```http
GET /v1/leaderboards/scenario?scenario=VT%201w3ts&limit=100&offset=0
```

You can also query by `scenario_id`.

Response:

```json
{
	"scenario_id": 42,
	"scenario_name": "VT 1w3ts Intermediate S5",
	"entries": [
		{
			"rank": 1,
			"best_score": 1562.4,
			"best_epoch_milli": 1762190000000,
			"steam_id": "76561198000000000",
			"steam_username": "alice"
		}
	],
	"limit": 100,
	"offset": 0,
	"count": 1,
	"has_more": false,
	"refreshed_at": "2026-04-04T14:28:00Z"
}
```

### Benchmark Difficulty Leaderboard

Request:

```http
GET /v1/leaderboards/benchmark-difficulty?kovaaks_benchmark_id=12345&limit=100&offset=0
```

You can also query by `difficulty_id`.

Response:

```json
{
	"difficulty_id": 10,
	"kovaaks_benchmark_id": 12345,
	"benchmark_name": "Voltaic Intermediate S5",
	"difficulty_name": "Intermediate",
	"entries": [
		{
			"rank": 1,
			"composite_score": 8654.2,
			"matched_scenarios": 18,
			"last_epoch_milli": 1762190000000,
			"steam_id": "76561198000000000",
			"steam_username": "alice"
		}
	],
	"limit": 100,
	"offset": 0,
	"count": 1,
	"has_more": false,
	"refreshed_at": "2026-04-04T14:28:00Z"
}
```

## Run Sync Endpoints

Run sync endpoints are intentionally public for desktop ingestion. No auth header or user account is required for uploads, hash negotiation, or raw downloads.

Run files use the canonical `.refleks` extension. The API normalizes filenames to keep the extension attached on storage and download responses.

### Sync One

Request:

```http
POST /v1/runs/sync
Content-Type: application/octet-stream

<raw .refleks bytes>
```

Also supported: `multipart/form-data` with a `file` field.

Response:

```json
{
	"hash": "<sha256>",
	"already_present": false,
	"stored": true,
	"file_name": "air far long strafes %70 - Challenge - 2024.12.22-19.47.47 Stats.refleks",
	"epoch_milli": 1734896867000,
	"size_bytes": 123456
}
```

### Sync Bulk

Request:

```http
POST /v1/runs/sync/bulk
Content-Type: multipart/form-data
```

Body fields: `files[]` (or repeated `file`) containing .refleks files.

Response:

```json
{
	"results": [
		{
			"file_name": "sample.refleks",
			"hash": "<sha256>",
			"already_present": false,
			"stored": true
		}
	],
	"count": 1
}
```

### Missing Hashes Check

Request:

```http
POST /v1/runs/sync/missing
Content-Type: application/json

{
	"hashes": ["<sha256>", "<sha256>"]
}
```

Note: `hashes` are SHA-256 digests of the full raw `.refleks` file bytes (the same `hash` returned by sync responses), not the internal payload checksum embedded inside the `.refleks` binary.

Response:

```json
{
	"missing_hashes": ["<sha256>"]
}
```

### Browse Runs

Request:

```http
GET /v1/runs?limit=25&offset=0&sort=uploaded_at_desc&scenario=VT&steam_username=alice&has_mouse_trace=true
```

Supported filters:

- `limit` and `offset` for pagination
- `sort` with `uploaded_at_desc`, `uploaded_at_asc`, `epoch_desc`, `epoch_asc`, `score_desc`, or `score_asc`
- `scenario` for scenario name matching
- `steam_id` and `steam_username` for account filtering
- `q` for a general text search across filename, scenario, and Steam username
- `has_mouse_trace` for trace presence filtering
- `min_score` and `max_score` for score range filtering
- `from_epoch` and `to_epoch` for epoch bounds

Response:

```json
{
	"runs": [
		{
			"hash": "<sha256>",
			"file_name": "air far long strafes %70 - Challenge - 2024.12.22-19.47.47 Stats.refleks",
			"scenario_name": "air far long strafes %70",
			"steam_username": "alice",
			"epoch_milli": 1734896867000,
			"uploaded_at": "2026-03-25T11:00:00Z",
			"size_bytes": 123456,
			"score": 1162.9,
			"accuracy": 0.925,
			"avg_ttk_seconds": 0.495745,
			"duration_seconds": 42.08,
			"sens_cm360": 36.48,
			"has_mouse_trace": true,
			"avg_mouse_speed": 1284.2,
			"mouse_vid": "046D",
			"mouse_pid": "C539"
		}
	],
	"limit": 25,
	"offset": 0,
	"count": 1,
	"has_more": false
}
```

### Download Raw File

Request:

```http
GET /v1/runs/raw/{hash}
```

Response:

- `200 OK` with raw `.refleks` bytes from R2
- `404 Not Found` when the hash is not present

Headers:

- `Content-Type: application/octet-stream`
- `Content-Disposition: attachment; filename="<original file name>.refleks"`

### Download Raw URL

Request:

```http
GET /v1/runs/raw/{hash}/url
```

Response (public bucket mode):

```json
{
	"hash": "<sha256>",
	"file_name": "<original file name>.refleks",
	"url": "https://pub-xxx.r2.dev/runs/2026/03/25/<sha256>.refleks",
	"access": "public"
}
```

Response (signed fallback mode):

```json
{
	"hash": "<sha256>",
	"file_name": "<original file name>.refleks",
	"url": "https://<account>.r2.cloudflarestorage.com/...",
	"access": "signed",
	"expires_at": "2026-03-22T15:04:05Z"
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
- `RUNSYNC_ENABLED` (default: `false`)
- `SUPABASE_DB_URL` (optional; when set, enables database-backed endpoints such as benchmarks, leaderboards, and run sync, and is validated on startup)
- `R2_ENDPOINT` (required when run sync is enabled)
- `R2_REGION` (default: `auto`)
- `R2_RAW_PUBLIC_BUCKET` (default: `refleks-raw-public`)
- `R2_LAB_PRIVATE_BUCKET` (default: `refleks-lab-private`, reserved for future premium/lab data flows)
- `R2_RAW_PUBLIC_BASE_URL` (optional, enables direct public URL responses for raw downloads)
- `R2_SIGNED_URL_TTL` (default: `15m`, used when public base URL is not configured)
- `R2_ACCESS_KEY_ID` (required when run sync is enabled)
- `R2_SECRET_ACCESS_KEY` (required when run sync is enabled)
- `R2_KEY_PREFIX` (default: `runs`)
- `RUNSYNC_MAX_FILE_BYTES` (default: `26214400`)
- `RUNSYNC_MAX_BULK_FILES` (default: `100`)
- `RUNSYNC_MAX_BULK_BYTES` (default: `262144000`)
- `RUNSYNC_MAX_MISSING_HASHES` (default: `1000`)

When `RUNSYNC_ENABLED=true`, the API expects `accounts`, `scenarios`, and `runs` tables/indexes to already exist (for example created by a separate worker or migration container).

When benchmark/leaderboard endpoints are enabled (`SUPABASE_DB_URL` is set), the API expects benchmark and leaderboard tables to exist (for example created by a separate worker or migration container).

## Test

```bash
go test ./...
```
