import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { publicCompute } from '../api'
import { buildChartPoints, targetMapFrom } from '../chartData'
import { getGuestEntries, addGuestEntry, updateGuestEntry, deleteGuestEntry } from '../guestStore'
import { CURRENCIES, PRESETS, rangeForPreset } from '../currencies'
import SalaryChart from '../components/SalaryChart'
import SalaryEntryForm from '../components/SalaryEntryForm'
import DateField from '../components/DateField'

const entryKey = (e) => `${e.effective_date.slice(0, 10)}|${e.amount}|${e.currency_code}`

// Guest mode: the full tracker, but entries live in sessionStorage and all
// computation happens via the stateless public endpoint. Nothing is stored
// server-side; closing the tab clears the data.
export default function GuestTrackerPage() {
  const [entries, setEntries] = useState(getGuestEntries)
  const [displayCurrency, setDisplayCurrency] = useState('USD')
  const [preset, setPreset] = useState('5y')
  const [customFrom, setCustomFrom] = useState('')
  const [customTo, setCustomTo] = useState('')
  const [editingEntry, setEditingEntry] = useState(null)
  const [showInflation, setShowInflation] = useState(true)

  const [from, to] = rangeForPreset(preset, customFrom, customTo)
  const isoFrom = from.toISOString().slice(0, 10)
  const isoTo = to.toISOString().slice(0, 10)

  const entriesKey = JSON.stringify(entries.map(entryKey).sort())
  const computeQ = useQuery({
    queryKey: ['guest-compute', entriesKey, displayCurrency, isoFrom, isoTo],
    queryFn: () => publicCompute({
      entries: entries.map(({ amount, currency_code, effective_date, note }) =>
        ({ amount, currency_code, effective_date, note })),
      displayCurrency, from: isoFrom, to: isoTo,
    }),
    enabled: entries.length > 0,
  })

  const computed = computeQ.data

  // Map computed conversions back onto local entries (for the table).
  const tableEntries = useMemo(() => {
    const byKey = new Map((computed?.entries ?? []).map((e) => [entryKey(e), e]))
    return [...entries]
      .sort((a, b) => a.effective_date.localeCompare(b.effective_date))
      .map((e) => ({ ...e, computed: byKey.get(entryKey(e)) }))
  }, [entries, computed])

  const points = useMemo(() => {
    if (!computed) return []
    const targetMap = showInflation ? targetMapFrom(computed.inflation?.points) : null
    return buildChartPoints(computed.series.points, computed.entries, targetMap)
  }, [computed, showInflation])

  const busy = entries.length > 0 && computeQ.isLoading

  return (
    <main>
      <div className="guest-banner">
        <span>
          <strong>Guest mode</strong> — your entries live only in this browser tab
          and disappear when it closes.
        </span>
        <Link to="/register" className="btn btn-primary">Create account to keep them</Link>
      </div>

      <section className="section section-alt guest-first-section">
        <div className="section-inner">
          <div className="toolbar">
            <div className="segmented">
              {PRESETS.map((p) => (
                <button key={p.key} className={preset === p.key ? 'active' : ''}
                  onClick={() => setPreset(p.key)}>
                  {p.label}
                </button>
              ))}
            </div>
            <select className="ccy-select" value={displayCurrency}
              onChange={(e) => setDisplayCurrency(e.target.value)} aria-label="Display currency">
              {CURRENCIES.map((c) => <option key={c} value={c}>{c}</option>)}
            </select>
            <button className={`toggle-pill ${showInflation ? 'active' : ''}`}
              onClick={() => setShowInflation((v) => !v)}>
              Inflation target
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
            {entries.length === 0 && (
              <div className="chart-empty">Add your first salary entry below — the chart appears instantly.</div>
            )}
            {busy && <div className="chart-empty">Computing with real historical rates…</div>}
            {computeQ.isError && (
              <div className="chart-empty">
                Could not compute right now.{' '}
                <button className="btn btn-primary" onClick={() => computeQ.refetch()}>Try again</button>
              </div>
            )}
            {computed && !busy && (
              <>
                <SalaryChart points={points} from={from} to={to}
                  displayCurrency={displayCurrency} events={computed.events} />
                <div className="chart-legend">
                  <span><span className="legend-dot" /> salary change</span>
                  <span><span className="legend-band red" /> sharp drop</span>
                  <span><span className="legend-band green" /> sharp rise</span>
                  {showInflation && computed.inflation && (
                    <span><span className="legend-target" /> inflation target ({computed.inflation.country_code} CPI)</span>
                  )}
                </div>
              </>
            )}
          </div>
        </div>
      </section>

      <section className="section">
        <div className="section-inner narrow">
          <h2 className="section-title">{editingEntry ? 'Edit entry.' : 'Add a salary entry.'}</h2>
          <p className="section-sub">
            {editingEntry
              ? `Updating the entry effective ${editingEntry.effective_date.slice(0, 10)}.`
              : 'Each entry starts a new chapter of your timeline.'}
          </p>
          {editingEntry ? (
            <SalaryEntryForm
              key={`edit-${editingEntry.id}`}
              initial={editingEntry}
              onSubmit={async (body) => {
                setEntries(updateGuestEntry(editingEntry.id, body))
                setEditingEntry(null)
              }}
              onCancel={() => setEditingEntry(null)}
            />
          ) : (
            <SalaryEntryForm onSubmit={async (body) => setEntries(addGuestEntry(body))} />
          )}
        </div>
      </section>

      <section className="section section-alt">
        <div className="section-inner">
          <h2 className="section-title">History.</h2>
          {tableEntries.length === 0 ? (
            <p className="section-sub">No entries yet — everything you add stays in this tab only.</p>
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
                {tableEntries.map((e) => (
                  <tr key={e.id}>
                    <td>{e.effective_date.slice(0, 10)}</td>
                    <td className="num">{Number(e.amount).toLocaleString()} {e.currency_code}</td>
                    <td className="num">
                      {e.computed?.converted_amount != null
                        ? e.computed.converted_amount.toLocaleString(undefined, { maximumFractionDigits: 2 })
                        : <span className="muted">…</span>}
                    </td>
                    <td className="note-cell">{e.note || ''}</td>
                    <td className="row-actions">
                      <button className="link-btn" onClick={() => setEditingEntry(e)}>Edit</button>
                      <button className="link-btn danger"
                        onClick={() => {
                          if (confirm('Delete this entry?')) setEntries(deleteGuestEntry(e.id))
                        }}>
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
