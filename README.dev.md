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

# Snippets: list / search
```bash
curl -v 'http://localhost:8080/v1/snippets?q=example&limit=10'
```

# Snippets: get by id (public)
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

