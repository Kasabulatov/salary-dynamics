# Feature Spec + Implementation Plan: Offer Comparison ("What-If")

**Version:** 1.0
**Date:** July 6, 2026
**Target repo:** `github.com/Kasabulatov/salary-dynamics`
**For:** AI implementation agent (Fable)
**Status:** Ready for implementation. Read `spec_v2.md` and `todo_v2.md` first; this
document is an addendum and follows the same conventions (migration numbering, cookie
auth, `RateSource` resolver, guest/stateless pattern, right-sized testing, CI-green gate).

---

## 0. Context for the agent (read before coding)

Current state (from `todo_v2.md`, July 5 2026): MVP + v2 markers + v3 news +
guest mode all shipped and deployed. Last migration is `004_create_currency_events_table.sql`.
Guest mode already works via stateless `POST /api/public/compute` + sessionStorage.
Charts use Apache ECharts. Owner analytics = Yandex Metrica counter 110422979 with
12 goal events. CI runs backend tests + frontend build + Playwright e2e + govulncheck +
npm audit on every push; Render deploy is gated on CI green (`autoDeployTrigger: checksPass`).

**This feature adds a new page/route, one new backend endpoint, and one committed data
file. It does NOT touch auth, the salary CRUD, or the existing conversion pipeline.**
Keep the blast radius small.

---

## 1. What we're building and why

**Job-to-be-done:** "I earn X in currency A. I have (or am considering) an offer of Y in
currency B, possibly in a different city. Am I actually better off — by exchange rate, and
by real purchasing power?"

**Primary user:** someone changing jobs / relocating / going remote, who will start
receiving a new salary in a new currency and wants an honest comparison against their
current one. Secondary: anyone weighing a competing offer.

**Why it matters to the product:** this is an **acquisition hook**, not a retention feature.
It is a decision-moment tool — used intensely for a few days, then the user decides and
leaves. That is fine and expected. Its job is to widen the top of the funnel and produce a
**shareable result** (see §6). Do not over-invest; ship the honest version and move on.

**The one thing that makes it more than a currency converter:** a **cost-of-living /
purchasing-power adjustment.** Comparing €3,500 in Lisbon to $4,000 in San Francisco by FX
alone is misleading. The comparison must show both:
1. Raw FX comparison (both salaries in a common currency at **today's** rate — this is a
   forward-looking decision, so today's rate is correct here, unlike the historical
   salary tracker).
2. Purchasing-power-adjusted comparison (using cost-of-living indices).

---

## 2. Scope

### In scope (v1 of this feature)
- A new **Offer Comparison** page, reachable in **guest mode** (no login) and when logged in.
- Inputs: current salary (amount + currency + optional city), offer salary (amount +
  currency + optional city).
- Output: side-by-side comparison — raw FX conversion + cost-of-living-adjusted "real"
  comparison, with a plain-language verdict ("The offer is ~12% better in real terms").
- A **static committed cost-of-living dataset** (CSV/JSON) with a small set of cities +
  a country-level fallback.
- **Share-image export** of the result (see §6). This is high-leverage; do it in this feature.
- Reuse existing ECharts styling + the Apple-style UI already in the app.

### Explicitly OUT of scope (do not build now)
- Any paid or live cost-of-living API (Numbeo API is paid; scraping Numbeo violates its
  ToS — **do not scrape it**).
- Tax modelling / net-salary calculation. (Note it as a future idea; do not build.)
- Saving/comparing multiple offers per user, history of comparisons.
- Any change to the historical salary conversion pipeline.

---

## 3. Data: cost-of-living dataset (static, committed)

Create `backend/services/data/cost_of_living.json` (committed to the repo).

Shape:
```json
{
  "updated": "2026-07-06",
  "source_note": "Approximate cost-of-living indices, base = New York City = 100. Manually curated. For guidance only.",
  "cities": [
    { "city": "New York", "country": "US", "index": 100.0 },
    { "city": "Lisbon", "country": "PT", "index": 55.0 },
    { "city": "Almaty", "country": "KZ", "index": 34.0 },
    { "city": "Berlin", "country": "DE", "index": 62.0 }
  ],
  "countries": [
    { "country": "US", "index": 90.0 },
    { "country": "PT", "index": 52.0 },
    { "country": "KZ", "index": 33.0 }
  ]
}
```

