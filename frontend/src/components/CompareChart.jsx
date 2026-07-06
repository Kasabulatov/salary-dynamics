import {
  ResponsiveContainer, BarChart, Bar, XAxis, YAxis,
  CartesianGrid, Tooltip, Legend,
} from 'recharts'

const fmt = (v) => Number(v).toLocaleString(undefined, { maximumFractionDigits: 0 })

function CompareTooltip({ active, payload, label, displayCurrency }) {
  if (!active || !payload?.length) return null
  return (
    <div className="chart-tooltip">
      <div className="chart-tooltip-date">{label}</div>
      {payload.map((p) => (
        <div key={p.dataKey} style={{ color: p.fill }}>
          {p.dataKey}: {fmt(p.value)} {displayCurrency}
        </div>
      ))}
    </div>
  )
}

// Two-bar-group comparison: FX value and (when available) purchasing-power
// value, current vs offer. Same visual language as the salary chart.
export default function CompareChart({ result }) {
  const data = [
    {
      name: 'By exchange rate',
      Current: result.current.amountConverted,
      Offer: result.offer.amountConverted,
    },
  ]
  if (result.real.available) {
    data.push({
      name: 'Real purchasing power',
      Current: result.current.realValue,
      Offer: result.offer.realValue,
    })
  }
  return (
    <ResponsiveContainer width="100%" height={300}>
      <BarChart data={data} margin={{ top: 12, right: 24, bottom: 4, left: 8 }} barGap={8}>
        <CartesianGrid strokeDasharray="3 3" stroke="#e8e8ed" vertical={false} />
        <XAxis dataKey="name" tick={{ fontSize: 13, fill: '#6e6e73' }}
          axisLine={{ stroke: '#d2d2d7' }} tickLine={false} />
        <YAxis tickFormatter={(v) => v.toLocaleString(undefined, { notation: 'compact' })}
          tick={{ fontSize: 12, fill: '#86868b' }} width={64}
          axisLine={false} tickLine={false} />
        <Tooltip content={<CompareTooltip displayCurrency={result.displayCurrency} />} />
        <Legend iconType="circle" wrapperStyle={{ fontSize: 13 }} />
        <Bar dataKey="Current" fill="#86868b" radius={[6, 6, 0, 0]} isAnimationActive={false} />
        <Bar dataKey="Offer" fill="#0071e3" radius={[6, 6, 0, 0]} isAnimationActive={false} />
      </BarChart>
    </ResponsiveContainer>
  )
}
