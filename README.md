# URL Shortener + Analytics

Go backend · Next.js frontend · Postgres · Docker.

## Run

```bash
docker compose up --build
```

- Frontend: http://localhost:3000
- Backend API: http://localhost:8080

## Use

1. Open the frontend, paste a long URL, click **Shorten**.
2. Visit the short link (`http://localhost:8080/<code>`) — redirects + logs the click.
3. Click **Analytics** on a link to see total clicks, a 30-day chart, and recent clicks.

## API

| Method | Path                       | Body / result                                  |
|--------|----------------------------|------------------------------------------------|
| GET    | `/health`                  | `{status:"ok"}`                                |
| POST   | `/api/links`               | `{url}` → `{code, url, short_url}`             |
| GET    | `/api/links`               | `[{code, url, short_url, clicks, created_at}]` |
| GET    | `/api/links/{code}/stats`  | `{code, url, total, daily[], recent[]}`        |
| GET    | `/{code}`                  | 302 redirect (logs click)                      |

## Backend tests

```bash
cd backend && go test ./...
```

## Architecture / defaults

- Go stdlib `net/http` (1.22 routing), no web framework.
- Postgres only (no Redis yet — add caching when read load needs it).
- Schema auto-created on startup (`CREATE TABLE IF NOT EXISTS`).
- Click logging is fire-and-forget (goroutine).
- CORS open (`*`) for dev — lock down for production.

## Stack diagram

```
browser ──> frontend (Next.js :3000) ──> backend (Go :8080) ──> Postgres :5432
   └──────────────── short link GET /{code} ────────┘
```
