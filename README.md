# TV Time → Serializd

Migrate your watched TV shows from [TV Time](https://www.tvtime.com/) to [Serializd](https://serializd.com/).

This repo is a Go HTTP API that logs into both services, exports your TV Time library, resolves shows via TVDB/Wikidata/TMDB, and imports watch progress into Serializd. A frontend can drive the full flow through a single endpoint with live progress over SSE or polling.

## Features

- **One-shot migration** — `POST /migrate/init` with TV Time + Serializd credentials; credentials are validated before the job is queued
- **Live progress** — SSE stream (`GET /migrate/init/{id}/stream`) or polling (`GET /migrate/init/{id}`)
- **TV Time export** — optional standalone export flow (`POST /tvtime/login`, `POST /tvtime/export`, download JSON/CSV)
- **Show resolution** — TVDB lookup with Wikidata and optional TMDB fallback
- **Parallel pipeline** — configurable concurrency for gather, lookup, and import stages
- **Rate limiting** — per-IP limits on credential endpoints; shared outbound gates per external API

## Architecture

```
┌─────────────┐     ┌──────────────────────────────────────┐
│   Client    │────▶│  API (cmd/server)                    │
│  (browser)  │     │  Chi router · handlers · services    │
└─────────────┘     └──────────┬───────────────┬───────────┘
                               │               │
                    ┌──────────▼───┐   ┌───────▼────────┐
                    │  PostgreSQL  │   │     Redis      │
                    │  jobs, tokens│   │ sessions, cache│
                    └──────────────┘   └────────────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
         TV Time API     Serializd API    Wikidata / TMDB
```

| Component | Role |
|-----------|------|
| `cmd/server` | HTTP API |
| `cmd/migrate` | Database schema migrations (one-shot) |
| `internal/service` | Migration, export, and show lookup logic |
| `internal/platform/postgres` | SQL migrations and connection pool |
| `internal/cache` | Redis-backed sessions, progress, TMDB cache |

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose v2
- Go **1.25+** (only for local development outside Docker)

## Quick start (Docker)

**1. Create environment file**

```bash
./scripts/setup-env.sh
```

This copies `.env.example` → `.env` and generates Postgres/Redis passwords plus `TOKEN_ENCRYPTION_KEY`.

**2. Start infrastructure**

```bash
docker compose -f docker-compose.infra.yml up -d
```

**3. Run database migrations**

```bash
docker compose -f docker-compose.infra.yml -f docker-compose.migrate.yml run --rm migrate
```

**4. Start the API**

```bash
docker compose -f docker-compose.infra.yml -f docker-compose.yml up --build -d
```

The API listens on `http://localhost:8080` by default (`API_HOST_PORT` in `.env`).

**5. Verify**

```bash
curl http://localhost:8080/health
```

### Useful commands

| Task | Command |
|------|---------|
| Follow API logs | `docker compose -f docker-compose.infra.yml -f docker-compose.yml logs -f api` |
| Rebuild on code changes | `docker compose -f docker-compose.infra.yml -f docker-compose.yml watch` |
| Stop API (keep DB/Redis) | `docker compose -f docker-compose.infra.yml -f docker-compose.yml down` |
| Stop everything | `docker compose -f docker-compose.infra.yml down` |
| Reset all data | `docker compose -f docker-compose.infra.yml down -v` |
| Roll back one migration | `docker compose -f docker-compose.infra.yml -f docker-compose.migrate.yml run --rm migrate down` |

## API overview

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Liveness + Postgres/Redis checks |
| `POST` | `/tvtime/login` | Store encrypted TV Time session |
| `POST` | `/tvtime/export` | Start TV Time library export |
| `GET` | `/tvtime/export/{id}` | Export status |
| `GET` | `/tvtime/export/{id}/download` | Download export file |
| `POST` | `/serializd/login` | Store encrypted Serializd session |
| `POST` | `/migrate/init` | Start full migration job |
| `GET` | `/migrate/init/{id}` | Poll migration progress |
| `GET` | `/migrate/init/{id}/stream` | SSE migration progress |

**Start a migration:**

```bash
curl -X POST http://localhost:8080/migrate/init \
  -H 'Content-Type: application/json' \
  -d '{
    "tvtime_email": "you@example.com",
    "tvtime_password": "secret",
    "serializd_email": "you@example.com",
    "serializd_password": "secret"
  }'
```

Response: `{ "id": "<job-uuid>" }` — then poll or stream progress with that ID.

For frontend integration (request/response shapes, stepper UI, error codes), see [docs/frontend-migrate-api.md](docs/frontend-migrate-api.md).

OpenAPI specs live in [docs/swagger.yaml](docs/swagger.yaml) and [docs/swagger.json](docs/swagger.json).

## Configuration

Copy [`.env.example`](.env.example) or run `./scripts/setup-env.sh`. Key variables:

| Variable | Required | Description |
|----------|----------|-------------|
| `TOKEN_ENCRYPTION_KEY` | yes | AES-256 key (32 bytes, base64) for stored tokens |
| `DATABASE_URL` | yes | Postgres connection string |
| `REDIS_URL` | yes | Redis connection string |
| `CORS_ALLOWED_ORIGINS` | no | Comma-separated frontend origins for browser access |
| `TMDB_API_KEY` | no | Improves show matching when TVDB/Wikidata lookup fails |
| `TVTIME_GATHER_CONCURRENCY` | no | Parallel TV Time fetches (default 24) |
| `MIGRATE_LOOKUP_CONCURRENCY` | no | Parallel show lookups (default 32) |
| `MIGRATE_IMPORT_CONCURRENCY` | no | Parallel Serializd imports (default 8) |

See `.env.example` for the full list including outbound rate limits and credential rate limits.

## Local development (without Docker for the API)

If Postgres and Redis are already running (e.g. via `docker-compose.infra.yml`):

```bash
cp .env.example .env   # or ./scripts/setup-env.sh
# Point DATABASE_URL / REDIS_URL at localhost ports from .env

go run ./cmd/migrate
go run ./cmd/server
```

Run tests and lint:

```bash
go test ./...
golangci-lint run       # requires golangci-lint installed locally
```

CI runs both on every push/PR (see [`.github/workflows/ci.yml`](.github/workflows/ci.yml)).

## Project layout

```
cmd/
  server/          API entrypoint
  migrate/         DB migration CLI
internal/
  handler/         HTTP handlers
  service/         Business logic (migrate, export, lookup)
  tvtime/          TV Time API client
  serializd/       Serializd API client
  platform/        Postgres + Redis
  cache/           Redis caches
  repository/      SQL repositories
docker-compose*.yml
scripts/setup-env.sh
docs/              API specs and frontend integration guide
```

## Security notes

- Credentials are sent once in the migration request body and are **not** returned in progress responses.
- TV Time and Serializd tokens are encrypted at rest with `TOKEN_ENCRYPTION_KEY`.
- Credential endpoints are rate-limited per client IP.
- Set `CORS_ALLOWED_ORIGINS` explicitly before exposing the API to a browser frontend.
- Do not commit `.env` — it is listed in `.gitignore`.

## License

See repository license file if present.
