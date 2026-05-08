# refleks-api

Production-oriented Go monolith API foundation.

This repository now includes:

- Opinionated project layout for long-term maintainability.
- Structured logging and middleware-ready HTTP stack.
- Graceful shutdown and timeout configuration.
- Status endpoint.
- Stats endpoint for aggregate database counts.
- Run sync API for `.refleks` uploads, deduplication, browsing, and raw downloads.
- Scenario browser (searchable, sortable, with score/sensitivity distributions).
- Player browser (searchable, sortable, with run counts).
- Benchmark and leaderboard read endpoints.
- Postgres-backed metadata persistence.
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
│   │   │   └── postgres_repository.go
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
│   │   │   └── postgres_repository.go
│   │   ├── contracts.go
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── types.go
│   ├── players/
│   │   ├── adapters/
│   │   │   └── postgres_repository.go
│   │   ├── contracts.go
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── types.go
│   ├── runs/
│   │   ├── adapters/
│   │   │   ├── r2_store.go
│   │   │   └── postgres_repository.go
│   │   ├── contracts.go
│   │   ├── handler.go
│   │   ├── refleks_format.go
│   │   ├── service.go
│   │   └── types.go
│   ├── scenarios/
│   │   ├── adapters/
│   │   │   └── postgres_repository.go
│   │   ├── contracts.go
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── types.go
│   ├── postgres/
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
- Both services should point to the same Postgres database and R2 credentials.
- In Docker Compose, the API and worker should usually use `POSTGRES_HOST=postgres` on the shared `refleks-net` network.

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

## Stats Endpoint

This endpoint is read-only and enabled when database configuration is present (`DATABASE_URL` or `POSTGRES_*`).

Request:

```http
GET /v1/stats
```

Example response:

