import {
  ResponsiveContainer, LineChart, Line, XAxis, YAxis,
  CartesianGrid, Tooltip, ReferenceArea,
} from 'recharts'
import { track } from '../analytics'

const DAY = 86400e3
const tsOf = (dateStr) => new Date(dateStr.slice(0, 10) + 'T00:00:00Z').getTime()

const fmtDate = (ts) =>
  new Date(ts).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })

const fmtMoney = (v, ccy) =>
  `${Number(v).toLocaleString(undefined, { maximumFractionDigits: 2 })} ${ccy}`

// Blue dot marks a salary-change entry (spec §2.2).
function EntryDot({ cx, cy, payload }) {
  if (!payload.isEntry) return null
  return <circle cx={cx} cy={cy} r={5} fill="#0071e3" stroke="#fff" strokeWidth={2} />
}

const windowLabel = { daily: 'in one day', weekly: 'over a week' }

// Which annual CPI is in effect on the hovered day, and the cumulative
// inflation applied since the baseline (mirrors the backend's Jan-1 steps).
function inflationInfo(ts, meta) {
  if (!meta?.rates) return null
  const hoverYear = new Date(ts).getUTCFullYear()
  const baseYear = new Date(meta.base_date + 'T00:00:00Z').getUTCFullYear()
  const appliedYear = hoverYear - 1
  const applied = appliedYear >= baseYear ? meta.rates[appliedYear] : undefined
  let cumulative = 1
  for (let y = baseYear; y <= hoverYear - 1; y++) {
    if (meta.rates[y] != null) cumulative *= 1 + meta.rates[y] / 100
  }
  return {
    appliedYear,
    applied,
    cumulativePct: (cumulative - 1) * 100,
    country: meta.country_code,
    baseYear,
  }
}

