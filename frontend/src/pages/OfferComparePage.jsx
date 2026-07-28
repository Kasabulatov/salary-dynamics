import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api, computeRetry, rateLimited } from '../api'
import { track } from '../analytics'
import { CURRENCIES } from '../currencies'
import AmountInput from '../components/AmountInput'
import CityPicker from '../components/CityPicker'
import CompareChart from '../components/CompareChart'
import { shareResult, shareLinks } from '../shareImage'
import { useWakingUp, WAKEUP_MESSAGE } from '../components/wakeup'

const STORAGE_KEY = 'compare_inputs'

// Compelling default example (feature plan §5.5): a first-time visitor sees
// a filled result before typing anything.
const DEFAULTS = {
  current: { amount: '800000', currency: 'KZT', city: 'Almaty' },
  offer: { amount: '3500', currency: 'EUR', city: 'Lisbon' },
  displayCurrency: 'USD',
}

function loadInputs() {
  try {
    const saved = JSON.parse(sessionStorage.getItem(STORAGE_KEY))
    if (saved?.current?.amount && saved?.offer?.amount) return saved
  } catch { /* fall through to defaults */ }
  return DEFAULTS
}

function SideCard({ title, side, onChange, cities }) {
  return (
    <div className="compare-card">
      <h3>{title}</h3>
      <label>
        Amount
        <AmountInput value={side.amount} onChange={(amount) => onChange({ ...side, amount })} />
      </label>
      <label>
        Currency
        <select value={side.currency}
          onChange={(e) => onChange({ ...side, currency: e.target.value })}>
          {CURRENCIES.map((c) => <option key={c} value={c}>{c}</option>)}
        </select>
      </label>
      <label>
        City <span className="muted">(optional, improves accuracy)</span>
        <CityPicker value={side.city} cities={cities}
          onChange={(city) => onChange({ ...side, city })} />
      </label>
    </div>
  )
}

const fmt = (v) => Number(v).toLocaleString(undefined, { maximumFractionDigits: 0 })

export default function OfferComparePage() {
  const [inputs, setInputs] = useState(loadInputs)
  // The submitted payload IS the query key: the initial value auto-runs the
  // pre-filled example on first load (payoff before effort), clicking
  // Compare submits a new key, and React Query guarantees latest-wins,
  // dedupes StrictMode double-fetches, and caches repeat comparisons.
  const [submitted, setSubmitted] = useState(loadInputs)
  const chartRef = useRef(null)

  const metaQ = useQuery({
    queryKey: ['compare-meta'],
    queryFn: () => api('/api/public/compare/meta'),
    staleTime: Infinity,
  })

  const compareQ = useQuery({
    queryKey: ['compare', submitted],
    queryFn: () => api('/api/public/compare', {
      method: 'POST',
      body: {
        current: { ...submitted.current, amount: Number(submitted.current.amount) },
        offer: { ...submitted.offer, amount: Number(submitted.offer.amount) },
        displayCurrency: submitted.displayCurrency,
      },
    }),
    staleTime: Infinity,
    ...computeRetry,
  })
  const result = compareQ.data ?? null
  const waking = useWakingUp(compareQ.isFetching)

  useEffect(() => {
    track('compare_page_view', {})
  }, [])

  // Per successful comparison: persist the inputs, fire the funnel goals.
  useEffect(() => {
    if (!compareQ.data) return
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(submitted))
    track('compare_run', {})
    if (!compareQ.data.real.available) track('compare_city_unknown', {})
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [compareQ.dataUpdatedAt])

  const runnable = Number(inputs.current.amount) > 0 && Number(inputs.offer.amount) > 0

  const onShare = async () => {
    track('compare_result_shared', {})
    try {
      await shareResult({ chartContainer: chartRef.current, verdict: result.verdict })
    } catch {
      alert('Could not create the image in this browser — try a screenshot instead.')
    }
  }

  return (
    <main>
      <header className="hero hero-compact">
        <h1>Is the offer actually better?</h1>
        <p className="hero-sub">
          Compare salaries across currencies and cities — by exchange rate,
          and by what the money really buys.
        </p>
      </header>

      <section className="section section-alt">
        <div className="section-inner">
          <div className="compare-grid">
            <SideCard title="Your current salary" side={inputs.current}
              cities={metaQ.data?.cities}
              onChange={(current) => setInputs({ ...inputs, current })} />
            <div className="compare-vs">vs</div>
            <SideCard title="The offer" side={inputs.offer}
              cities={metaQ.data?.cities}
              onChange={(offer) => setInputs({ ...inputs, offer })} />
          </div>

          <div className="toolbar compare-toolbar">
            <label className="compare-display">
              Compare in
              <select className="ccy-select" value={inputs.displayCurrency}
                onChange={(e) => setInputs({ ...inputs, displayCurrency: e.target.value })}>
                {CURRENCIES.map((c) => <option key={c} value={c}>{c}</option>)}
              </select>
            </label>
            <button className="btn btn-primary btn-large" disabled={!runnable || compareQ.isFetching}
              onClick={() => setSubmitted(structuredClone(inputs))}>
              {compareQ.isFetching ? 'Comparing…' : 'Compare'}
            </button>
          </div>

          {waking && (
            <p className="section-sub compare-waking">{WAKEUP_MESSAGE}</p>
          )}

          {compareQ.isError && (
            <div className="form-error compare-error">
              {rateLimited(compareQ.error)
                ? 'A lot of requests just now — wait a few seconds and press Compare again.'
                : compareQ.error.status
                  ? compareQ.error.message
                  : 'The server was asleep and may just have woken up — press Compare again.'}
            </div>
          )}

          {result && (
            <div className="chart-panel compare-result">
              <p className="compare-verdict">{result.verdict}</p>
              <div className="compare-summary">
                <span>
                  Current: <strong>{fmt(result.current.amountConverted)} {result.displayCurrency}</strong>
                  {result.current.realValue != null &&
                    ` · real ${fmt(result.current.realValue)}`}
                </span>
                <span>
                  Offer: <strong>{fmt(result.offer.amountConverted)} {result.displayCurrency}</strong>
                  {result.offer.realValue != null &&
                    ` · real ${fmt(result.offer.realValue)}`}
                </span>
              </div>
              <div ref={chartRef}>
                <CompareChart result={result} />
              </div>
              <div className="compare-actions">
                <button className="btn btn-primary" onClick={onShare}>
                  Share this result
                </button>
                <div className="share-links">
                  {Object.entries(shareLinks(result.verdict)).map(([name, url]) => (
                    <a key={name} href={url} target="_blank" rel="noopener noreferrer"
                      className="btn btn-mini"
                      onClick={() => track('compare_result_shared', {})}>
                      {name === 'telegram' ? 'Telegram' : name === 'whatsapp' ? 'WhatsApp' : 'LinkedIn'}
                    </a>
                  ))}
                </div>
              </div>
              <ul className="compare-notes">
                {result.notes.map((n, i) => <li key={i}>{n}</li>)}
              </ul>
            </div>
          )}
        </div>
      </section>
    </main>
  )
}