Rules:
- Index base: NYC = 100. A city with index 50 is ~half as expensive as NYC.
- **Resolution order:** exact city → country fallback → if neither known, skip the
  purchasing-power line and show only the FX comparison, with a note ("cost-of-living data
  unavailable for this location — showing exchange-rate comparison only").
- Seed with ~30–40 cities biased toward the target audience: Kazakhstan (Almaty, Astana),
  major relocation destinations (Lisbon, Berlin, Amsterdam, Warsaw, Dubai, Istanbul,
  Tbilisi, Yerevan, Belgrade, Bangkok), and common source cities (Moscow, Kyiv, NYC, SF,
  London). Populate `index` from any reputable public cost-of-living index; **round to
  whole numbers** and mark approximate. Do not claim precision you don't have.
- Add a top-of-file comment / `source_note` making clear these are approximate and
  manually updated. The UI must surface this disclaimer.

> Design rule (mirrors the FX `RateSource` rule in spec §3.3): the dataset is loaded once at
> startup (or embedded via Go `embed`) and served from memory. No external calls.

---

## 4. Backend

### 4.1 Endpoint (guest-friendly, stateless — mirrors `/api/public/compute`)

`POST /api/public/compare` — **no auth, rate-limited** via the existing `httprate` limiter
(same tier as `/api/public/compute`).

Request:
```json
{
  "current": { "amount": 800000, "currency": "KZT", "city": "Almaty" },
  "offer":   { "amount": 3500,   "currency": "EUR", "city": "Lisbon" },
  "displayCurrency": "USD"
}
```
- `city` is optional on each side.
- `displayCurrency` optional, default `USD`.

Response:
```json
{
  "displayCurrency": "USD",
  "current": { "amountConverted": 1680.0, "colIndex": 34.0, "realValue": 4941.2 },
  "offer":   { "amountConverted": 3780.0, "colIndex": 55.0, "realValue": 6872.7 },
  "fx": { "offerVsCurrentPct": 125.0 },
  "real": { "offerVsCurrentPct": 39.1, "available": true },
  "verdict": "The offer is worth about 125% more by exchange rate, and about 39% more in real purchasing power.",
  "notes": ["Exchange rates are today's rates.", "Cost-of-living indices are approximate."]
}
```
- `amountConverted`: salary converted to `displayCurrency` at **today's** rate (reuse the
  existing rate resolver, but with the current/latest rate — NOT historical carry-forward
  by an effective date; there is no effective date here).
- `realValue` = `amountConverted / (colIndex / 100)` — i.e. purchasing-power-normalized.
- `real.available = false` when either side has no COL match; in that case omit/zero the
  real numbers and add the appropriate note. The FX comparison must still work.
- `verdict`: build server-side in plain language so the client and the share-image use the
  same string.

### 4.2 Rate lookup

- Reuse the existing `RateSource` resolver and `daily_rates` cache. Use the **latest
  available** rate for each pair (most recent row in `daily_rates`), applying the same
  last-known carry-forward you already use. If a pair isn't cached yet, trigger the same
  on-demand fetch path the salary conversion uses. **Do not add a new rate provider.**
- KZT still routes through the National Bank of Kazakhstan source; everything else through
  Frankfurter — unchanged.

### 4.3 Cost-of-living service

- New file `backend/services/costofliving.go`: load the JSON once (prefer Go `embed` so
  it ships in the binary and needs no runtime file path), expose
  `Lookup(city, country string) (index float64, matched bool, level string)` where
  `level` ∈ {"city","country","none"}.

### 4.4 Validation (reuse spec §5.3 style)
- `amount` > 0 on both sides; valid ISO 4217 currency the resolver supports; reject far-future
  / nonsensical input; cap request body (you already enforce a 1 MiB cap globally).
- Fail with the standard `{"error": "message"}` shape (spec §6).

### 4.5 Tests (right-sized, per spec §7)
- **Unit (do first):** the comparison math — FX %, real % with COL, the `real.available=false`
  path when a city is unknown, same-currency no-op, verdict string generation. This is the
  risky logic; cover it thoroughly like you did for historical conversion.
