Development README for Sniply

This document explains how to run and develop Sniply locally.

Prerequisites
- Docker & Docker Compose
- Go 1.20+ (to build/run the API locally)
- Make (optional)

Repository layout
- `cmd/api` — entrypoint for the HTTP API
- `internal` — app internals (db, httpapi, auth, users, snippets)
- `migrations` — SQL migrations (golang-migrate)

Start the stack with Docker
1. Start only the DB:

```bash
cd src
docker compose up -d db
```

2. Run migrations (apply):

```bash
docker compose run --rm migrate \
  -source file:///migrations \
  -database=postgres://sniply:sniply@db:5432/sniply?sslmode=disable \
  up
```

3. Start the API container:

```bash
docker compose up -d api
```

4. Start the API debug:
```bash
docker compose --profile debug up --build api-debug
```
5. setup.json for vscode:
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Attach Go (Delve remote)",
      "type": "go",
      "request": "attach",
      "mode": "remote",
      "host": "127.0.0.1",
      "port": 40000,
      "apiVersion": 2,
      "cwd": "${workspaceFolder}/src",
      "substitutePath": [
        {
          "from": "${workspaceFolder}/src",
          "to": "/app"
        }
      ]
    }
  ]
}
```

OpenAPI (automatic with swaggo)
1. Install swag CLI:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

2. Generate docs (writes to `src/docs`):

```bash
cd src
go generate ./...
```

3. Start the API and open the Swagger UI:

```
http://localhost:8080/swagger/index.html
```

Run the API locally (without Docker)
1. Ensure PostgreSQL is running and reachable. Use the same DB from Docker or a local Postgres instance.
2. Export environment variables (example):

```bash
export DATABASE_URL=postgres://sniply:sniply@localhost:5432/sniply?sslmode=disable
export AUTH_SECRET=$(openssl rand -hex 32)
export AUTH_ISSUER=sniply
export AUTH_AUDIENCE=sniply
export AUTH_ACCESS_TTL=1h
```

3. Run migrations (locally with golang-migrate or reuse container):

```bash
# with the migrate container (recommended):
cd src
docker compose run --rm migrate \
  -source file:///migrations \
  -database="$DATABASE_URL" up

# or install golang-migrate locally and run:
# migrate -path ./migrations -database "$DATABASE_URL" up
```

4. Run the API:

```bash
cd src
go run ./cmd/api
```

Observability (Grafana, Prometheus, Loki, Tempo, OTEL)
This stack is split into two docker-compose files and connected by a shared Docker network.
Create the network once:

```bash
docker network create sniply-observability
```

Start the observability stack:

```bash
cd src/observability
docker compose up -d
```

Start the app stack (API + DB + migrate):

```bash
cd src
docker compose up -d
```

High-level flow
```
API (OTLP) --> OTel Collector --> Tempo (traces)
                           \\-> Loki (logs)
                           \\-> Prometheus (metrics scrape)
```

Docker network layout
```
docker-compose.yml (app)          docker-compose.yml (observability)
  api, db, migrate                grafana, prometheus, loki, tempo, otel-collector
  \\___________________________________________/
                  sniply-observability network