function ChartTooltip({ active, payload, displayCurrency, coveringEvents, valueByTs, inflationMeta }) {
  if (!active || !payload?.length) return null
  const p = payload[0].payload
  const e = p.entry
  const events = coveringEvents(p.ts)
  const inf = p.target != null ? inflationInfo(p.ts, inflationMeta) : null
  return (
    <div className="chart-tooltip">
      <div className="chart-tooltip-date">{fmtDate(p.ts)}</div>
      <div className="chart-tooltip-value">{fmtMoney(p.value, displayCurrency)}</div>
      {p.target != null && (
        <div className="chart-tooltip-target">
          inflation target: {fmtMoney(p.target, displayCurrency)}
          {inf && (
            <div className="chart-tooltip-target-detail">
              {inf.applied != null
                ? `${inf.country} CPI ${inf.appliedYear}: ${inf.applied >= 0 ? '+' : ''}${inf.applied.toFixed(1)}%`
                : `no inflation applied yet`}
              {inf.cumulativePct > 0 &&
                ` · since ${inf.baseYear}: +${inf.cumulativePct.toFixed(1)}%`}
            </div>
          )}
        </div>
      )}
      {e && e.currency_code !== displayCurrency && (
        <div className="chart-tooltip-orig">
          = {fmtMoney(e.amount, e.currency_code)}
          {e.rate_date && <span> (rate of {e.rate_date})</span>}
        </div>
      )}
      {p.isEntry && e?.note && <div className="chart-tooltip-note">“{e.note}”</div>}
      {p.isEntry && <div className="chart-tooltip-note muted">salary change</div>}
      {events.map((ev, i) => {
        const startTs = tsOf(ev.ref_date)
        const endTs = tsOf(ev.date)
        const before = valueByTs.get(startTs)
        const after = valueByTs.get(endTs)
        const drop = ev.percent_change < 0
        return (
          <div key={i} className={`chart-tooltip-event ${drop ? 'down' : 'up'}`}>
            <div className="chart-tooltip-event-head">
              {ev.base_currency} {drop ? 'weakened' : 'strengthened'}{' '}
              {Math.abs(ev.percent_change).toFixed(2)}% vs {ev.quote_currency}{' '}
              {windowLabel[ev.change_window] || ev.change_window}
            </div>
            {before != null && after != null && (
              <div className="chart-tooltip-event-abs">
                your value: {fmtMoney(before, displayCurrency)} → {fmtMoney(after, displayCurrency)}{' '}
                ({after - before >= 0 ? '+' : '−'}{fmtMoney(Math.abs(after - before), displayCurrency)})
              </div>
            )}
            <div className="chart-tooltip-event-period">
              compared: {fmtDate(startTs)} → {fmtDate(endTs)}
            </div>
            {ev.news_headline && (
              <div className="chart-tooltip-news">
                📰 “{ev.news_headline}”
                {ev.news_url && <div className="chart-tooltip-news-hint">click the shaded area to read</div>}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

export default function SalaryChart({ points, from, to, displayCurrency, events = [], inflationMeta = null }) {
  if (!points.length) {
    return <div className="chart-empty">No salary entries in this period.</div>
  }

  // Fit the X axis to the data (deleting the first entry re-anchors the axis).
  let domainFrom = points[0].ts
  let domainTo = points[points.length - 1].ts
  if (domainTo <= domainFrom) {
    domainFrom -= 15 * DAY
    domainTo += 15 * DAY
  }
  const span = domainTo - domainFrom
  const spanDays = span / DAY
  const fmtTick = (ts) =>
    new Date(ts).toLocaleDateString(undefined,
      spanDays > 365
        ? { year: 'numeric', month: 'short' }
        : { month: 'short', day: 'numeric' })

  const valueByTs = new Map(points.map((p) => [p.ts, p.value]))

  // Event bands: [ref_date, date], clamped to the domain, with a minimum
  // visual width so 1-day events stay hoverable on a 5-year view.
  const minWidth = span * 0.008
  const bands = events
    .map((ev, i) => {
      let x1 = Math.max(tsOf(ev.ref_date), domainFrom)
      let x2 = Math.min(tsOf(ev.date), domainTo)
      if (x2 <= x1) return null
      if (x2 - x1 < minWidth) {
        const mid = (x1 + x2) / 2
        x1 = Math.max(domainFrom, mid - minWidth / 2)
        x2 = Math.min(domainTo, mid + minWidth / 2)
      }
      return { key: `band-${i}`, x1, x2, drop: ev.percent_change < 0, newsUrl: ev.news_url || null }
    })
    .filter(Boolean)

  const coveringEvents = (ts) =>
    events.filter((ev) => tsOf(ev.ref_date) - DAY / 2 <= ts && ts <= tsOf(ev.date) + DAY / 2)

  const hasTarget = points.some((p) => p.target != null)

  return (
    <ResponsiveContainer width="100%" height={340}>
      <LineChart data={points} margin={{ top: 12, right: 24, bottom: 4, left: 8 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="#e8e8ed" />
        <XAxis
          dataKey="ts" type="number" scale="time"
          domain={[domainFrom, domainTo]}
          tickFormatter={fmtTick} tick={{ fontSize: 12, fill: '#86868b' }}
          axisLine={{ stroke: '#d2d2d7' }} tickLine={false}
        />
        <YAxis
          tickFormatter={(v) => v.toLocaleString(undefined, { notation: 'compact' })}
          tick={{ fontSize: 12, fill: '#86868b' }} width={70}
          axisLine={false} tickLine={false}
        />
        {bands.map((b) => (
          <ReferenceArea
            key={b.key} x1={b.x1} x2={b.x2}
            fill={b.drop ? '#d70015' : '#248a3d'} fillOpacity={0.09}
            stroke={b.drop ? '#d70015' : '#248a3d'} strokeOpacity={0.25}
            // Bands with a news article open it on click.
            shape={(props) => (
              <rect
                x={props.x} y={props.y} width={props.width} height={props.height}
                fill={b.drop ? '#d70015' : '#248a3d'} fillOpacity={0.09}
                stroke={b.drop ? '#d70015' : '#248a3d'} strokeOpacity={0.25}
                style={b.newsUrl ? { cursor: 'pointer' } : undefined}
                onClick={b.newsUrl ? () => {
                  track('news_band_clicked', {})
                  window.open(b.newsUrl, '_blank', 'noopener')
                } : undefined}
              />
            )}
          />
        ))}
        <Tooltip
          content={
            <ChartTooltip
              displayCurrency={displayCurrency}
              coveringEvents={coveringEvents}
              valueByTs={valueByTs}
              inflationMeta={inflationMeta}
            />
          }
        />
        {hasTarget && (
          <Line
            type="stepAfter" dataKey="target" stroke="#b25000" strokeWidth={1.5}
            strokeDasharray="6 4" dot={false} isAnimationActive={false}
          />
        )}
        <Line
          type="linear" dataKey="value" stroke="#0071e3" strokeWidth={2}
          dot={<EntryDot />} activeDot={{ r: 6, fill: '#0071e3' }} isAnimationActive={false}
        />
      </LineChart>
    </ResponsiveContainer>
  )
}