- **Integration:** `POST /api/public/compare` happy path + unknown-city path + invalid input,
  with the rate source mocked (USD-only fixtures, matching your existing e2e "no external
  API flake" approach).
- Do **not** block this feature on new Playwright coverage; add one light e2e in §5.6.

---

## 5. Frontend

### 5.1 Route + nav
- New route `/compare` (or `/offer`), reachable **without login**. Add it to the landing
  page as a clear secondary CTA ("Comparing a job offer in another currency? Try the
  Offer Comparison →"). Keep the salary tracker as the primary CTA.
- Logged-in users get a nav link to it too.

### 5.2 Page: `OfferComparePage`
- Two input cards ("Your current salary" / "The offer"): amount, currency dropdown (reuse
  the existing currency dropdown component), optional city (a simple text input with a
  datalist of known cities from the dataset is enough — no fancy autocomplete needed).
- A display-currency dropdown (default USD), reusing the existing control.
- A "Compare" button that calls `POST /api/public/compare` via the existing API client
  (guest path — no `credentials: 'include'` needed for the public endpoint, matching
  `/api/public/compute`). Use React Query.

### 5.3 Result
- Two-bar ECharts comparison (reuse existing chart styling): one chart/bar-group for FX
  value, one for real (purchasing-power) value, current vs offer.
- A prominent **verdict line** (the server `verdict` string).
- The `notes` shown as small print, including the COL "approximate" disclaimer.
- If `real.available === false`, hide the real chart and show the note; keep the FX chart.

### 5.4 Guest persistence
- Persist the last comparison inputs in `sessionStorage` (mirrors existing guest mode) so a
  refresh doesn't wipe them. No account required.

### 5.5 Payoff-before-effort (important, cheap)
- Pre-fill the form with a compelling default example on first load (e.g. current
  800000 KZT / Almaty vs offer 3500 EUR / Lisbon) and **auto-run the comparison once** so a
  first-time visitor sees a filled result immediately, before typing anything. Then let them
  edit. This is the single highest-leverage UX choice — do not skip it.

### 5.6 Tests
- Vitest unit for the result-formatting util.
- One light Playwright e2e: load `/compare`, the pre-filled example renders a verdict,
  change one amount, re-compare, verdict updates. USD/known-city fixtures only.

---

## 6. Share-image export (do it here — high leverage)

The whole growth thesis depends on the result being shareable. Add a **"Share this result"**
button on the comparison result that renders the verdict + the two-bar chart to a PNG the
user can download / share.

- Implement client-side (e.g. render the result card to canvas via `html-to-image` or an
  ECharts `getDataURL()` composite — pick the lighter dependency; ECharts can already export
  its own canvas, so compositing the verdict text + the chart's dataURL onto one canvas
  avoids a heavy new dep).
- Include a small watermark: the site name + URL, so shared images drive traffic back.
- No backend, no storage, no cost.
- Add the same export button to the **salary tracker chart** if cheap — but if it risks
  scope creep, ship it for the comparison page first and file a follow-up.

> This button is the marketing engine. Treat it as a first-class part of the feature, not a
> nice-to-have.

---

## 7. Analytics (owner-only, reuse existing Metrica)

Add Yandex Metrica goal events (counter 110422979) for the funnel — **behavior only, never
amounts or currencies**, consistent with your existing 12 goals and the "never amounts" rule:
- `compare_page_view`
- `compare_run` (a comparison was computed)
- `compare_result_shared` (share-image button clicked)
- `compare_city_unknown` (fired when COL data was missing — tells you which cities to add next)

Do **not** send salary figures, currencies, or cities as event values. Event fire only.

---

## 8. Docs + housekeeping
- Update `README.md` API table with `POST /api/public/compare`.
- Update `spec_v2.md` §2 with the Offer Comparison user story, and `todo_v2.md` STATUS.
- Add the cost-of-living dataset's `updated` date to the UI disclaimer so staleness is visible.
- License note: ensure the COL index values you seed are from a source whose terms permit
  redistribution; if unsure, derive/curate approximate values yourself and label them as
  original estimates rather than copying a proprietary table verbatim.

---

## 9. Definition of done
- [ ] `/compare` works in guest mode, pre-filled example auto-runs on first load.
- [ ] FX comparison correct; real (COL-adjusted) comparison correct; unknown-city path
      degrades gracefully to FX-only.
- [ ] Share-image button produces a watermarked PNG.
- [ ] Unit tests cover the comparison math + verdict; integration test covers the endpoint;
      one light e2e passes.
- [ ] Metrica goals fire (behavior only).
- [ ] `govulncheck` + `npm audit` clean; CI green; README/spec/todo updated.
- [ ] No change to existing auth, salary CRUD, or historical conversion behavior.

---

## 10. Sequencing for the agent (do in this order)
1. Backend: COL dataset + `costofliving.go` + unit tests for lookup.
2. Backend: `/api/public/compare` handler + comparison math + unit/integration tests.
3. Frontend: `OfferComparePage` + inputs + call + result charts (with pre-filled example).
4. Frontend: share-image export + watermark.
5. Metrica goals.
6. Docs (README/spec/todo) + light e2e.
7. Verify CI green locally (`docker compose run --rm backend go test ./...`, frontend build,
   Playwright) before pushing so the Render deploy gate passes.

Commit in small, reviewable steps. Tag a rollback point before starting
(e.g. `git tag pre-offer-compare`).
