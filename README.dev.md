# Sniply — Guia de Onboarding (Docker & Deploy)

Este documento explica **como rodar, desenvolver, migrar e fazer deploy** do Sniply usando Docker Compose.

Se você está chegando agora no projeto, **leia na ordem**.  
Este README foi escrito para evitar erros comuns e perda de contexto.

---

## TL;DR (para quem já é experiente)

- **Nunca rode `docker compose up` “no seco”** → use os aliases (`DEV` / `PROD`)
- **Migrações não rodam automaticamente** → precisam ser executadas explicitamente
- **Produção só expõe 80/443** → todo o resto é interno
- **Segredos nunca ficam no Git**
- **Traefik é o único ponto de entrada externo**

---

## Visão Geral da Arquitetura

O Sniply roda como **um único stack Docker**, composto por:

- **Traefik** – Reverse proxy e TLS (HTTPS)
- **API (Go)** – Serviço principal
- **PostgreSQL** – Banco local em container
- **Observabilidade** – Grafana, Prometheus, Loki, Tempo, OpenTelemetry

Todos os serviços, exceto o Traefik, rodam em redes internas.

```
Internet
   │
   ▼
Traefik (80/443)
   │
   ├── API
   │     └── Postgres
   │
   └── Grafana
         ├── Prometheus
         ├── Loki
         └── Tempo
``` 

---

## Estrutura do Repositório

```
.
├── compose.base.yml
├── compose.dev.yml
├── compose.prod.yml
├── Dockerfile
├── migrations/
├── grafana/
├── prometheus.yml
├── loki-config.yaml
├── tempo.yaml
├── otel-collector.yml
```

Pré-requisitos
---------------------------------------

**Desenvolvimento (local)**

Você precisa ter:

- Docker >= 24
- Docker Compose v2 (docker compose)
- Portas livres:
  - 8080 (API)
  - 5432 (Postgres)
  - 3001 (Grafana)
  - 9090, 3100, 3200, 4317 (telemetria)

**Produção**

- Servidor Linux com Docker + Docker Compose
- Portas 80 e 443 abertas
- DNS configurado:
  - api.DOMAIN
  - grafana.DOMAIN
---

## Convenção de Comandos

**⚠️ Regra importante
Nunca rode apenas docker compose up.
Sempre use compose.base.yml + um override.**

### Desenvolvimento
```
DEV="docker compose -f compose.base.yml -f compose.dev.yml"
```

### Produção
```
PROD="docker compose -f compose.base.yml -f compose.prod.yml"
```

---

## Desenvolvimento

### Subir ambiente
```
$DEV up -d --build
```
Isso sobe:
- API
- Postgres
- Observabilidade
- Traefik (mesmo em dev)

### Rodar migrações
```
$DEV --profile migrate up --abort-on-container-exit migrate
```

**⚠️ Importante
A API não roda migrações sozinha.
Sempre execute este comando após subir o ambiente pela primeira vez.**

### Logs
```
$DEV logs -f api
```

### Status
```
$DEV ps
```

### Desenvolvimento — Ciclo diário

**Rebuild + restart**
```
$DEV up -d --build 
```

**Reaplicar migrações (se mudou algo)**
```
$DEV --profile migrate up --abort-on-container-exit migrate
```

**Debug com Delve**
```
$DEV --profile debug up -d --build api-debug
```

**Logs do debug:**
```
$DEV logs -f api-debug
```

**Parar debug:**
```
$DEV stop api-debug
```

---

###  Reset do ambiente (DEV)

**Parar tudo**
```
$DEV down
```

**Reset completo (apaga banco e volumes)**
```
$DEV down -v
```

⚠️ Nunca use -v em produção

## Produção

**Criar arquivo de ambiente (fora do Git)**

No servidor:

```
sudo mkdir -p /etc/sniply
sudo nano /etc/sniply/sniply.env
sudo chmod 600 /etc/sniply/sniply.env
```

Exemplo:

```
DOMAIN=example.com
ACME_EMAIL=you@example.com

POSTGRES_PASSWORD=super-secure-password
JWT_SECRET=very-long-random-secret

GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=strong-password

# htpasswd -nb user 'password'
GRAFANA_BASIC_AUTH=user:$apr1$HASH
```

**Carregar variáveis no shell**
```
set -a; source /etc/sniply/sniply.env; set +a
```

### Subir stack
```
$PROD up -d
```

### Rodar migrações
```
$PROD --profile migrate up --abort-on-container-exit migrate
```

### Produção — Deploy / Atualização
```
git pull

set -a; source /etc/sniply/sniply.env; set +a
$PROD up -d --build
$PROD --profile migrate up --abort-on-container-exit migrate
$PROD up -d --build api
```

### Operação (Produção)

**Status**
```
$PROD ps
```

**Logs**
```
$PROD logs -f traefik
$PROD logs -f api
$PROD logs -f db
```

**Reiniciar API**
```
$PROD restart api
```

**Acessar Postgres**
```
$PROD exec db psql -U sniply -d sniply
```

---

Start the API debug
----------------------------------------------
```bash
docker compose --profile debug up --build api-debug
```
**setup.json for vscode:**
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
----------------------------------------------------------

**1. Install swag CLI:**

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

**2. Generate docs (writes to `src/docs`):**

```bash
cd src
go generate ./...
```

**3. Start the API and open the Swagger UI:**
```
http://localhost:8080/swagger/index.html
```

Telemetry 
---------------------------------

Observability (Grafana, Prometheus, Loki, Tempo, OTEL)
This stack is split into two docker-compose files and connected by a shared Docker network.

**High-level flow**
```
API (OTLP) --> OTel Collector --> Tempo (traces)
                           \\-> Loki (logs)
                           \\-> Prometheus (metrics scrape)
```


**What each file does (observability-related)**
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

**Signal mapping**
```
Traces:  API -> OTLP -> OTel Collector -> Tempo -> Grafana
Logs:    API -> OTLP -> OTel Collector -> Loki  -> Grafana
Metrics: API -> OTLP -> OTel Collector -> Prometheus -> Grafana
DB:      API -> OTLP -> Otel Collector -> Prometheus -> Grafana
```
**DB telemetry (traces + metrics)**
- Spans: each DB call (Query/QueryRow/Exec) creates a span named `DB <OPERATION>` with attributes like `db.system=postgresql` and `db.operation=SELECT/INSERT/...`.
- Metrics:
  - `sniply_db_query_duration_seconds` (histogram) with labels `db.system`, `db.operation`, `db.status`
  - `sniply_db_query_errors_total` (counter) with labels `db.system`, `db.operation`, `db.status`
- Implementation files:
  - `src/internal/db/telemetry.go` contains the instrumentation and metrics definitions.
  - `src/internal/db/base.go` wraps the queryer to emit spans/metrics.
  - `src/cmd/api/main.go` calls `db.InitTelemetry("sniply-api")` during startup.
  
**Key environment variable**
- `OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317` (set in `src/docker-compose.yml`)

**Notes**
- The API listens on the address configured in `cmd/api` (check the file if you need to change port).
- Use `Authorization: Bearer <token>` for protected endpoints. Get a token via `POST /v1/auth/login`.

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
