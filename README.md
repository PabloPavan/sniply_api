# Sniply

[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white)](#)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)](#)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-18%2B-336791?logo=postgresql&logoColor=white)](#)
[![Observability](https://img.shields.io/badge/Observability-Grafana%20%7C%20Prometheus%20%7C%20Loki%20%7C%20Tempo-orange)](#)
[![Logs](https://img.shields.io/badge/Logs-Alloy-6E46FF?logo=grafana&logoColor=white)](#)
[![License](https://img.shields.io/badge/License-MIT-green)](#)

## Overview

**Sniply** is a modern, production‑ready backend service for storing, searching, and managing **text and code snippets**.
It is designed with a strong focus on simplicity, performance, and clean architecture, making it suitable for personal tools, developer platforms, or integration into larger systems.

The project follows best practices commonly found in high‑quality open‑source backend repositories: clear domain boundaries, explicit configuration, containerized workflows, and first‑class API documentation.

At runtime, all traffic flows through Traefik (TLS + routing), the API talks to PostgreSQL, and observability is handled via OpenTelemetry + Grafana (Prometheus, Loki, Tempo).

---

## Key Features

* JWT‑based authentication
* User management (signup, login, profile management)
* Full CRUD for snippets
* Snippet visibility control (`public` / `private`)
* Advanced search using PostgreSQL **Full‑Text Search** and **pg_trgm** (fuzzy search)
* Filtering by language, tags, and visibility
* Pagination support
* OpenAPI / Swagger documentation
* Docker‑first development and deployment
* Ready for observability (metrics, logs, traces)

---

## Tech Stack

| Category         | Technology                              |
| ---------------- | --------------------------------------- |
| Language         | Go                                      |
| HTTP Router      | `chi`                                   |
| Database         | PostgreSQL 18                           |
| Search           | Full‑Text Search + `pg_trgm`            |
| Migrations       | `golang-migrate`                        |
| Containerization | Docker, Docker Compose                  |
| API Docs         | Swagger / OpenAPI                       |
| Proxy            | Traefik                                 |
| Observability    | OpenTelemetry + Grafana (Prometheus, Loki, Tempo, Alloy) |

For more details and flow diagrams, see [here](./README.dev.md)

---

## Getting Started

### Prerequisites

Make sure you have the following installed:

* Go 1.24 or newer
* Docker and Docker Compose
* PostgreSQL 18 (or run via Docker)

---

## Installation

### Clone the Repository

```bash
git clone https://github.com/PabloPavan/Sniply.git
cd Sniply
```

---

## Running with Docker (Recommended)

For development, debugging, and advanced Docker workflows (including debug images and troubleshooting), please refer to [README.dev.md](./README.dev.md).
That document describes the recommended and correct way to run Sniply with Docker in development environments.

## API Overview

### Health Check

```http
GET /health
```

Returns service health status.

---

### Authentication

#### Login

```http
POST /v1/auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password"
}
```

Response:

```json
{
  "access_token": "<jwt>",
  "token_type": "Bearer",
  "expires_at": "2025-01-01T00:00:00Z"
}
```

---

## Users

| Method | Endpoint       | Description         |
| ------ | -------------- | ------------------- |
| POST   | `/v1/users`    | Create a new user   |
| GET    | `/v1/users/me` | Get current user    |
| PUT    | `/v1/users/me` | Update current user |
| DELETE | `/v1/users/me` | Delete current user |

All `/me` endpoints require authentication.

---

## Snippets

| Method | Endpoint            | Description                |
| ------ | ------------------- | -------------------------- |
| GET    | `/v1/snippets`      | List snippets with filters |
| GET    | `/v1/snippets/{id}` | Get snippet by ID          |
| POST   | `/v1/snippets`      | Create a snippet           |
| PUT    | `/v1/snippets/{id}` | Update a snippet           |
| DELETE | `/v1/snippets/{id}` | Delete a snippet           |

### Query Parameters (List)

* `q` – search term (full‑text / fuzzy)
* `language` – filter by language
* `tags` – filter by tags
* `visibility` – `public` or `private`
* `limit` – pagination size
* `offset` – pagination offset

### Example – Create Snippet

```json
POST /v1/snippets
{
  "name": "Hello World",
  "content": "print('Hello, world!')",
  "language": "python",
  "tags": ["example", "demo"],
  "visibility": "public"
}
```

---

## Security Considerations

* All protected endpoints require a valid JWT
* Passwords are stored hashed
* Always run behind HTTPS in production
* Validate environment variables before startup

---

## Project Structure (High Level)

```text
src/
  cmd/                # Application entrypoints
  internal/           # Application core (domain, services, repositories)
  migrations/         # Database migrations
  observability/      # Grafana/Prometheus/Loki/Tempo configs
  compose.base.yml    # Base Docker Compose
  compose.dev.yml     # Development overrides
  compose.prod.yml    # Production overrides
  Dockerfile
```

---

## Contributing

Contributions are welcome.

Recommended workflow:

1. Fork the repository
2. Create a feature branch (`feature/my-feature`)
3. Commit with clear messages
4. Add tests when applicable
5. Open a Pull Request with a clear description

---

## Roadmap

* Automated tests (unit and integration)
* CI pipeline (lint, test, build)
* Rate limiting and API quotas
* Extended observability dashboards
* Deployment examples (Kubernetes, cloud providers)

---

## License

This project is licensed under the **MIT License**. See the `LICENSE` file for details.

---

## Author

Developed by **Pablo Pavan**.