```json
{
	"total_runs": 124523,
	"total_players": 3812,
	"total_scenarios": 296
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
	"database_configured": true,
	"database_reachable": true,
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

Benchmark endpoints are read-only and are enabled when database configuration is present (`DATABASE_URL` or `POSTGRES_*`).

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

Leaderboard endpoints are read-only and are enabled when database configuration is present (`DATABASE_URL` or `POSTGRES_*`).

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

Uploads are deduplicated first by the API's SHA-256 of the full raw `.refleks` file and then, when present, by the embedded Kovaaks stats `Hash` value.

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

Supported query params:

- `limit` and `offset` for pagination
- `sort` — ordering; supported values:
  - `uploaded_at_desc` (default), `uploaded_at_asc`
  - `epoch_desc`, `epoch_asc`
  - `score_desc`, `score_asc`
  - `accuracy_desc`, `accuracy_asc`
  - `avg_ttk_desc`, `avg_ttk_asc`
- `q` — general text search across filename, scenario name, Steam username, and Steam ID
- `scenario_id` — filter by exact scenario ID (matches the `id` returned from scenario endpoints)
- `scenario` — scenario name substring filter (case-insensitive)
- `steam_id` — Steam ID substring filter
- `steam_username` — Steam username substring filter
- `has_mouse_trace` — `true` or `false`
- `min_score`, `max_score` — inclusive score range
- `min_accuracy`, `max_accuracy` — inclusive accuracy range (0–1)
- `from_epoch`, `to_epoch` — run timestamp bounds in milliseconds

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

### Get Run Detail

Request:

```http
GET /v1/runs/{hash}
```

Returns the full metadata card for a single run identified by its SHA-256 hash.

Response:

```json
{
	"hash": "<sha256>",
	"file_name": "air far long strafes %70 - Challenge - 2024.12.22-19.47.47 Stats.refleks",
	"scenario_name": "air far long strafes %70",
	"steam_id": "76561198000000000",
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
```

Error responses:

- `400 Bad Request` when the hash is not a valid 64-character hex string
- `404 Not Found` when no run with that hash exists

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

## Scenario Endpoints

Scenario endpoints are read-only and are enabled when database configuration is present (`DATABASE_URL` or `POSTGRES_*`).

### Browse Scenarios

Request:

```http
GET /v1/scenarios?q=VT&sort=run_count_desc&limit=50&offset=0
```

Supported query params:

- `q` — optional name filter (case-insensitive substring match)
- `sort` — `run_count_desc` (default), `run_count_asc`, `name_asc`, `name_desc`, `updated_at_desc`
- `limit` — page size (default `50`, max `200`)
- `offset` — pagination offset (default `0`)

Response:

```json
{
	"scenarios": [
		{
			"id": 42,
			"scenario_name": "VT 1w3ts",
			"run_count": 18432,
			"score_sample_count": 18432,
			"updated_at": "2026-04-28T10:00:00Z"
		}
	],
	"limit": 50,
	"offset": 0,
	"count": 1,
	"has_more": false
}
```

### Get Scenario Detail

Request:

```http
GET /v1/scenarios/{id}
```

Returns full metadata for one scenario including score and sensitivity distributions. Distributions are omitted when no data has been collected yet.

Response:

```json
{
	"id": 42,
	"scenario_name": "VT 1w3ts",
	"run_count": 18432,
	"score_sample_count": 18432,
	"score_distribution": {"1000-1100": 120, "1100-1200": 340},
	"sens_sample_count": 9210,
	"sens_distribution": {"30-40": 512, "40-50": 723},
	"updated_at": "2026-04-28T10:00:00Z"
}
```

Error responses:

- `400 Bad Request` when the id is not a valid integer
- `404 Not Found` when no scenario with that id exists

## Player Endpoints

Player endpoints are read-only and are enabled when database configuration is present (`DATABASE_URL` or `POSTGRES_*`).

### Browse Players

Request:

```http
GET /v1/players?q=alice&sort=run_count_desc&limit=50&offset=0
```

Supported query params:

- `q` — optional filter matched against both Steam username and Steam ID (case-insensitive substring)
- `sort` — `run_count_desc` (default), `run_count_asc`, `name_asc`
- `limit` — page size (default `50`, max `200`)
- `offset` — pagination offset (default `0`)

Response:

```json
{
	"players": [
		{
			"steam_id": "76561198000000000",
			"steam_username": "alice",
			"run_count": 412,
			"last_run_at": "2026-04-28T09:15:00Z"
		}
	],
	"limit": 50,
	"offset": 0,
	"count": 1,
	"has_more": false
}
```

### Get Player Detail

Request:

```http
GET /v1/players/{steam_id}
```

Returns full profile for one player identified by their Steam ID.

Response:

```json
{
	"steam_id": "76561198000000000",
	"steam_username": "alice",
	"run_count": 412,
	"last_run_at": "2026-04-28T09:15:00Z",
	"created_at": "2025-11-01T08:00:00Z"
}
```

Error responses:

- `400 Bad Request` when `steam_id` is empty
- `404 Not Found` when no account with that Steam ID exists

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
- `RUNSYNC_ENABLED` (default: `false`) — gates upload endpoints only (sync, bulk sync, missing hashes); browse/detail/download endpoints are enabled independently of this flag
- `DATABASE_URL` (optional; when set, enables database-backed endpoints: benchmarks, leaderboards, scenarios, players, and runs browse/detail)
- `POSTGRES_HOST` (default: `postgres`; used when `DATABASE_URL` is not set)
- `POSTGRES_PORT` (default: `5432`; used when `DATABASE_URL` is not set)
- `POSTGRES_DB` (optional; enables database-backed endpoints when provided with `POSTGRES_USER` and `DATABASE_URL` is not set)
- `POSTGRES_USER` (optional; enables database-backed endpoints when provided with `POSTGRES_DB` and `DATABASE_URL` is not set)
- `POSTGRES_PASSWORD` (optional; used when `DATABASE_URL` is not set)
- `POSTGRES_SSLMODE` (default: `disable`; used when `DATABASE_URL` is not set)
- `R2_ENDPOINT` (required for raw download and upload endpoints)
- `R2_REGION` (default: `auto`)
- `R2_RAW_PUBLIC_BUCKET` (default: `refleks-raw-public`)
- `R2_LAB_PRIVATE_BUCKET` (default: `refleks-lab-private`, reserved for future premium/lab data flows)
- `R2_RAW_PUBLIC_BASE_URL` (optional, enables direct public URL responses for raw downloads)
- `R2_SIGNED_URL_TTL` (default: `15m`, used when public base URL is not configured)
- `R2_ACCESS_KEY_ID` (required for raw download and upload endpoints)
- `R2_SECRET_ACCESS_KEY` (required for raw download and upload endpoints)
- `R2_KEY_PREFIX` (default: `runs`)
- `RUNSYNC_MAX_FILE_BYTES` (default: `26214400`)
- `RUNSYNC_MAX_BULK_FILES` (default: `100`)
- `RUNSYNC_MAX_BULK_BYTES` (default: `262144000`)
- `RUNSYNC_MAX_MISSING_HASHES` (default: `1000`)

Endpoints activate in tiers based on configuration:

1. **Database configured** (`DATABASE_URL` or `POSTGRES_DB` + `POSTGRES_USER`) — enables benchmarks, leaderboards, scenarios, players, and run browse/detail.
2. **R2 credentials set** (`R2_ENDPOINT`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`) — additionally enables raw file download endpoints. Startup fails with an error if `RUNSYNC_ENABLED=true` but R2 is not fully configured.
3. **`RUNSYNC_ENABLED=true`** (requires R2) — additionally enables upload endpoints (sync, bulk sync, missing hashes).

When uploads are enabled (`RUNSYNC_ENABLED=true`), the API expects `players`, `scenarios`, and `runs` tables/indexes to already exist (for example created by the worker schema bootstrap).

When benchmark/leaderboard endpoints are enabled (database configuration is present), the API expects benchmark and leaderboard tables to exist (for example created by the worker schema bootstrap).

## Test

```bash
go test ./...
```
