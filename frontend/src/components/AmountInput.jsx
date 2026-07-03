// Amount input with live thousands separators (650000 -> "650 000").
// Keeps a raw numeric string in state; formatting is display-only.

function format(raw) {
  if (!raw) return ''
  const [int, dec] = String(raw).split('.')
  const grouped = int.replace(/\B(?=(\d{3})+(?!\d))/g, ' ') // narrow space
  return dec !== undefined ? `${grouped}.${dec}` : grouped
}

export default function AmountInput({ value, onChange, ...rest }) {
  const handle = (e) => {
    // Strip separators, accept comma as decimal point.
    const raw = e.target.value
      .replace(/[\s  ]/g, '')
      .replace(',', '.')
    if (raw === '' || /^\d+\.?\d{0,2}$/.test(raw)) {
      onChange(raw)
    }
  }
  return (
    <input
      type="text"
      inputMode="decimal"
      autoComplete="off"
      value={format(value)}
      onChange={handle}
      placeholder="650 000"
      {...rest}
    />
  )
}
