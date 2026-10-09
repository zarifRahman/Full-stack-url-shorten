# URL Shortener + Analytics — Build Plan

## Context
User wants a portfolio-grade project using **Go backend + Next.js frontend + Docker**.
Chosen idea: URL shortener with an analytics dashboard. Greenfield — target folder
`~/Documents/my-project/url-shortener` already created and empty. Goal: a working MVP
that runs end-to-end with `docker compose up`.

## Scope (MVP)
- Shorten a URL → get a short code + short link.
- Redirect `/{code}` → original URL, logging each click (timestamp, referer, user-agent).
- Dashboard: list links with click counts; per-link analytics (total, clicks-over-time chart, recent clicks).

## Lazy defaults (upgrade paths noted)
- Go **stdlib `net/http`** routing (Go 1.22 pattern mux). No web framework. → add chi/gin only if routing grows.
- **Postgres only**, no Redis for MVP. → add Redis cache for hot-code lookups when read load demands.
- Next.js **app router in plain JS** (no TS config churn). → convert to TS later if desired.
- Schema via `CREATE TABLE IF NOT EXISTS` on startup. No migration tool. → add goose/atlas when schema evolves.
- Click logging fire-and-forget goroutine (a dropped click on crash is acceptable). → make transactional if clicks are billing-critical.
- Permissive CORS (`*`) for dev. → lock to frontend origin for prod.

## Structure
```
url-shortener/
  docker-compose.yml          # postgres + backend + frontend
  README.md
  backend/
    go.mod                    # module urlshortener, pgx/v5
    main.go                   # pool + schema + routes + handlers
    main_test.go              # shortCode length/charset/uniqueness check
    Dockerfile                # multi-stage; go mod tidy + build static binary
    .dockerignore
  frontend/
    package.json              # next, react, react-dom, recharts
    next.config.js
    Dockerfile                # node:20-alpine, npm run dev
    .dockerignore
    lib/api.js                # API base = NEXT_PUBLIC_API_URL
    app/layout.js
    app/globals.css
    app/page.js               # create form + links list ("use client")
    app/analytics/[code]/page.js  # stats + recharts line chart + recent table
```

## Backend API (`:8080`)
- `GET  /health` → ok
- `POST /api/links` `{url}` → `{code, url, short_url}` (prepend `https://` if no scheme; reject empty host)
- `GET  /api/links` → `[{code, url, clicks, short_url, created_at}]`
- `GET  /api/links/{code}/stats` → `{code, url, total, daily:[{date,count}], recent:[{created_at,referer,user_agent}]}`
- `GET  /{code}` → log click (goroutine), 302 redirect; 404 if unknown

Code gen: 6-char base62 via `crypto/rand`, unique constraint + retry on collision.

## Schema
- `links(id BIGSERIAL pk, code TEXT UNIQUE, url TEXT, created_at TIMESTAMPTZ default now())`
- `clicks(id BIGSERIAL pk, link_id BIGINT fk→links ON DELETE CASCADE, referer TEXT, user_agent TEXT, created_at TIMESTAMPTZ default now())`
- `INDEX idx_clicks_link ON clicks(link_id)`

Daily agg: `date_trunc('day', created_at)` grouped, last 30 days.

## Data flow
1. User submits long URL in frontend form → `POST /api/links` → backend inserts `links` row with generated code → returns `short_url`.
2. User shares/visits `http://localhost:8080/<code>` → backend looks up `url` by code → spawns goroutine to insert a `clicks` row → `302` redirect to original URL.
3. Dashboard (`/`) calls `GET /api/links` → renders table (code, original, clicks, links to analytics).
4. Analytics page (`/analytics/<code>`) calls `GET /api/links/<code>/stats` → renders total, recharts line chart (daily), recent-clicks table.

## File-by-file responsibilities

### backend/
- **go.mod** — `module urlshortener`, `go 1.22`, require `github.com/jackc/pgx/v5`.
- **main.go**
  - `main()`: read `DATABASE_URL`/`BASE_URL`, connect pool with 10× retry loop (Postgres boot race), `Exec` schema, register routes on `http.ServeMux`, `ListenAndServe(":8080")`.
  - `withCORS(h)`: middleware — `Access-Control-Allow-Origin: *`, methods, headers; short-circuit `OPTIONS`.
  - `shortCode(n)`: `crypto/rand` → base62 string (alphabet `[A-Za-z0-9]`).
  - `normalizeURL(s)`: prepend `https://` when no scheme; parse with `net/url`; error on empty host.
  - Handlers: `createLink`, `listLinks`, `linkStats`, `redirect`, `health` (map to API table above). `createLink` retries code gen on unique-violation.
- **main_test.go** — `TestShortCode`: asserts length == 6, every char in alphabet, two calls differ. No DB.
- **Dockerfile** — stage 1 `golang:1.22-alpine`: copy, `go mod tidy`, `CGO_ENABLED=0 go build -o /server`. stage 2 `alpine`: copy binary, `EXPOSE 8080`, run `/server`.
- **.dockerignore** — ignore local build artifacts.

### frontend/
- **package.json** — deps `next` `^14`, `react`/`react-dom` `^18`, `recharts`; scripts `dev`/`build`/`start`.
- **next.config.js** — minimal (`output` default), `reactStrictMode: true`.
- **lib/api.js** — `const API = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'`; helpers `createLink`, `listLinks`, `getStats`.
- **app/layout.js** — root layout, imports `globals.css`, nav header.
- **app/globals.css** — minimal clean styling (no Tailwind).
- **app/page.js** — `"use client"`: create form + links table with per-row "Analytics" link + copy button.
- **app/analytics/[code]/page.js** — `"use client"`: fetch stats, show total + `recharts` `LineChart` (daily) + recent-clicks table.
- **Dockerfile** — `node:20-alpine`: copy, `npm install`, `EXPOSE 3000`, `npm run dev` (dev server; fine for scaffold).
- **.dockerignore** — `node_modules`, `.next`.

## Env vars
| Service  | Var                   | Value (compose)                                         |
|----------|-----------------------|---------------------------------------------------------|
| backend  | `DATABASE_URL`        | `postgres://urls:urls@postgres:5432/urls?sslmode=disable` |
| backend  | `BASE_URL`            | `http://localhost:8080` (used to build `short_url`)     |
| frontend | `NEXT_PUBLIC_API_URL` | `http://localhost:8080` (browser → backend)             |
| postgres | `POSTGRES_*`          | user/pw/db = `urls`                                     |

## Build order (execution)
1. `docker-compose.yml`
2. backend: `go.mod` → `main.go` → `main_test.go` → `Dockerfile` → `.dockerignore`
3. frontend: `package.json` → `next.config.js` → `lib/api.js` → `app/layout.js` → `app/globals.css` → `app/page.js` → `app/analytics/[code]/page.js` → `Dockerfile` → `.dockerignore`
4. `README.md` (run instructions)

## Verification
1. `cd ~/Documents/my-project/url-shortener && docker compose up --build`
2. Frontend http://localhost:3000 — create a link from a long URL.
3. Hit the short link a few times (browser / `curl -L http://localhost:8080/<code>`).
4. Open analytics page — total increments, chart + recent list populate.
5. Backend unit check: `cd backend && go test ./...` (shortCode charset/length/uniqueness).
