import { useEffect, useRef, useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { api } from '../api'
import { track } from '../analytics'
import { CURRENCIES } from '../currencies'
import AmountInput from '../components/AmountInput'
import CompareChart from '../components/CompareChart'
import { downloadShareImage } from '../shareImage'

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
        <input type="text" value={side.city} list="col-cities" placeholder="e.g. Almaty"
          onChange={(e) => onChange({ ...side, city: e.target.value })} />
      </label>
      {cities}
    </div>
  )
}

const fmt = (v) => Number(v).toLocaleString(undefined, { maximumFractionDigits: 0 })

export default function OfferComparePage() {
  const [inputs, setInputs] = useState(loadInputs)
  const [result, setResult] = useState(null)
  const chartRef = useRef(null)

  const metaQ = useQuery({
    queryKey: ['compare-meta'],
    queryFn: () => api('/api/public/compare/meta'),
    staleTime: Infinity,
  })

  const compareMut = useMutation({
    mutationFn: (payload) => api('/api/public/compare', {
      method: 'POST',
      body: {
        current: { ...payload.current, amount: Number(payload.current.amount) },
        offer: { ...payload.offer, amount: Number(payload.offer.amount) },
        displayCurrency: payload.displayCurrency,
      },
    }),
    onSuccess: (data, payload) => {
      setResult(data)
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(payload))
      track('compare_run', {})
      if (!data.real.available) track('compare_city_unknown', {})
    },
  })

  // Payoff before effort: auto-run once on first load with the (restored or
  // default) inputs, so the page never starts empty.
  const autoRan = useRef(false)
  useEffect(() => {
    if (autoRan.current) return
    autoRan.current = true
    track('compare_page_view', {})
    compareMut.mutate(loadInputs())
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const runnable = Number(inputs.current.amount) > 0 && Number(inputs.offer.amount) > 0

  const onShare = async () => {
    track('compare_result_shared', {})
    try {
      await downloadShareImage({ chartContainer: chartRef.current, verdict: result.verdict })
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
          <datalist id="col-cities">
            {(metaQ.data?.cities ?? []).map((c) => (
              <option key={`${c.city}-${c.country}`} value={c.city} />
            ))}
          </datalist>

          <div className="compare-grid">
            <SideCard title="Your current salary" side={inputs.current}
              onChange={(current) => setInputs({ ...inputs, current })} />
            <div className="compare-vs">vs</div>
            <SideCard title="The offer" side={inputs.offer}
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
            <button className="btn btn-primary btn-large" disabled={!runnable || compareMut.isPending}
              onClick={() => compareMut.mutate(inputs)}>
              {compareMut.isPending ? 'Comparing…' : 'Compare'}
            </button>
          </div>

          {compareMut.isError && (
            <div className="form-error compare-error">{compareMut.error.message}</div>
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
