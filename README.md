# Beans News API & MCP

**Version:** 0.8

Beans is an intelligent news and blogs aggregation service. This read-only HTTP API exposes curated articles from RSS feeds with semantic vector search, tag filters, trend analytics, and propagation tracking (cross-publisher coverage and social mentions).

---

## Data model

A **bean** is the basic unit of content — a single article or post indexed by its canonical URL. Each bean carries publishing metadata, NLP-derived classification (`categories`, `entities`, `regions`, `sentiments`), and an optional `embedding` vector for semantic search.

| Concept | Description |
|---------|-------------|
| **bean** | One article/post row in `beans`, keyed by `url` |
| **publisher** | Source metadata in `publishers`, keyed by `source` (domain id) |
| **chatter** | A social/forum mention of a bean URL in `chatters` |
| **related bean** | A link between a seed URL and a related URL in `related_beans` (same story elsewhere) |
| **trend aggregate** | Materialized view combining chatter and related counts into `trend_score` |

Article list endpoints return JSON shaped by the types in [`beansack/types.go`](beansack/types.go). Stable response field names use API-oriented JSON tags (for example `published_at`, `content_type`, `source_base_url`).

### Database schema

The API expects a PostgreSQL database initialized by the Beans ingestion pipeline ([`pycoffeemaker/pybeansack/pgsack.sql`](https://github.com/soumitsalman/pycoffeemaker/blob/main/pybeansack/pgsack.sql)).

**Extensions**

| Extension | Purpose |
|-----------|---------|
| `vector` | pgvector for bean embeddings (`vector(384)`) |
| `pg_trgm` | Trigram support (used by the pipeline) |

#### Table: `beans`

Primary store for all indexed articles and posts.

| Column | Type | Notes |
|--------|------|-------|
| `url` | `VARCHAR` | Primary key |
| `kind` | `VARCHAR` | Content type (`news`, `blog`, etc.) |
| `title`, `author`, `source`, `image_url` | `VARCHAR` | Core metadata |
| `created` | `TIMESTAMP` | Original publish time |
| `collected` | `TIMESTAMP` | Ingest timestamp |
| `summary`, `content` | `TEXT` | Body fields; `content` optional via `full_content` |
| `embedding` | `vector(384)` | Semantic search (`q` + `acc`) |
| `categories`, `sentiments`, `regions`, `entities` | `VARCHAR[]` | NLP classification |
| `tags` | `tsvector` | Generated from regions + entities + categories |

**Indexes:** `kind`, `created`, `source`, GIN on array fields and `tags`, HNSW on `embedding`.

#### Table: `publishers`

| Column | Type | Notes |
|--------|------|-------|
| `source` | `VARCHAR` | Primary key; matches `beans.source` |
| `base_url`, `site_name`, `description`, `favicon`, `rss_feed` | | Display metadata |

#### Table: `chatters`

Social/forum mentions of bean URLs.

| Column | Type | Notes |
|--------|------|-------|
| `chatter_url` | `VARCHAR` | Share/post URL |
| `url` | `VARCHAR` | Referenced bean URL |
| `source`, `forum` | `VARCHAR` | Platform and community |
| `collected` | `TIMESTAMP` | Observation time |
| `likes`, `comments`, `subscribers`, `shares` | `INTEGER` | Engagement metrics |

#### Table: `related_beans`

Directed edges: seed `url` → `related_url` (same story on another outlet).

| Column | Type | Notes |
|--------|------|-------|
| `url` | `VARCHAR` | Seed article |
| `related_url` | `VARCHAR` | Related article |

**Unique constraint:** `(url, related_url)`.

#### Materialized view: `trend_aggregates`

Rolls up chatter stats and related counts per bean URL; exposes `likes`, `comments`, `shares`, `related`, `trend_score` used by trending routes.

```mermaid
erDiagram
    publishers ||--o{ beans : "source"
    beans ||--o{ related_beans : "url"
    beans ||--o{ related_beans : "related_url"
    beans ||--o{ chatters : "url"
    beans ||--o| trend_aggregates : "url"

    publishers {
        varchar source PK
        varchar base_url
        varchar site_name
    }

    beans {
        varchar url PK
        varchar kind
        timestamp created
        varchar source
        vector embedding
    }

    related_beans {
        varchar url
        varchar related_url
    }

    chatters {
        varchar chatter_url
        varchar url
        timestamp collected
    }
```

Interactive API documentation is available at `/swagger/index.html` after starting the server (generated from [`router/routes.go`](router/routes.go) via [swaggo](https://github.com/swaggo/swag)).

---

## General notes

- Success responses use `200` or `204` (empty result set). Errors use `400`, `401`, `429`, or `500` with a JSON body like `{ "error": "..." }`.
- Authentication is optional. When `API_KEYS` is set, each request must include a matching header (see [Configuration](#configuration)).
- Concurrency is limited by an in-memory queue; excess requests wait rather than fail immediately.
- Protected routes accept `GET`, `POST` (propagation only), and `OPTIONS` (CORS enabled).

### Pagination and batch limits

| Parameter | Applies to | Default | Max |
|-----------|------------|---------|-----|
| `limit` | All paginated list routes | 16 | 128 |
| `offset` | All paginated list routes | 0 | — |
| `urls` | `/articles/propagation` | — | 128 seed URLs per request |

List endpoints: `limit` 1–128, `offset` ≥ 0. Propagation always returns `200` with one object per input URL (empty `coverage` / `mentions` arrays when nothing is found).

### Article filtering tips

- **`tags`** — case/whitespace-insensitive full-text search across categories, regions, and entities (AND combination). Good starting point.
- **`categories`**, **`regions`**, **`entities`** — precise array filters (OR within each dimension; case/whitespace-sensitive). Use `/tags/*` endpoints to discover values.
- **`q` + `acc`** — semantic vector search; requires a running embedder service.

---

## Quick start

```bash
BASE_URL="http://localhost:8080"
API_KEY="my-secret"                              # only if API key enforcement is enabled
AUTH='-H "X-API-KEY: '"$API_KEY"'"'             # omit when API_KEYS is unset
```

### Health check

```bash
curl -s "$BASE_URL/health" | jq
```

```json
{ "status": "alive" }
```

---

## Routes

### Tag lists

Discover filter values for article endpoints.

```bash
curl -s $AUTH "$BASE_URL/tags/categories?limit=20&offset=0" | jq
curl -s $AUTH "$BASE_URL/tags/entities?limit=20" | jq
curl -s $AUTH "$BASE_URL/tags/regions?limit=20" | jq
```

| | |
|---|---|
| **Method** | `GET` |
| **Paths** | `/tags/categories`, `/tags/entities`, `/tags/regions` |
| **Description** | Paginated lists of unique categories, named entities, or geographic regions extracted from articles. |

**Query parameters**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 16 | Page size (1–128) |
| `offset` | int | 0 | Items to skip |

**Response:** JSON array of strings, e.g. `["Artificial Intelligence", "Cybersecurity", "Politics"]`.

---

### Sources (publishers)

```bash
curl -s $AUTH "$BASE_URL/sources?sources=techcrunch.com,theverge.com&limit=5" | jq
```

| | |
|---|---|
| **Method** | `GET` |
| **Path** | `/sources` |
| **Description** | Publisher metadata for one or more source IDs. |

**Query parameters**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `sources` | CSV strings | — | Source IDs to fetch (recommended) |
| `limit` | int | 16 | Page size (1–128) |
| `offset` | int | 0 | Pagination offset |

**Response:** JSON array of publisher objects. Example element:

```json
{
  "source": "techcrunch.com",
  "source_base_url": "https://techcrunch.com",
  "source_site_name": "TechCrunch",
  "source_description": "Startup and technology news",
  "source_favicon": "https://techcrunch.com/favicon.ico"
}
```

---

### Articles — shared query parameters

These apply to `/articles/search`, `/articles/latest`, `/articles/trending`, and `/articles/top-headlines`.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `q` | string | — | Semantic search query (max 512 chars; requires embedder) |
| `acc` | float | 0.75 | Minimum embedding similarity for `q` (0.0–1.0; higher = stricter) |
| `content_type` | string | — | Filter: `news` or `blog` |
| `urls` | CSV strings | — | Fetch specific article URLs directly |
| `tags` | CSV strings | — | Tag search across categories/regions/entities (AND) |
| `categories` | CSV strings | — | Category filter (OR) |
| `regions` | CSV strings | — | Region filter (OR) |
| `entities` | CSV strings | — | Entity filter (OR) |
| `sources` | CSV strings | — | Publisher source filter (OR) |
| `from` | date | varies | `YYYY-MM-DD` date filter (see each route) |
| `full_content` | bool | `false` | Include full article body (large payload) |
| `limit` | int | 16 | Page size (1–128) |
| `offset` | int | 0 | Pagination offset |

---

### Search articles

```bash
curl -s $AUTH \
  "$BASE_URL/articles/search?q=artificial+intelligence&acc=0.8&limit=5" | jq
```

Tag filter example:

```bash
curl -s $AUTH \
  "$BASE_URL/articles/search?tags=cybersecurity,Artificial+Intelligence&limit=5" | jq
```

| | |
|---|---|
| **Method** | `GET` |
| **Path** | `/articles/search` |
| **Description** | Search the full database sorted by relevance. At least one of `q`, `tags`, `categories`, `regions`, `entities`, or `urls` is required. Heavier than time-windowed routes. |

**Response:** JSON array of article aggregates with publisher info and trend fields. Example element:

```json
{
  "url": "https://example.com/story",
  "content_type": "news",
  "title": "Breaking AI News",
  "summary": "A short summary...",
  "author": "Jane Doe",
  "source": "example.com",
  "published_at": "2024-02-28T12:34:56Z",
  "categories": ["Artificial Intelligence"],
  "entities": ["OpenAI"],
  "regions": ["US"],
  "source_base_url": "https://example.com",
  "source_site_name": "Example News",
  "likes": 12,
  "comments": 4,
  "shares": 2,
  "related": 3,
  "trend_score": 1.42
}
```

---

### Latest articles

```bash
curl -s $AUTH \
  "$BASE_URL/articles/latest?categories=Artificial+Intelligence&from=2026-06-01&limit=5" | jq
```

| | |
|---|---|
| **Method** | `GET` |
| **Path** | `/articles/latest` |
| **Description** | Most recently published articles, sorted by publish date (newest first). When `from` is omitted, defaults to roughly the last 7 days. |

**Response:** JSON array of article objects (core fields; trend/publisher fields omitted unless queried via search/trending).

---

### Trending articles

```bash
curl -s $AUTH \
  "$BASE_URL/articles/trending?from=2026-06-01&limit=10" | jq
```

| | |
|---|---|
| **Method** | `GET` |
| **Path** | `/articles/trending` |
| **Description** | Articles ranked by `trend_score` (engagement + cross-publisher coverage + recency). When `from` is omitted, defaults to roughly the last 7 days on the trending window. |

**Response:** JSON array including `trend_score`, `likes`, `comments`, `shares`, and `related`.

---

### Top headlines

```bash
curl -s $AUTH "$BASE_URL/articles/top-headlines?limit=10" | jq
```

| | |
|---|---|
| **Method** | `GET` |
| **Path** | `/articles/top-headlines` |
| **Description** | Top trending headlines from the past 24 hours, ranked by `trend_score`. Same filters as trending; time window is fixed to the last day. |

---

### Article propagation

Track how a story spread to other publishers and social platforms.

```bash
# GET — URLs as query param (CSV)
curl -s $AUTH \
  "$BASE_URL/articles/propagation?urls=https://example.com/story-a,https://example.com/story-b" | jq

# POST — URLs as JSON body
curl -s $AUTH -X POST "$BASE_URL/articles/propagation" \
  -H "Content-Type: application/json" \
  -d '{"urls":["https://example.com/story-a","https://example.com/story-b"]}' | jq
```

| | |
|---|---|
| **Methods** | `GET`, `POST` |
| **Path** | `/articles/propagation` |
| **Description** | For each input URL, returns publisher **coverage** (related articles from `related_beans`) and social **mentions** (from `chatters`). |

**Input**

| Method | Input | Max |
|--------|-------|-----|
| `GET` | `urls` query param (CSV) | 128 URLs |
| `POST` | JSON body `{ "urls": [...] }` | 128 URLs |

No overlap: GET uses query only; POST uses body only.

**Response:** JSON array, one object per input URL (always `200`):

```json
[
  {
    "url": "https://example.com/story-a",
    "coverage": [
      {
        "url": "https://other.com/same-story",
        "created": "2025-06-20T10:00:00Z",
        "source": "other.com",
        "site_name": "Other News"
      }
    ],
    "mentions": [
      {
        "share_url": "https://reddit.com/r/tech/comments/abc",
        "source": "reddit",
        "forum": "r/technology",
        "observed": "2025-06-21T08:00:00Z",
        "comments": 42,
        "likes": 120
      }
    ]
  }
]
```

---

### Other endpoints

| Path | Description |
|------|-------------|
| `GET /health` | Liveness probe |
| `GET /favicon.ico` | Static favicon |
| `GET /swagger/index.html` | Swagger UI (OpenAPI spec from `docs/`) |

---

## Development

### Prerequisites

- Go 1.25+
- PostgreSQL with the Beans schema (`pgsack.sql`) and pgvector extension
- A TEI-compatible embedding service (gRPC; see `docker-compose.yml` as `tei`)

### Build and run

```bash
go mod download
go build -o beansapi .

# or
make run
```

```bash
export PORT=8080
export PG_CONNECTION_STRING="postgres://user:pass@localhost:5432/beans?sslmode=disable"
export EMBEDDER_BASE_URL="localhost:10000"
export EMBEDDER_API_KEY=""              # optional
export EMBEDDER_MODEL=""                # optional
export MAX_CONCURRENT_REQUESTS=512      # optional; defaults to 512 in router
export API_KEYS="X-API-KEY=secret"      # optional; semicolon-separated Header=Value pairs

./beansapi
```

Environment variables can also be loaded from a `.env` file (see `main.go` and `docker-compose.yml`).

### Docker Compose

```bash
docker compose up --build
```

Starts the API on port `8080` and a local TEI embedder on port `10000`. Place secrets and the database connection string in `.env`.

### Regenerate OpenAPI docs

After changing swagger annotations in `router/`:

```bash
go run github.com/swaggo/swag/cmd/swag@v1.16.4 init -g main.go -o docs
```

### Tests

Integration tests live under `tests/` and require a reachable database (configured via `.env`):

```bash
go test ./tests/...
```

Router tests (`tests/router_test.go`) exercise pagination validation and propagation limits via `httptest`. Stress tests hit a live server at `STRESS_BASE_URL` (default `http://localhost:8080`).

---

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `PG_CONNECTION_STRING` | yes | — | PostgreSQL DSN |
| `EMBEDDER_BASE_URL` | yes | — | gRPC host:port of the embedding service (see `nlp/embedder.go`) |
| `EMBEDDER_API_KEY` | no | — | API key for the embedder |
| `EMBEDDER_MODEL` | no | — | Model name passed to the embedder |
| `PORT` | no | `8080` | HTTP listen port |
| `MAX_CONCURRENT_REQUESTS` | no | `512` | Max in-flight protected requests |
| `API_KEYS` | no | — | Semicolon-separated `Header=Value` pairs; when unset, auth is disabled |

**API key format:** `API_KEYS="X-API-KEY=secret;Authorization=Bearer token"`

When `API_KEYS` is empty, the server accepts unauthenticated requests. Set it before exposing the service publicly.

---

## Project layout

| Path | Purpose |
|------|---------|
| `main.go` | Entry point, env loading, server startup |
| `router/` | HTTP routes, swagger annotations, request types |
| `beansack/` | PostgreSQL access layer and response types (`Bean`, `Publisher`, `PropagationResult`) |
| `nlp/` | Remote embedder client (gRPC / TEI) |
| `docs/` | Generated OpenAPI spec (`swag init`) |
| `tests/` | DB integration, router, and stress tests |

---

## License

MIT — see [`LICENSE`](LICENSE).
