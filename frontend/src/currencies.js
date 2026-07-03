// Currencies covered by our rate sources (Frankfurter/ECB set + NBK extras).
export const CURRENCIES = [
  'USD', 'EUR', 'KZT', 'RUB', 'GBP', 'CHF', 'JPY', 'CNY', 'TRY',
  'AUD', 'BGN', 'BRL', 'CAD', 'CZK', 'DKK', 'HKD', 'HUF', 'IDR',
  'ILS', 'INR', 'ISK', 'KRW', 'MXN', 'MYR', 'NOK', 'NZD', 'PHP',
  'PLN', 'RON', 'SEK', 'SGD', 'THB', 'ZAR',
]

export const PRESETS = [
  { key: '6m', label: 'Last 6 Months' },
  { key: 'ytd', label: 'Year to Date' },
  { key: '12m', label: 'Last 12 Months' },
  { key: 'year', label: 'Current Year' },
  { key: '5y', label: 'Last 5 Years' },
  { key: 'custom', label: 'Custom' },
]

export function rangeForPreset(preset, customFrom, customTo) {
  const now = new Date()
  const monthsAgo = (n) => {
    const d = new Date(now)
    d.setMonth(d.getMonth() - n)
    return d
  }
  switch (preset) {
    case '6m': return [monthsAgo(6), now]
    case 'ytd': return [new Date(now.getFullYear(), 0, 1), now]
    case '12m': return [monthsAgo(12), now]
    case 'year': return [new Date(now.getFullYear(), 0, 1), new Date(now.getFullYear(), 11, 31)]
    case '5y': return [monthsAgo(60), now]
    case 'custom': {
      const from = customFrom ? new Date(customFrom) : monthsAgo(60)
      const to = customTo ? new Date(customTo) : now
      return [from, to]
    }
    default: return [monthsAgo(60), now]
  }
}
