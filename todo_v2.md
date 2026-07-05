# Dynamics Dashboard - Project Checklist

**Version:** 2.0
**Date:** July 3, 2026

---
## STATUS — updated July 3, 2026

**Done (built, tested, running locally in Docker):**
- ✅ Phases 1–4: backend (auth, CRUD, historical conversion), React frontend
- ✅ Phase 5.3: local manual test pass (user-verified)
- ✅ Phase 6.1–6.2 (v2): ⚡ event markers — `currency_events` table, detection with run-coalescing, `GET /api/events`, chart markers + tooltips
- ✅ Beyond plan: daily dynamics series (`GET /api/salary/series` — salary value per day at that day's rate, with auto rate backfill), Apple-style UI redesign, thousands separators, hybrid type-or-pick date fields
- Git tags: `mvp-v1` (pre-markers rollback point), `v2-events` (current)

**Deployed to cloud (July 4, 2026):** ✅ Phase 5 complete
- Frontend: https://salary-dynamics.vercel.app (Vercel, root dir `frontend`)
- Backend: https://salary-dynamics-api.onrender.com (Render free/Frankfurt, blueprint in `render.yaml`)
- DB: Neon Postgres 17 (Frankfurt, pooled) · Daily rate cron: GitHub Actions, 06:30 UTC, verified green
- Repo: github.com/Kasabulatov/salary-dynamics (private)

**v2.2 + v3 shipped (July 5, 2026):**
- ✅ Landing page + guest mode (sessionStorage, stateless `POST /api/public/compute`)
- ✅ Phase 6.3 (v3): news headlines on event bands — fetched by the GitHub Actions
  workflow from **Google News RSS** (GDELT throttles all cloud/CI IPs; backend
  serves queue via `/api/internal/news/pending` + stores via `/api/internal/news`),
  cached forever, clickable bands open the article

**v4 shipped, pivoted (July 5, 2026):** owner-only analytics — site-wide Metrica tag
(counter 110422979, webvisor off) + 12 product goal events (behavior only, never
amounts); owner analyzes at metrika.yandex.com. The original per-user OAuth module
was built then removed same day (git history: 715605e → 2479c4e).

**Phase 8 (July 5, 2026):** Playwright E2E suite (auth/salary/guest, USD-only
fixtures — no external API flake), CI on every push (backend tests, frontend
build, e2e, govulncheck + npm audit), Render deploys gated on CI green
(autoDeployTrigger: checksPass), Dependabot weekly, §5.2 audit fixes (1 MiB
body cap, bcrypt 72-char password cap, Referrer-Policy header).

**Next up:**
- ⏭ optional: CSP header on the frontend (needs Metrica allowances) · repo public decision

**Run locally:** `docker compose up -d` → http://localhost:5173 (local test account: test@example.com / password123)
---

This checklist breaks development into actionable steps, sequenced **MVP-first**. Phases 1–4 deliver a fully testable, deployable Salary Tracker. Phases 5–7 add markers, news, and the Analytics module and are explicitly post-MVP.

> Conversion always uses the exchange rate **on each entry's effective date** (historical), never today's rate. See spec §2.2.

---

## Phase 1: Backend Foundation & User Authentication

- [ ] **Step 1.1: Basic Server Setup**
    - [ ] Initialize Go project (`go mod init`).
    - [ ] Add `chi` router dependency.
    - [ ] Add `httprate` for rate limiting; apply a global limiter.
    - [ ] Create a basic HTTP server in `main.go`.
    - [ ] Write a test for the `/health` endpoint.
    - [ ] Implement `/health` returning `200 OK` and `{"status": "ok"}`.

- [ ] **Step 1.2: Database Integration**
    - [ ] Add `pgx` driver dependency.
    - [ ] Create a `database` package for the PostgreSQL connection (Neon or Supabase free tier).
    - [ ] Load connection config from environment variables.
    - [ ] Create migration `001_create_users_table.sql` (include `default_display_currency` default `'USD'`, `created_at`, `updated_at`).
    - [ ] Connect to the database on startup.

- [ ] **Step 1.3: User Registration**
    - [ ] Create `security` package with a `bcrypt` password-hashing function.
    - [ ] Add input validation (well-formed unique email, password rules).
    - [ ] Create `database.CreateUser`.
    - [ ] Implement `authHandler.Register`.
    - [ ] Add `POST /api/register`.
    - [ ] Write unit/integration tests for registration.

- [ ] **Step 1.4: User Login (cookie-based JWT)**
    - [ ] Add JWT library (`github.com/golang-jwt/jwt/v5`).
    - [ ] Create `database.GetUserByEmail`.
    - [ ] Create `security.CheckPasswordHash`.
    - [ ] Implement `Login`: verify credentials, issue a JWT, set it in an **HttpOnly, Secure, SameSite=Lax cookie** (NOT a JSON body / localStorage).
    - [ ] Add `POST /api/login`.
    - [ ] Add `POST /api/logout` that clears the cookie.
    - [ ] Write tests for login/logout.

- [ ] **Step 1.5: Authentication Middleware & CSRF**
    - [ ] Implement `chi` middleware that validates the JWT from the **cookie**.
    - [ ] Add user ID to the request context on success; return `401` on failure.
    - [ ] Add **CSRF protection** for state-changing routes (double-submit token or strict SameSite).
    - [ ] Set security headers (HSTS, etc.).
    - [ ] Write tests for valid and invalid token cases.

---

## Phase 2: Salary Tracker - Backend (CRUD)

- [ ] **Step 2.1: Salary Data Model**
    - [ ] Create migration `002_create_salary_entries_table.sql` (FK to `users`, `created_at`, `updated_at`).
    - [ ] Define the `SalaryEntry` struct in `models`.

- [ ] **Step 2.2: Create Salary Entry**
    - [ ] Create `database.CreateSalaryEntry`.
    - [ ] Validate input (amount > 0, valid ISO 4217 currency, sane date).
    - [ ] Implement `HandleCreateEntry` (scoped to the authenticated user).
    - [ ] Add protected `POST /api/salary`.
    - [ ] Tests: endpoint is protected and saves correctly.

- [ ] **Step 2.3: Retrieve Salary Entries**
    - [ ] Create `database.GetSalaryEntriesByUserID`.
    - [ ] Implement `HandleGetEntries` (authenticated user only).
    - [ ] Add protected `GET /api/salary`.
    - [ ] Tests.

- [ ] **Step 2.4: Edit & Delete Salary Entries**
    - [ ] Create `database.UpdateSalaryEntry` and `database.DeleteSalaryEntry` (both verify ownership).
    - [ ] Implement `HandleUpdateEntry` and `HandleDeleteEntry`.
    - [ ] Add protected `PUT /api/salary/{id}` and `DELETE /api/salary/{id}`.
    - [ ] Tests, including that users cannot modify another user's entries.

---

## Phase 3: Historical Currency Conversion (core value)

- [ ] **Step 3.1: Rate Storage**
    - [ ] Create migration `003_create_daily_rates_table.sql` (`date`, `base_currency`, `quote_currency`, `rate`, `source`; unique index on the first three).

- [ ] **Step 3.2: Rate Source Clients**
    - [ ] Create a `services` package with a `RateSource` interface (fetch historical range for a pair).
    - [ ] Implement a **Frankfurter** client (no API key) for supported currencies.
    - [ ] Implement a **National Bank of Kazakhstan** client for KZT pairs.
    - [ ] Implement a resolver that picks the source per currency pair.

- [ ] **Step 3.3: Rate Ingestion & Cache**
    - [ ] Implement a routine that fetches a historical rate range and upserts into `daily_rates`.
    - [ ] Add a protected internal endpoint `POST /api/internal/refresh` to trigger ingestion (secured by a shared secret).
    - [ ] Add request-level in-memory caching in the service to avoid redundant DB hits within a request.

- [ ] **Step 3.4: Conversion Logic**
    - [ ] Implement conversion using the rate **on each entry's `effective_date`**, with **last-known-rate carry-forward** when the exact date is missing (weekend/holiday).
    - [ ] Unit-test conversion thoroughly (exact-date hit, carry-forward, same-currency no-op, unknown pair error).

- [ ] **Step 3.5: Enhance GET with Conversion**
    - [ ] Add a `displayCurrency` query parameter to `GET /api/salary` (default `USD`).
    - [ ] Return both the original amount and the converted (historical) value per entry.
    - [ ] Update tests to check converted values.

---

## Phase 4: Frontend - Salary Tracker MVP

- [ ] **Step 4.1: App Setup**
    - [ ] Initialize React + Vite.
    - [ ] Install `react-router-dom`, `recharts`, and `@tanstack/react-query` (native `fetch` for HTTP; axios optional).
    - [ ] Set up file structure (components, pages, services).
    - [ ] Configure the API client to send cookies (`credentials: 'include'`).

- [ ] **Step 4.2: Authentication UI**
    - [ ] Create `LoginPage` and `RegisterPage`.
    - [ ] Create `AuthContext` for auth state.
    - [ ] Implement login/registration forms + API calls; handle logout.
    - [ ] Set up routing including a `ProtectedRoute` component.

- [ ] **Step 4.3: Salary Tracker Page & Data Display**
    - [ ] Create a protected `SalaryTrackerPage`.
    - [ ] Create an `api` service to fetch salary entries.
    - [ ] Use React Query to fetch and cache; render entries in a table.
    - [ ] Loading/error states handled by React Query.

- [ ] **Step 4.4: Add / Edit / Delete Forms**
    - [ ] Create `SalaryEntryForm` (start with one form, add more via a **+** icon).
    - [ ] Wire up create (`POST`), edit (`PUT`), delete (`DELETE`).
    - [ ] Invalidate/refetch the list on success (React Query).
    - [ ] Inline validation messages.

- [ ] **Step 4.5: Chart Visualization**
    - [ ] Create `SalaryChart` (accepts data as a prop) using **Recharts** (line: time X, value Y).
    - [ ] Show a dot per salary-change event with a tooltip (note + exact details).
    - [ ] Integrate into `SalaryTrackerPage`.

- [ ] **Step 4.6: UI Controls**
    - [ ] Dropdowns for **Salary Currency** and **Display Currency** (default `USD`).
    - [ ] Preset date filters (Last 6 Months, YTD, Last 12 Months, Current Year, Last 5 Years) + custom date-range picker.
    - [ ] Re-fetch with correct query params and update the chart.

---

## Phase 5: Deployment of MVP (test in the cloud)

- [ ] **Step 5.1: Deploy Backend + DB**
    - [ ] Provision Postgres on **Neon** or **Supabase**; run migrations.
    - [ ] Deploy the Go backend to **Render** or **Fly.io**; set env vars (DB URL, JWT secret, refresh secret).
    - [ ] Schedule daily rate refresh via a **GitHub Actions** cron hitting `/api/internal/refresh`.

- [ ] **Step 5.2: Deploy Frontend**
    - [ ] Deploy the React app to **Vercel**; point the API base URL at the backend.
    - [ ] Verify CORS + cookie settings work cross-origin (Secure, SameSite).

- [ ] **Step 5.3: Manual Test Pass**
    - [ ] Register/login, add/edit/delete entries, verify historical conversion and date filters end-to-end.

---

## Phase 6: Event Markers & News (v2 / v3, post-MVP)

- [ ] **Step 6.1: Currency Fluctuation Caching (v2)**
    - [ ] Create migration `004_create_currency_events_table.sql`.
    - [ ] Extend the refresh job to compute significant changes (**> 2% daily or weekly**) into `currency_events`.

- [ ] **Step 6.2: Event Markers API & UI (v2)**
    - [ ] Add `GET /api/events` (by currency pair + date range).
    - [ ] Fetch event data on the frontend.
    - [ ] Render ⚡ markers on the chart. Consider migrating the chart to **Apache ECharts** here for built-in `markPoint`/`markLine` support; tooltip shows percent + absolute change.

- [ ] **Step 6.3: News Headlines (v3)**
    - [ ] Integrate the **GDELT** client; attach a relevant headline/URL to event days.
    - [ ] Extend the event tooltip to show the headline (fail silently if none).

---

## Phase 7: Analytics Module (v4, post-MVP)

- [ ] **Step 7.1: Backend**
    - [ ] Create migration `005_create_analytics_connections_table.sql`.
    - [ ] Implement OAuth handlers for Google (GA4) and Yandex Metrica (read-only scopes).
    - [ ] Encrypt tokens at rest using a key from `TOKEN_ENCRYPTION_KEY`.
    - [ ] Implement protected `GET /api/analytics`.

- [ ] **Step 7.2: UI**
    - [ ] Components to connect Google/Yandex accounts.
    - [ ] Create `ProductMetricsPage`.
    - [ ] Fetch and chart Users/Sessions/Pageviews; reuse the shared date-filter controls.

---

## Phase 8: Finalization

- [ ] **Step 8.1: Security & Quality Audit**
    - [ ] Manual audit against the spec §5.2 checklist.
    - [ ] Run `npm audit` and `govulncheck`; update dependencies.
    - [ ] Verify error handling is user-friendly and non-leaky.

- [ ] **Step 8.2: E2E Tests (now that flows are stable)**
    - [ ] Playwright: register/login, add/edit/delete/view entries, chart interactions, (v4) connect analytics.

- [ ] **Step 8.3: CI/CD & Docs**
    - [ ] Configure CI to run tests on push; auto-deploy on green.
    - [ ] Write `README.md` (setup + deployment); comment Go code where useful.
