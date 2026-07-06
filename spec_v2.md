# Developer Specification: Dynamics Dashboard

**Version:** 2.0
**Date:** July 3, 2026
**Status:** Ready for implementation

---

## 1.0 Introduction

### 1.1 Purpose
This document provides a complete technical specification for the "Dynamics Dashboard" web application. It is intended for the development team (and AI implementation agents) to use as a guide for implementation, testing, and deployment.

### 1.2 Project Scope
The project is a single-page web application with two modules, delivered in sequence:

1. **Salary Tracker (MVP):** A personal finance tool for visualizing salary history converted between currencies using **historical exchange rates**.
2. **Product Metrics Dashboard (post-MVP):** An analytics tool for visualizing key metrics from Google Analytics (GA4) and Yandex Metrica.

The application must be built using **only free and open-source technologies and free service tiers**, and adhere to modern security best practices.

### 1.3 Delivery Milestones
The scope is explicitly sequenced so a testable product ships early:

| Milestone | Contents |
|---|---|
| **MVP** | Auth · add/edit/delete salary entries · line chart · historical currency conversion · date-range presets |
| **v2** | ⚡ significant exchange-rate change markers (no news) |
| **v3** | News headlines attached to markers (GDELT) |
| **v4** | Product Metrics Dashboard (GA4 + Yandex Metrica OAuth) |

> The two modules are only loosely related. The Salary Tracker must be fully buildable, testable, and deployable **before** any Analytics work begins.

### 1.4 Target Audience
IT specialists, data analysts, and product managers who need a simple, unified dashboard for personal finance and professional analytics.

---

## 2.0 Functional Requirements

### 2.1 User Story: Core
- As a new user, I want to register and log in securely, so my data is private.
- As a logged-in user, I want a navigation menu to switch between the Salary Tracker and (once available) the Product Metrics Dashboard.

### 2.2 User Stories: Salary Tracker (MVP)
- As a user, I want to input my salary history — each entry containing an **amount**, a **currency**, an **"effective from" date**, and an **optional note** — so I can build a timeline of my compensation.
- As a user, I want to **edit and delete** existing entries.
- As a user, I want to start with one salary input form and add more by clicking a **+** icon.
- As a user, I want to see my salary history on a line chart with time on the X-axis and value on the Y-axis.
- As a user, I want to select my **Salary Currency** (the currency of my inputs) and a **Display Currency** (the currency for the chart) from two dropdowns. Display Currency defaults to **USD**.
- As a user, I want to filter the chart's time period using presets (Last 6 Months, Year to Date, Last 12 Months, Current Year, Last 5 Years) and a custom date-range picker.
- As a user, I want a dot on the chart for each salary-change event; hovering shows the note and exact salary details in a tooltip.

> **Conversion semantics (decided):** When converting a salary amount to the Display Currency, the application uses the exchange rate **effective on that entry's `effective_date`** — NOT today's rate. This is the whole point of a "dynamics" chart: it reflects the real value of each salary at the time it was earned. Where an exact-date rate is unavailable (weekend/holiday), use the most recent prior available rate ("last known rate" carry-forward).

### 2.3 User Stories: Exchange-Rate Event Markers (v2 / v3)
- **(v2)** As a user, I want a special icon (⚡) on the chart on days when the exchange rate between my Salary and Display currencies experienced a significant change (**> 2% daily or weekly**). Hovering shows the change percentage and the absolute rate change.
- **(v3)** As a user, I want that same tooltip to optionally include a relevant news headline for the event day.

### 2.4 User Stories: Offer Comparison ("What-If")
- As a visitor (no account needed), I want to enter my current salary (amount, currency, optional city) and a job offer (amount, currency, optional city), and see which is worth more — **by today's exchange rate** and **adjusted for cost of living** — with a plain-language verdict.
- As a visitor, I want the page pre-filled with a working example so I see a result before typing anything.
- As a visitor, I want to download a shareable image of the result (watermarked with the site URL).

> Rates: **today's** (latest cached) rate — a forward-looking decision, unlike the tracker's rate-on-effective-date above. Cost of living: the committed `cost_of_living.json` (NYC = 100, approximate, embedded via Go `embed`; city → currency-derived country → FX-only fallback). Full details: `feature_offer_comparison.md`.

