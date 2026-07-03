import { useMemo, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api'
import { CURRENCIES, PRESETS, rangeForPreset } from '../currencies'
import SalaryChart from '../components/SalaryChart'
import SalaryEntryForm from '../components/SalaryEntryForm'
import DateField from '../components/DateField'

// Merge the backend's daily series (one value per day at that day's rate)
// with entry metadata, so salary-change days get a dot + note in the tooltip.
function buildChartPoints(seriesPoints, entries) {
  const sorted = [...entries].sort((a, b) => a.effective_date.localeCompare(b.effective_date))
  const entryByDate = new Map(sorted.map((e) => [e.effective_date.slice(0, 10), e]))
  let idx = 0
  let active = null
  return seriesPoints.map((p) => {
    while (idx < sorted.length && sorted[idx].effective_date.slice(0, 10) <= p.date) {
      active = sorted[idx]
      idx++
    }
    const isEntry = entryByDate.has(p.date)
    return {
      ts: new Date(p.date + 'T00:00:00Z').getTime(),
      value: p.value,
      entry: isEntry ? entryByDate.get(p.date) : active,
      isEntry,
    }
  })
}

const fmtBig = (v) => v.toLocaleString(undefined, { maximumFractionDigits: 0 })

export default function SalaryTrackerPage() {
  const queryClient = useQueryClient()
  const [displayCurrency, setDisplayCurrency] = useState('USD')
  const [preset, setPreset] = useState('5y')
  const [customFrom, setCustomFrom] = useState('')
  const [customTo, setCustomTo] = useState('')
  const [editingEntry, setEditingEntry] = useState(null)

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['salary', displayCurrency],
    queryFn: () => api(`/api/salary?displayCurrency=${displayCurrency}`),
  })

  const [from, to] = rangeForPreset(preset, customFrom, customTo)
  const isoFrom = from.toISOString().slice(0, 10)
  const isoTo = to.toISOString().slice(0, 10)

  // Daily dynamics line: salary in effect each day, at that day's rate.
  const seriesQ = useQuery({
    queryKey: ['series', displayCurrency, isoFrom, isoTo],
    queryFn: () => api(`/api/salary/series?displayCurrency=${displayCurrency}&from=${isoFrom}&to=${isoTo}`),
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['salary'] })
    queryClient.invalidateQueries({ queryKey: ['series'] })
  }

  const createMut = useMutation({
    mutationFn: (body) => api('/api/salary', { method: 'POST', body }),
    onSuccess: invalidate,
  })
  const updateMut = useMutation({
    mutationFn: ({ id, body }) => api(`/api/salary/${id}`, { method: 'PUT', body }),
    onSuccess: () => {
      setEditingEntry(null)
      invalidate()
    },
  })
  const deleteMut = useMutation({
    mutationFn: (id) => api(`/api/salary/${id}`, { method: 'DELETE' }),
    onSuccess: invalidate,
  })

  const entries = data?.entries ?? []
  const seriesPoints = seriesQ.data?.points ?? []
  const points = useMemo(() => buildChartPoints(seriesPoints, entries), [seriesPoints, entries])

  // Hero stat: today's value + 30-day movement.
  const latest = points.length ? points[points.length - 1] : null
  let delta = null
  let deltaPct = null
  if (latest) {
    const cutoff = latest.ts - 30 * 86400e3
    let ref = null
    for (const p of points) {
      if (p.ts <= cutoff) ref = p
      else break
    }
    if (ref && ref.value) {
      delta = latest.value - ref.value
      deltaPct = (delta / ref.value) * 100
    }
  }

  const busy = isLoading || seriesQ.isLoading
  const failed = (isError || seriesQ.isError) && !busy

  return (
    <main>
      {/* ——— Hero ——— */}
      <header className="hero">
        <h1>Salary Dynamics.</h1>
        <p className="hero-sub">What your salary is really worth. Every single day.</p>
        {latest && (
          <div className="hero-stat">
            <div className="hero-value">
              {fmtBig(latest.value)}
              <span className="hero-ccy">{displayCurrency}</span>
            </div>
            {delta != null && (
              <div className={`hero-delta ${delta >= 0 ? 'up' : 'down'}`}>
                {delta >= 0 ? '▲' : '▼'} {fmtBig(Math.abs(delta))} {displayCurrency}
                {' '}({deltaPct >= 0 ? '+' : ''}{deltaPct.toFixed(1)}%) in the past 30 days
              </div>
            )}
          </div>
        )}
      </header>

      {/* ——— Chart ——— */}
      <section className="section section-alt">
        <div className="section-inner">
          <div className="toolbar">
            <div className="segmented">
              {PRESETS.map((p) => (
                <button
                  key={p.key}
                  className={preset === p.key ? 'active' : ''}
                  onClick={() => setPreset(p.key)}
                >
                  {p.label}
                </button>
              ))}
            </div>
            <select
              className="ccy-select"
              value={displayCurrency}
              onChange={(e) => setDisplayCurrency(e.target.value)}
              aria-label="Display currency"
            >
              {CURRENCIES.map((c) => <option key={c} value={c}>{c}</option>)}
            </select>
          </div>
          {preset === 'custom' && (
            <div className="toolbar toolbar-custom">
              <DateField value={customFrom} onChange={setCustomFrom} />
              <span className="muted">to</span>
              <DateField value={customTo} onChange={setCustomTo} />
            </div>
          )}

          <div className="chart-panel">
            {busy && (
              <div className="chart-empty">
                Loading daily rates… (the first load of a long period can take up to a minute)
              </div>
            )}
            {failed && (
              <div className="chart-empty">
                Could not load your salary data.{' '}
                <button className="btn btn-primary" onClick={() => { refetch(); seriesQ.refetch() }}>
                  Try again
                </button>
              </div>
            )}
            {!busy && !failed && (
              <SalaryChart points={points} from={from} to={to} displayCurrency={displayCurrency} />
            )}
          </div>
        </div>
      </section>

      {/* ——— Add / Edit ——— */}
      <section className="section">
        <div className="section-inner narrow">
          <h2 className="section-title">
            {editingEntry ? 'Edit entry.' : 'Add a salary entry.'}
          </h2>
          <p className="section-sub">
            {editingEntry
              ? `Updating the entry effective ${editingEntry.effective_date.slice(0, 10)}.`
              : 'Each entry starts a new chapter of your timeline.'}
          </p>
          {editingEntry ? (
            <SalaryEntryForm
              key={`edit-${editingEntry.id}`}
              initial={editingEntry}
              busy={updateMut.isPending}
              onSubmit={(body) => updateMut.mutateAsync({ id: editingEntry.id, body })}
              onCancel={() => setEditingEntry(null)}
            />
          ) : (
            <SalaryEntryForm
              busy={createMut.isPending}
              onSubmit={(body) => createMut.mutateAsync(body)}
            />
          )}
        </div>
      </section>

      {/* ——— History ——— */}
      <section className="section section-alt">
        <div className="section-inner">
          <h2 className="section-title">History.</h2>
          {entries.length === 0 && !isLoading ? (
            <p className="section-sub">No entries yet — add your first salary above.</p>
          ) : (
            <table className="entries-table">
              <thead>
                <tr>
                  <th>Effective from</th>
                  <th className="num">Amount</th>
                  <th className="num">In {displayCurrency}</th>
                  <th>Note</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {entries.map((e) => (
                  <tr key={e.id}>
                    <td>{e.effective_date.slice(0, 10)}</td>
                    <td className="num">{e.amount.toLocaleString()} {e.currency_code}</td>
                    <td className="num">
                      {e.converted_amount != null
                        ? e.converted_amount.toLocaleString(undefined, { maximumFractionDigits: 2 })
                        : <span className="muted" title={e.conversion_error}>n/a</span>}
                    </td>
                    <td className="note-cell">{e.note || ''}</td>
                    <td className="row-actions">
                      <button className="link-btn" onClick={() => setEditingEntry(e)}>Edit</button>
                      <button
                        className="link-btn danger"
                        onClick={() => {
                          if (confirm('Delete this entry?')) deleteMut.mutate(e.id)
                        }}
                      >
                        Delete
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </section>
    </main>
  )
}
