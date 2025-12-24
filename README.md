# Snippet Service

Serviço backend para **armazenamento e busca de snippets de texto**, com suporte a metadados (nome, linguagem, tags), visibilidade (`public` / `private`) e busca fuzzy.  
Este projeto é desenvolvido em **Go**, utilizando **PostgreSQL**, **Docker** e **migrations versionadas**.

---

## Visão Geral

O objetivo do projeto é fornecer um **endpoint simples e eficiente** para:

- Criar snippets de texto
- Consultar snippets públicos
- Evoluir futuramente para autenticação e snippets privados
- Realizar buscas fuzzy por nome, metadados e conteúdo

O foco inicial é um **MVP funcional**, acessado via **Insomnia** ou `curl`.

---

## Stack Tecnológica

- **Linguagem:** Go (stdlib + `chi`)
- **Roteamento HTTP:** `github.com/go-chi/chi`
- **Banco de dados:** PostgreSQL 16
- **Busca:** PostgreSQL Full-Text Search + `pg_trgm`
- **Migrações:** `golang-migrate`
- **Containerização:** Docker + Docker Compose

---

## Arquitetura

```
API (Go + chi)
    |
    v
PostgreSQL
```

Um container separado é utilizado exclusivamente para executar as **migrations**.


## Banco de Dados

### Entidades

#### users
- Representa usuários do sistema
- No MVP existe um usuário fixo `usr_demo`

#### snippets
- Conteúdo textual do snippet
- Metadados: linguagem e tags
- Visibilidade: `public` ou `private`
- Relacionado a um usuário

---

## Busca

- **Fuzzy search no nome:** `pg_trgm`
- **Busca textual:** Full-Text Search do PostgreSQL
- Campos indexados: `name`, `content`, `tags`

---

## Migrations

O schema do banco é controlado por **migrations versionadas** usando `golang-migrate`.

- Cada alteração estrutural gera uma nova migration
- A API nunca cria ou altera tabelas diretamente
- O histórico é mantido na tabela `schema_migrations`

---

## Docker

### Serviços

| Serviço | Função |
|-------|-------|
| db | PostgreSQL |
| migrate | Executa migrations |
| api | Servidor HTTP Go |

### Subir o banco

```
docker compose up -d db
```

### Rodar migrations

```bash
docker compose run --rm migrate \
  -source file:///migrations \
  -database=postgres://snippet:snippet@db:5432/snippet?sslmode=disable \
  up
```

### Subir a API

```
docker compose up -d api
```

---

## API

### Health Check

```
GET /health
```

### Criar snippet

```
POST /v1/snippets
```

Body:
```json
{
  "name": "Exemplo",
  "content": "print('hello')",
  "language": "python",
  "tags": ["demo"],
  "visibility": "public"
}
```

---

### Buscar snippet público

```
GET /v1/snippets/{id}
```

Somente snippets públicos são retornados no MVP atual.

### Listar snippets (ListAll)

```
GET /v1/snippets
```

# Sniply — API de snippets

API simples para armazenar, buscar e gerenciar snippets de texto. Este README foi atualizado para refletir o estado atual da API (rotas, exemplos e segurança).

Tecnologias principais: Go, chi, PostgreSQL, Docker, golang-migrate.

Índice
- Visão geral
- Como rodar (Docker + migrations)
- Endpoints (exemplos curl)
- Autenticação e segurança
- Notas adicionais

---

Visão geral
-----------

O serviço expõe uma API REST em `/v1` com os recursos principais:
- `/v1/snippets` — CRUD de snippets (criar/listar/consultar/atualizar/excluir)
- `/v1/users` — criar conta pública e endpoints protegidos para autogerenciamento e administração
- `/v1/auth/login` — gera token de acesso (JWT)

As rotas públicas e protegidas estão descritas abaixo com exemplos.

Como rodar (Docker)
-------------------

Subir apenas o banco:

```bash
docker compose up -d db
```

Rodar migrations (use o banco definido em `docker-compose.yml`):