### 2.5 User Stories: Product Metrics Dashboard (v4)
- As a user, I want to securely connect my Google Analytics and Yandex Metrica accounts using OAuth, granting read-only access.
- As a user, I want to select a connected account and property to view its data.
- As a user, I want to see key metrics (Users, Sessions, Pageviews) on a line chart over a selected time period.
- As a user, I want to use the same time-period filter controls as the Salary Tracker.

---

## 3.0 System Architecture & Technology Stack

### 3.1 High-Level Architecture
Modern client-server architecture:
- **Frontend:** A single-page application (SPA) responsible for all UI rendering and interaction.
- **Backend:** A RESTful API in Go handling business logic, authentication, and communication with the database and external services.
- **Database:** PostgreSQL, storing user data, salary entries, and cached rate/news data.
- **External Services:** Third-party sources for historical exchange rates and (later) news.

### 3.2 Technology Stack

| Layer | Choice | Notes |
|---|---|---|
| Backend | **Go** + `chi` router | |
| Auth | `bcrypt` + JWT stored in an **HttpOnly cookie** | See §5.2 — not localStorage |
| Frontend | **React + Vite** | Decided (no longer TBD) |
| Data fetching | **TanStack Query (React Query)** | Handles loading/error/caching/refetch |
| Charts (MVP) | **Recharts** | Line chart |
| Charts (v2+) | **Apache ECharts** | Built-in `markPoint`/`markLine` for ⚡ markers — reduces custom code |
| HTTP client | native `fetch` | axios optional; not required |
| Rate limiting | `github.com/go-chi/httprate` | |
| Database | **PostgreSQL** | |
| DB driver | `pgx` | |

### 3.3 External Services & APIs (all free)

**Historical exchange rates** (open-source / official, no paid tier):
- **Frankfurter** (`frankfurter.dev`) — free, **no API key**, open-source, ECB reference data, full **historical time-series** endpoint. Covers ~31 major currencies (USD, EUR, GBP, RUB, etc.). *Does not cover KZT.*
- **National Bank of Kazakhstan** (`nationalbank.kz`) — free official daily KZT rates, including history. Used for any KZT pair.
- **Design rule:** the backend resolves each currency pair to a source (Frankfurter for supported currencies; National Bank of Kazakhstan for KZT), fetches history **once**, and stores it locally (§3.4). All conversion then runs against the local DB — no per-request external calls.