```

What each file does (observability-related)
- `src/cmd/api/main.go` initializes OTEL trace/metrics/logs, and DB telemetry. All signals use OTLP.
- `src/internal/httpapi/router.go` wires HTTP middlewares for traces, logs, and metrics.
- `src/internal/telemetry/trace.go` sets up OTLP trace exporter and tracer provider.
- `src/internal/telemetry/metrics.go` sets up OTLP metrics exporter and meter provider.
- `src/internal/telemetry/httpmetrics.go` defines HTTP metrics instruments (OTEL only).
- `src/internal/telemetry/logs.go` sets up OTLP log exporter and logger provider.
- `src/internal/telemetry/httplogs.go` emits structured HTTP logs with trace correlation.
- `src/internal/telemetry/otlp.go` centralizes OTLP endpoint resolution via envs.
- `src/internal/db/telemetry.go` adds DB spans + DB metrics (latency/errors).
- `src/internal/db/base.go` wraps the queryer to emit DB telemetry on each call.
- `src/observability/otel-collector.yml` receives OTLP and exports to Tempo (traces), Loki (logs), and a Prometheus scrape endpoint (metrics).
- `src/observability/prometheus.yml` scrapes only the collector metrics endpoint.
- `src/observability/tempo.yaml` configures Tempo storage and OTLP receiver.
- `src/observability/grafana/provisioning/datasources/datasources.yml` declares Prometheus, Loki, and Tempo datasources.
- `src/observability/grafana/dashboards/sniply-api.json` defines the HTTP and DB dashboards.
- `src/observability/docker-compose.yml` runs the observability stack and attaches the collector to the shared network.
- `src/docker-compose.yml` runs the app stack and configures OTEL endpoint + network.

Signal mapping
```
Traces:  API -> OTLP -> OTel Collector -> Tempo -> Grafana
Logs:    API -> OTLP -> OTel Collector -> Loki  -> Grafana
Metrics: API -> OTLP -> OTel Collector -> Prometheus -> Grafana
```

Key environment variable
- `OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317` (set in `src/docker-compose.yml`)

Notes
- The API listens on the address configured in `cmd/api` (check the file if you need to change port).
- Use `Authorization: Bearer <token>` for protected endpoints. Get a token via `POST /v1/auth/login`.

Rollback a migration
To undo the last applied migration:

```bash
cd src
docker compose run --rm migrate \
  -source file:///migrations \
  -database="$DATABASE_URL" down 1
```

To rollback a specific version use `goto` or run `down` stepwise.

Database seeding
- The project inserts a demo user `usr_demo` in `000001_init.up.sql`.
- For development you can update or insert additional data via psql or a small SQL file.

Running tests
- Add tests under `internal/*` as needed.
- Use `go test ./...` in the `src` folder to run unit tests.

Troubleshooting
- If migrations are not detected, verify filenames end with `.up.sql` / `.down.sql` (e.g., `000002_users_role.up.sql`).
- If a migration partially applied (client timeout), inspect DB (`schema_migrations`) and re-run `up`.

Examples for each endpoint (curl)
---------------------------------

Below are minimal curl examples for the main API endpoints. Replace `localhost:8080` and `$TOKEN` with your values.

# Health
```bash
curl -v http://localhost:8080/health
```

# Auth: login (obtain token)
```bash
curl -v -X POST http://localhost:8080/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@local","password":"x"}'
# Response: {"access_token":"...","token_type":"Bearer","expires_at":"..."}
```

# Users: create (public)
```bash
curl -v -X POST http://localhost:8080/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"new@local","password":"secret"}'
```

# Users: get current user (protected)
```bash
curl -v -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/users/me
```

# Users: update current user (protected)
```bash
curl -v -X PUT http://localhost:8080/v1/users/me \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"email":"me@local","password":"newpass"}'
```

# Users: delete current user (protected)
```bash
curl -v -X DELETE http://localhost:8080/v1/users/me \
  -H "Authorization: Bearer $TOKEN"
```

# Users: list (admin)
```bash
curl -v -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:8080/v1/users
```

# Snippets: list / search (protected)
```bash
curl -v 'http://localhost:8080/v1/snippets?q=example&limit=10'
```

# Snippets: get by id (protected)
```bash
curl -v http://localhost:8080/v1/snippets/snp_abc123
```

# Snippets: create (protected)
```bash
curl -v -X POST http://localhost:8080/v1/snippets \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"Example","content":"print(\"hi\")","language":"python","tags":["dev"],"visibility":"public"}'
```

# Snippets: update (protected)
```bash
curl -v -X PUT http://localhost:8080/v1/snippets/snp_abc123 \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"Updated","content":"x","language":"txt"}'
```

# Snippets: delete (protected)
```bash
curl -v -X DELETE http://localhost:8080/v1/snippets/snp_abc123 \
  -H "Authorization: Bearer $TOKEN"
```
