import {
  ResponsiveContainer, LineChart, Line, XAxis, YAxis,
  CartesianGrid, Tooltip,
} from 'recharts'

const fmtDate = (ts) =>
  new Date(ts).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })

const fmtMoney = (v, ccy) =>
  `${Number(v).toLocaleString(undefined, { maximumFractionDigits: 2 })} ${ccy}`

// Dot is rendered only for real salary-change events (spec §2.2).
function EntryDot({ cx, cy, payload }) {
  if (!payload.isEntry) return null
  return <circle cx={cx} cy={cy} r={5} fill="#0071e3" stroke="#fff" strokeWidth={2} />
}

function ChartTooltip({ active, payload, displayCurrency }) {
  if (!active || !payload?.length) return null
  const p = payload[0].payload
  const e = p.entry
  return (
    <div className="chart-tooltip">
      <div className="chart-tooltip-date">{fmtDate(p.ts)}</div>
      <div className="chart-tooltip-value">{fmtMoney(p.value, displayCurrency)}</div>
      {e && e.currency_code !== displayCurrency && (
        <div className="chart-tooltip-orig">
          = {fmtMoney(e.amount, e.currency_code)}
          {e.rate_date && <span> (rate of {e.rate_date})</span>}
        </div>
      )}
      {p.isEntry && e?.note && <div className="chart-tooltip-note">“{e.note}”</div>}
      {p.isEntry && <div className="chart-tooltip-note muted">salary change</div>}
    </div>
  )
}

export default function SalaryChart({ points, from, to, displayCurrency }) {
  if (!points.length) {
    return <div className="chart-empty">No salary entries in this period.</div>
  }

  // Fit the X axis to the data: start at the first visible point (which is
  // the range start when an older salary carries in, or the earliest entry
  // otherwise — so deleting the first entry re-anchors the axis).
  let domainFrom = points[0].ts
  let domainTo = points[points.length - 1].ts
  if (domainTo <= domainFrom) {
    // Single-point safety: pad a month around it.
    domainFrom -= 15 * 86400e3
    domainTo += 15 * 86400e3
  }
  const spanDays = (domainTo - domainFrom) / 86400e3
  const fmtTick = (ts) =>
    new Date(ts).toLocaleDateString(undefined,
      spanDays > 365
        ? { year: 'numeric', month: 'short' }
        : { month: 'short', day: 'numeric' })

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
        <Tooltip content={<ChartTooltip displayCurrency={displayCurrency} />} />
        <Line
          type="linear" dataKey="value" stroke="#0071e3" strokeWidth={2}
          dot={<EntryDot />} activeDot={{ r: 6, fill: '#0071e3' }} isAnimationActive={false}
        />
      </LineChart>
    </ResponsiveContainer>
  )
}