> If additional currencies are needed later that neither source covers, add a per-currency adapter behind the same interface (e.g. that country's central-bank open feed). Avoid paid aggregators (Alpha Vantage / ExchangeRate-API historical / OpenExchangeRates) — their free tiers do not provide usable historical data.

**News data (v3 only):**
- **GDELT Project** (`gdeltproject.org`) — free, open, global **historical** news API. Replaces NewsAPI.org, whose free tier is localhost-only, blocked in production, and limited to ~1 month of articles (unusable for this feature).

**Analytics (v4 only):**
- Google Analytics Data API (GA4) and Yandex Metrica API via OAuth.

### 3.4 Data Caching & Scheduled Jobs
To minimize external calls, a scheduled job fetches, processes, and caches exchange-rate (and later news) data into PostgreSQL. The frontend and API always query this cached data.

> **Deployment note:** free backend tiers spin down when idle, so an in-process cron will not fire reliably. Use a **GitHub Actions scheduled workflow** (free) that calls a protected `/api/internal/refresh` endpoint on a daily schedule. Alternatively, Supabase `pg_cron` if hosting the DB there.

The in-memory cache in the currency service (see todo 2.4) is a request-level optimization only; the Postgres cache is the source of truth.

### 3.5 Deployment Targets (free tiers, 2026)

| Component | Service | Notes |
|---|---|---|
| Frontend | **Vercel** | Free |
| Backend (Go) | **Render** (free web service) or **Fly.io** | Render spins down when idle |
| Database | **Neon** or **Supabase** (serverless Postgres) | Free tier |
| Scheduled refresh | **GitHub Actions** cron | Free |

> Heroku's free tier was discontinued in 2022 and must not be used.

---

## 4.0 Data Models

**User**
- `id` (PK)
- `email` (string, unique)
- `password_hash` (string)
- `default_display_currency` (string, default `"USD"`)
- `created_at`, `updated_at` (timestamp)

**SalaryEntry**
- `id` (PK)
- `user_id` (FK → User)
- `amount` (decimal)
- `currency_code` (string, ISO 4217, e.g. `"KZT"`)
- `effective_date` (date)
- `note` (text, nullable)
- `created_at`, `updated_at` (timestamp)

**DailyRate** (cached historical rates)
- `id` (PK)
- `date` (date)
- `base_currency` (string)
- `quote_currency` (string)
- `rate` (decimal)
- `source` (string, e.g. `"frankfurter"`, `"nbk"`)
- unique index on (`date`, `base_currency`, `quote_currency`)

**CurrencyEvent** (cached significant-change markers — v2/v3)
- `id` (PK)
- `date` (date)
- `base_currency` (string)
- `quote_currency` (string)
- `percent_change` (float)
- `absolute_change` (decimal)
- `news_headline` (string, nullable) — v3
- `news_url` (string, nullable) — v3

**AnalyticsConnection** (v4)
- `id` (PK)
- `user_id` (FK → User)
- `provider` (string, `"google"` or `"yandex"`)
- `encrypted_access_token` (string)
- `encrypted_refresh_token` (string)

> **Token encryption (v4):** tokens are encrypted at rest with a symmetric key supplied via an environment variable (e.g. `TOKEN_ENCRYPTION_KEY`). The key is never stored in the database or source control.

---

## 5.0 Non-Functional Requirements

### 5.1 Performance
- Initial page load < 3 seconds on a standard connection.
- API responses < 500 ms for typical requests.
- Because all conversion runs against locally cached rates (§3.4), chart queries never block on external APIs.

### 5.2 Security
**Frontend:** Use HTTPS; validate/sanitize all inputs (prevents XSS); implement CSRF protection; never expose API keys or secrets to the client.

**Authentication (decided):** The JWT is delivered and stored in an **HttpOnly, Secure, SameSite=Lax cookie** — not localStorage — so it cannot be stolen via XSS. Because auth is cookie-based, **CSRF protection is mandatory** on all state-changing endpoints (double-submit token or SameSite enforcement). Tokens have a bounded expiry; plan for a refresh mechanism.

**Backend:** Use established libraries for auth (bcrypt password hashing + salting); perform authorization checks on every action (users may only touch their own rows); protect all non-public endpoints; use parameterized queries / prepared statements via `pgx` (prevents SQL injection); set security headers (HSTS, etc.); apply **rate limiting** (`httprate`) on all endpoints, especially auth.

**Habits:** Keep dependencies updated; handle errors gracefully without leaking internals; use secure cookies (HttpOnly, Secure, SameSite).

### 5.3 Input Validation Rules
- `amount` > 0.
- `currency_code` must be a valid ISO 4217 code the backend can resolve to a rate source.
- `effective_date` must be a valid date, not in the far future.
- `email` must be well-formed and unique.

---

## 6.0 Error Handling Strategy

**Client-side**
- **Form validation:** clear, inline error messages (e.g. "Please enter a valid date", "Amount must be greater than 0").
- **API errors:** non-critical failures (e.g. missing news headline) fail silently or show a subtle toast; critical failures (e.g. chart data) show a clear error in the chart area with a "Try again" button.

**Server-side**
- **Standardized responses:** standard HTTP status codes and a consistent JSON error object: `{"error": "message"}`.
- **Logging:** all unexpected server errors logged with timestamp, request ID, and stack trace. Sensitive data (passwords, tokens) scrubbed from logs.

---

## 7.0 Testing Plan

Testing is **right-sized to the milestone**. Do not let full E2E coverage block the first manual/cloud test of the MVP.

### 7.1 Unit Testing (do first)
- **Backend (Go):** built-in `testing` package. Prioritize the risky logic: **historical currency conversion** (including last-known-rate carry-forward), auth (hashing, JWT issue/verify), and API handlers.
- **Frontend:** Vitest for utility functions and key components.

### 7.2 Integration Testing
- Full request/response cycle of each API endpoint, mocking the database and external services.
- Frontend integration (e.g. date picker correctly filters the chart).

### 7.3 End-to-End (defer until after MVP is validated)
- Playwright (preferred) or Cypress for critical flows: registration/login; add/edit/delete/view salary entries; chart interactions; (v4) connecting an analytics account.

### 7.4 Security Testing
- Run dependency scanners (`npm audit`, `govulncheck`) regularly.
- Manual audit against the §5.2 checklist before the first major release.

### 7.5 Coverage Goal
- Aim for meaningful coverage of **core business logic** (conversion + auth) first; broaden later. A blanket 80% target is not a gate for the MVP milestone.
