import {
  ResponsiveContainer, LineChart, Line, XAxis, YAxis,
  CartesianGrid, Tooltip, Legend,
} from 'recharts'

const fmtDate = (ts) =>
  new Date(ts).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })

const SERIES = [
  { key: 'users', label: 'Users', color: '#0071e3' },
  { key: 'sessions', label: 'Sessions', color: '#248a3d' },
  { key: 'pageviews', label: 'Pageviews', color: '#b25000' },
]

function MetricsTooltip({ active, payload, label }) {
  if (!active || !payload?.length) return null
  return (
    <div className="chart-tooltip">
      <div className="chart-tooltip-date">{fmtDate(label)}</div>
      {payload.map((p) => (
        <div key={p.dataKey} style={{ color: p.stroke }}>
          {SERIES.find((s) => s.key === p.dataKey)?.label}: {Number(p.value).toLocaleString()}
        </div>
      ))}
    </div>
  )
}

// Traffic metrics over time (Users / Sessions / Pageviews), spec §2.4.
export default function MetricsChart({ points }) {
  if (!points.length) {
    return <div className="chart-empty">No traffic data in this period.</div>
  }
  const data = points.map((p) => ({
    ts: new Date(p.date + 'T00:00:00Z').getTime(),
    users: p.users, sessions: p.sessions, pageviews: p.pageviews,
  }))
  return (
    <ResponsiveContainer width="100%" height={340}>
      <LineChart data={data} margin={{ top: 12, right: 24, bottom: 4, left: 8 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="#e8e8ed" />
        <XAxis
          dataKey="ts" type="number" scale="time" domain={['dataMin', 'dataMax']}
          tickFormatter={fmtDate} tick={{ fontSize: 12, fill: '#86868b' }}
          axisLine={{ stroke: '#d2d2d7' }} tickLine={false}
        />
        <YAxis
          tick={{ fontSize: 12, fill: '#86868b' }} width={60}
          axisLine={false} tickLine={false} allowDecimals={false}
        />
        <Tooltip content={<MetricsTooltip />} />
        <Legend iconType="plainline" wrapperStyle={{ fontSize: 13 }} />
        {SERIES.map((s) => (
          <Line
            key={s.key} type="monotone" dataKey={s.key} name={s.label}
            stroke={s.color} strokeWidth={2} dot={false} isAnimationActive={false}
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  )
}
