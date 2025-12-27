# Snippet Service

Serviço backend para **armazenamento e busca de snippets de texto**, com suporte a metadados (nome, linguagem, tags), visibilidade (`public` / `private`) e busca fuzzy.  
Este projeto é desenvolvido em **Go**, utilizando **PostgreSQL**, **Docker** e **migrations versionadas**.

---

## Visão Geral

O objetivo do projeto é fornecer um **endpoint simples e eficiente** para:

- Criar/Consultar snippets de texto
- Realizar buscas fuzzy por nome, metadados e conteúdo
- Gerenciar Usuarios e controloar visibilidade dos snippets
---

## Stack Tecnológica

- **Linguagem:** Go (stdlib + `chi`)
- **Roteamento HTTP:** `github.com/go-chi/chi`
- **Banco de dados:** PostgreSQL 16
- **Busca:** PostgreSQL Full-Text Search + `pg_trgm`
- **Migrações:** `golang-migrate`
- **Containerização:** Docker + Docker Compose
- **Observabilidade:** OpenTelemetry + Grafana (Prometheus, Loki, Tempo)
- **Documentação da API:** Swagger (OpenAPI)

---

## Arquitetura

```
API (Go + chi)
    |
    v
PostgreSQL < -- Migrate
```

Um container separado é utilizado exclusivamente para executar as **migrations**.

## Banco de Dados

### Entidades

#### users
- Representa usuários do sistema

#### snippets
- Conteúdo textual do snippet
- Metadados: linguagem e tags
- Visibilidade: `public` ou `private`
- Relacionado a um usuário

---

<img width="631" height="488" alt="image" src="https://github.com/user-attachments/assets/7280f7be-b0a3-4e87-8e32-5e8dcf081a5f" />

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
docker compose run --rm migrate up
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

### OpenAPI (Swagger UI)

```
GET /swagger/index.html
```

### Logar

Auth
- POST /v1/auth/login

Request body:
```json
{ "email": "you@example.com", "password": "secret" }
```

Response: JSON com `access_token` (Bearer token), `token_type` e `expires_at`.

### Users

- POST /v1/users — criar usuário (público)

Request body:
```json
{ "email": "you@example.com", "password": "secret" }
```

- Protected (requer `Authorization: Bearer <token>`)
  - GET /v1/users/me — obter perfil do usuário autenticado
  - PUT /v1/users/me — atualizar email/password do próprio usuário
  - DELETE /v1/users/me — deletar a própria conta

- Admin-only (requer role `admin`):
  - GET /v1/users — listar usuários
  - PUT /v1/users/{id} — atualizar usuário por id
  - DELETE /v1/users/{id} — deletar usuário por id
 
### Snippets

- GET /v1/snippets — listar snippets (protegido)
- GET /v1/snippets/{id} — obter snippet por id (protegido)
- POST /v1/snippets — criar snippet (protegido)
- PUT /v1/snippets/{id} — atualizar snippet (protegido)
- DELETE /v1/snippets/{id} — deletar snippet (protegido)

Query params para listagem:
- `q` — termo de busca (full-text / fuzzy)
- `creator` — filtrar por `creator_id` 
- `language` — filtrar por `language`
- `tags` — filtrar por `tags`
- `visibility` — filtrar por `visibility` (creator or user adm obrigatorio para privado)
- `limit`, `offset` — paginação

Exemplo de body para o criar

```json
{
  "name": "Exemplo",
  "content": "print('hello')",
  "language": "python",
  "tags": ["demo"],
  "visibility": "public"
}
```

**Para mais exemplos completos dos endpoints olhe o [guia DEV](./README.dev.md)**

## Segurança e autenticação
------------------------

- Autenticação: JWT (HS256). O serviço `auth.Service` emite tokens de acesso com `IssueAccessToken(userID, role)` e valida via `ValidateAccessToken`.
- Formato do header: `Authorization: Bearer <token>` — o middleware verifica e injeta o `user_id` e `role` no contexto da requisição.
- Permissões: a API usa o campo `role` do token para checar permissões; o helper `auth.IsAdmin(ctx)` retorna true para `role == "admin"`.
- Use HTTPS em produção e mantenha a chave secreta fora do código (variáveis de ambiente / secret manager).
- Tempo de vida do token é configurável em `auth.Service.AccessTTL`.

## Notas sobre comportamento atual
-----------------------------
- A rota `POST /v1/users` cria uma conta e a partir daí você pode chamar `/v1/auth/login` para obter o token.
- As migrations vivem em `migrations/` e devem ser aplicadas antes de subir a API.
- IDs usados pelo sistema seguem o formato `usr_*` e `snp_*`.

## Contribuindo e próximos passos
-----------------------------

- Tornar endpoints de snippets mais ricos em filtros
- Testes de integração e CI

---
