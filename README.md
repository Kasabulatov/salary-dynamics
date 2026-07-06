# Dynamics Dashboard

A single-page web app with two modules:

1. **Salary Tracker (MVP)** — visualize salary history converted between currencies using **historical exchange rates** (rate on each entry's effective date, not today's).
2. **Product Metrics Dashboard (post-MVP)** — Google Analytics (GA4) + Yandex Metrica.

Built with free and open-source technologies and free service tiers only.

> **For implementers:** read [`spec_v2.md`](./spec_v2.md) (full technical spec) and follow [`todo_v2.md`](./todo_v2.md) (step-by-step checklist, sequenced MVP-first). This README is the quick orientation.

---

## Documents

| File | Purpose |
|---|---|
| [`spec_v2.md`](./spec_v2.md) | Complete technical specification (requirements, architecture, data models, security, testing) |
| [`todo_v2.md`](./todo_v2.md) | Actionable build checklist, phases 1–8 |
| `README.md` | This file — setup, stack, and conventions |

---

## Tech Stack

| Layer | Choice |
|---|---|
| Backend | Go + `chi` router |
| Auth | `bcrypt` + JWT in an **HttpOnly cookie** + CSRF |
| Frontend | React + Vite |
| Data fetching | TanStack Query (React Query) |
| Charts | Recharts (MVP) → Apache ECharts (v2, for ⚡ markers) |
| Database | PostgreSQL (`pgx` driver) |
| Rate limiting | `github.com/go-chi/httprate` |

### Free services

| Component | Service |
|---|---|
| Frontend hosting | Vercel |
| Backend hosting | Render or Fly.io |
| Database | Neon or Supabase (serverless Postgres) |
| Scheduled rate refresh | GitHub Actions cron |
| Historical FX rates | Frankfurter (no key) + National Bank of Kazakhstan (KZT) |
| News (v3) | GDELT Project |

---

## Delivery Milestones

| Milestone | Contents |
|---|---|
| **MVP** | Auth · add/edit/delete salary entries · line chart · historical currency conversion · date-range presets |
| **v2** | ⚡ significant exchange-rate change markers (no news) |
| **v3** | News headlines attached to markers (GDELT) |
| **v4** | Product Metrics Dashboard (GA4 + Yandex Metrica OAuth) |

The Salary Tracker (Phases 1–5 in the checklist) must be fully buildable, testable, and deployable **before** any Analytics work.

---

## Quick Start (Docker — fully isolated, nothing installed on the host)

Everything (Go, Node, Postgres) runs inside containers; source code lives in
this folder and is volume-mounted in. The only host requirement is Docker.

```bash
cp .env.example .env        # then fill in secrets (any random strings locally)
docker compose up -d        # starts postgres + backend (:8080) + frontend (:5173)
```

Open **http://localhost:5173** — register an account and start adding entries.

Useful commands:

```bash
docker compose logs -f backend      # tail backend logs
docker compose exec db psql -U dynamics dynamics   # open a DB shell
docker compose run --rm backend go test ./...      # run backend tests
docker compose down                 # stop everything (data persists)
docker compose down -v              # stop AND wipe the database volume
```

Bulk-ingest historical rates (optional — conversion also fetches on demand):

```bash
curl -X POST http://localhost:8080/api/internal/refresh \
  -H "X-Refresh-Secret: <REFRESH_SECRET from .env>" \
  -H "Content-Type: application/json" \
  -d '{"start": "2021-01-01", "end": "2026-07-03"}'
```

---

## Local Development (target layout)

```
salary_dynamics_v2/
├── backend/          # Go API
│   ├── main.go
│   ├── database/     # pgx connection + queries
│   ├── security/     # bcrypt, JWT
│   ├── services/     # rate sources (Frankfurter, NBK)
│   ├── models/
│   └── migrations/   # 001_..._sql etc.
├── frontend/         # React + Vite
│   └── src/
│       ├── components/
│       ├── pages/
│       └── services/
├── spec_v2.md
├── todo_v2.md
└── README.md
```

### Backend

```bash
cd backend
go mod init dynamics-dashboard        # first time only
go mod tidy
# apply migrations (e.g. via psql or a migration tool) against $DATABASE_URL
go run .
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

The frontend API client must send cookies: `fetch(url, { credentials: 'include' })`.

---

## Environment Variables

| Name | Used by | Purpose |
|---|---|---|
| `DATABASE_URL` | backend | Postgres connection string (Neon/Supabase) |
| `JWT_SECRET` | backend | Signs auth JWTs |
| `REFRESH_SECRET` | backend | Shared secret protecting `POST /api/internal/refresh` |
| `TOKEN_ENCRYPTION_KEY` | backend (v4) | Symmetric key for encrypting OAuth tokens at rest |
| `CORS_ORIGIN` | backend | Allowed frontend origin (for cross-origin cookies) |
| `VITE_API_BASE_URL` | frontend | Backend API base URL |

> Never commit secrets. Provide a `.env.example` with empty values.

---

## Key Design Decisions

- **Historical conversion:** every salary amount is converted using the exchange rate on its `effective_date`, with **last-known-rate carry-forward** when the exact date has no rate (weekend/holiday). See spec §2.2.
- **Rate caching:** historical rates are fetched once into the `daily_rates` table; all conversion reads from Postgres. A daily GitHub Actions cron refreshes rates via `POST /api/internal/refresh` (free backends spin down, so no in-process cron).
- **Auth:** JWT lives in an HttpOnly/Secure/SameSite cookie (not localStorage), so CSRF protection is required on state-changing routes.
- **Rate sources are pluggable:** a `RateSource` interface resolves each currency pair to a provider — add more central-bank adapters as needed.

---

## API Endpoints (MVP)

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/health` | – | Health check |
| POST | `/api/register` | – | Create account |
| POST | `/api/login` | – | Log in (sets cookie) |
| POST | `/api/logout` | – | Clear cookie |
| GET | `/api/me` | ✅ | Current user (session restore) |
| GET | `/api/salary?displayCurrency=USD` | ✅ | List entries with historical conversion |
| POST | `/api/salary` | ✅ | Create entry |
| PUT | `/api/salary/{id}` | ✅ | Update entry |
| DELETE | `/api/salary/{id}` | ✅ | Delete entry |
| GET | `/api/salary/series` | ✅ | Daily dynamics line (value per day at that day's rate) |
| GET | `/api/salary/inflation` | ✅ | Inflation target line (World Bank CPI, Jan-1 steps) |
| GET | `/api/events?base=&quote=` | ✅ | ⚡ events (>2%/day, >4%/week) incl. cached news headlines |
| POST | `/api/public/compute` | – (rate-limited) | Stateless compute for guest mode & landing demo |
| POST | `/api/public/compare` | – (rate-limited) | Offer Comparison: today's-rate FX + cost-of-living-adjusted verdict |
| GET | `/api/public/compare/meta` | – (rate-limited) | Known-cities list + COL dataset disclaimer/date |
| POST | `/api/internal/refresh` | secret | Daily cron: rates + inflation + GDELT news headlines |
| GET | `/api/analytics` | ✅ | Product metrics (v4, not built yet) |

---

## Testing

- **Unit (first):** historical conversion logic (exact-date, carry-forward, same-currency, unknown pair) and auth (hashing, JWT). Go `testing` + Vitest.
- **Integration:** endpoint request/response with mocked DB and external services.
- **E2E (after MVP is stable):** Playwright for register/login and salary CRUD.
- **Security:** `npm audit` + `govulncheck`; manual audit against spec §5.2.

---

## License

Uses only free/open-source components. Add a project license (e.g. MIT) before publishing.
