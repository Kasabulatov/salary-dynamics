import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, API_BASE } from '../api'
import MetricsChart from '../components/MetricsChart'
import DateField from '../components/DateField'

const METRIC_PRESETS = [
  { key: '7d', label: 'Last 7 Days', days: 7 },
  { key: '30d', label: 'Last 30 Days', days: 30 },
  { key: '90d', label: 'Last 90 Days', days: 90 },
  { key: 'custom', label: 'Custom', days: null },
]

function metricRange(preset, customFrom, customTo) {
  const now = new Date()
  const p = METRIC_PRESETS.find((x) => x.key === preset)
  if (p?.days) {
    const from = new Date(now)
    from.setDate(from.getDate() - p.days)
    return [from, now]
  }
  const from = customFrom ? new Date(customFrom) : new Date(now.getFullYear(), 0, 1)
  const to = customTo ? new Date(customTo) : now
  return [from, to]
}

const OAUTH_ERRORS = {
  state_mismatch: 'The sign-in attempt expired — please try connecting again.',
  exchange_failed: 'Yandex rejected the sign-in. Please try again.',
  access_denied: 'You declined access — connect again when ready.',
  not_configured: 'Analytics is not configured on the server yet.',
  missing_code: 'Yandex did not return a code — please try again.',
  internal: 'Something went wrong on our side — please try again.',
}

export default function ProductMetricsPage() {
  const queryClient = useQueryClient()
  const [params, setParams] = useSearchParams()
  const [preset, setPreset] = useState('30d')
  const [customFrom, setCustomFrom] = useState('')
  const [customTo, setCustomTo] = useState('')
  const [counterID, setCounterID] = useState('')
  const [banner, setBanner] = useState(null)

  // Result of the OAuth redirect: show a one-time banner, clean the URL.
  useEffect(() => {
    const connected = params.get('connected')
    const error = params.get('error')
    if (connected) setBanner({ kind: 'ok', text: 'Yandex Metrica connected.' })
    if (error) setBanner({ kind: 'err', text: OAUTH_ERRORS[error] || `Connection failed (${error}).` })
    if (connected || error) setParams({}, { replace: true })
  }, [params, setParams])

  const connectionsQ = useQuery({
    queryKey: ['analytics-connections'],
    queryFn: () => api('/api/analytics/connections'),
  })
  const yandexConnected = connectionsQ.data?.connections?.some((c) => c.provider === 'yandex')
  const configured = connectionsQ.data?.configured

  const countersQ = useQuery({
    queryKey: ['analytics-counters'],
    queryFn: () => api('/api/analytics/counters'),
    enabled: !!yandexConnected,
  })
  const counters = countersQ.data?.counters ?? []
  useEffect(() => {
    if (!counterID && counters.length) setCounterID(String(counters[0].id))
  }, [counters, counterID])

  const [from, to] = metricRange(preset, customFrom, customTo)
  const isoFrom = from.toISOString().slice(0, 10)
  const isoTo = to.toISOString().slice(0, 10)

  const dataQ = useQuery({
    queryKey: ['analytics-data', counterID, isoFrom, isoTo],
    queryFn: () => api(`/api/analytics/data?counter=${counterID}&from=${isoFrom}&to=${isoTo}`),
    enabled: !!counterID,
  })

  const disconnectMut = useMutation({
    mutationFn: () => api('/api/analytics/connections?provider=yandex', { method: 'DELETE' }),
    onSuccess: () => {
      setCounterID('')
      queryClient.invalidateQueries({ queryKey: ['analytics-connections'] })
    },
  })

  const totals = useMemo(() => {
    const pts = dataQ.data?.points ?? []
    return pts.reduce(
      (acc, p) => ({
        users: acc.users + p.users,
        sessions: acc.sessions + p.sessions,
        pageviews: acc.pageviews + p.pageviews,
      }),
      { users: 0, sessions: 0, pageviews: 0 },
    )
  }, [dataQ.data])

  return (
    <main>
      <header className="hero hero-compact">
        <h1>Product Metrics.</h1>
        <p className="hero-sub">Your site's pulse — users, sessions, pageviews.</p>
      </header>

      <section className="section section-alt">
        <div className="section-inner">
          {banner && (
            <div className={banner.kind === 'ok' ? 'guest-import-note' : 'form-error'}>
              {banner.text}
            </div>
          )}

          {connectionsQ.isLoading && <div className="chart-empty">Loading…</div>}

          {connectionsQ.data && !yandexConnected && (
            <div className="connect-card">
              <h2 className="section-title">Connect Yandex Metrica.</h2>
              <p className="section-sub">
                Read-only access to your counters. Tokens are encrypted at rest;
                disconnect anytime.
              </p>
              <div className="hero-ctas">
                <button
                  className="btn btn-primary btn-large"
                  disabled={!configured}
                  onClick={() => { window.location.href = `${API_BASE}/api/oauth/yandex/start` }}
                >
                  Connect Yandex Metrica
                </button>
              </div>
              {!configured && (
                <p className="section-sub muted">
                  (Server is missing Yandex OAuth credentials — see .env)
                </p>
              )}
            </div>
          )}

          {yandexConnected && (
            <>
              <div className="toolbar">
                <div className="segmented">
                  {METRIC_PRESETS.map((p) => (
                    <button key={p.key} className={preset === p.key ? 'active' : ''}
                      onClick={() => setPreset(p.key)}>
                      {p.label}
                    </button>
                  ))}
                </div>
                {counters.length > 0 && (
                  <select className="ccy-select" value={counterID}
                    onChange={(e) => setCounterID(e.target.value)} aria-label="Counter">
                    {counters.map((c) => (
                      <option key={c.id} value={c.id}>{c.name || c.site || c.id}</option>
                    ))}
                  </select>
                )}
                <button className="link-btn danger" onClick={() => disconnectMut.mutate()}>
                  Disconnect
                </button>
              </div>
              {preset === 'custom' && (
                <div className="toolbar toolbar-custom">
                  <DateField value={customFrom} onChange={setCustomFrom} />
                  <span className="muted">to</span>
                  <DateField value={customTo} onChange={setCustomTo} />
                </div>
              )}

              <div className="chart-panel">
                {countersQ.isError && (
                  <div className="chart-empty">
                    {countersQ.error?.status === 401
                      ? 'Yandex session expired — disconnect and connect again.'
                      : 'Could not load your counters.'}
                  </div>
                )}
                {counters.length === 0 && countersQ.isSuccess && (
                  <div className="chart-empty">
                    No counters on this Yandex account yet — create one at metrika.yandex.com.
                  </div>
                )}
                {dataQ.isLoading && counterID && <div className="chart-empty">Loading traffic…</div>}
                {dataQ.isError && <div className="chart-empty">Could not load traffic data.</div>}
                {dataQ.data && (
                  <>
                    <div className="metric-totals">
                      <span><strong>{totals.users.toLocaleString()}</strong> users</span>
                      <span><strong>{totals.sessions.toLocaleString()}</strong> sessions</span>
                      <span><strong>{totals.pageviews.toLocaleString()}</strong> pageviews</span>
                    </div>
                    <MetricsChart points={dataQ.data.points} />
                  </>
                )}
              </div>
            </>
          )}
        </div>
      </section>
    </main>
  )
}