```bash
docker compose run --rm migrate \
  -source file:///migrations \
  -database=postgres://sniply:sniply@db:5432/sniply?sslmode=disable \
  up
```

Subir a API:

```bash
docker compose up -d api
```

API (endpoints e exemplos)
--------------------------

Base URL: `http://localhost:8080` (ajuste conforme `docker-compose.yml`).

Health
- GET /health

Exemplo:
```bash
curl http://localhost:8080/health
```

Auth
- POST /v1/auth/login

Request body:
```json
{ "email": "you@example.com", "password": "secret" }
```

Response: JSON com `access_token` (Bearer token), `token_type` e `expires_at`.

Exemplo:
```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"secret"}'
```

Users
- POST /v1/users — criar usuário (público)

Request body:
```json
{ "email": "you@example.com", "password": "secret" }
```

Exemplo:
```bash
curl -X POST http://localhost:8080/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"secret"}'
```

- Protected (requer `Authorization: Bearer <token>`)
  - GET /v1/users/me — obter perfil do usuário autenticado
  - PUT /v1/users/me — atualizar email/password do próprio usuário
  - DELETE /v1/users/me — deletar a própria conta

- Admin-only (requer role `admin`):
  - GET /v1/users — listar usuários
  - PUT /v1/users/{id} — atualizar usuário por id
  - DELETE /v1/users/{id} — deletar usuário por id

Exemplo (usar token obtido via `/v1/auth/login`):
```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/users/me
```

Snippets
- GET /v1/snippets — listar snippets (públicos + filtros)
- GET /v1/snippets/{id} — obter snippet por id
- POST /v1/snippets — criar snippet (protegido)
- PUT /v1/snippets/{id} — atualizar snippet (protegido)
- DELETE /v1/snippets/{id} — deletar snippet (protegido)

Query params para listagem:
- `q` — termo de busca (full-text / fuzzy)
- `creator` — filtrar por `creator_id`
- `limit`, `offset` — paginação

Criar snippet (exemplo):
```bash
curl -X POST http://localhost:8080/v1/snippets \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"Exemplo","content":"print(\"hello\")","language":"python","tags":["demo"],"visibility":"public"}'
```

Obter snippet público:
```bash
curl http://localhost:8080/v1/snippets/snp_abc123
```

Atualizar snippet:
```bash
curl -X PUT http://localhost:8080/v1/snippets/snp_abc123 \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"Atualizado","content":"x","language":"txt"}'
```

Deletar snippet:
```bash
curl -X DELETE http://localhost:8080/v1/snippets/snp_abc123 \
  -H "Authorization: Bearer $TOKEN"
```

Segurança e autenticação
------------------------

- Autenticação: JWT (HS256). O serviço `auth.Service` emite tokens de acesso com `IssueAccessToken(userID, role)` e valida via `ValidateAccessToken`.
- Formato do header: `Authorization: Bearer <token>` — o middleware verifica e injeta o `user_id` e `role` no contexto da requisição.
- Permissões: a API usa o campo `role` do token para checar permissões; o helper `auth.IsAdmin(ctx)` retorna true para `role == "admin"`.
- Use HTTPS em produção e mantenha a chave secreta fora do código (variáveis de ambiente / secret manager).
- Tempo de vida do token é configurável em `auth.Service.AccessTTL`.

Observações sobre segurança prática
- Nunca exponha o segredo JWT no repositório.
- Valide senhas com um algoritmo de hash forte (bcrypt é usado nos handlers).
- Proteja endpoints administrativos e remova contas demo em produção.

Notas sobre comportamento atual
-----------------------------
- A rota `POST /v1/users` cria uma conta e a partir daí você pode chamar `/v1/auth/login` para obter o token.
- As migrations vivem em `migrations/` e devem ser aplicadas antes de subir a API.
- IDs usados pelo sistema seguem o formato `usr_*` e `snp_*`.

Contribuindo e próximos passos
-----------------------------

- Adicionar OpenAPI/Swagger
- Tornar endpoints de snippets mais ricos em filtros
- Testes de integração e CI

---
