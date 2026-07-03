import { useState } from 'react'
import { CURRENCIES } from '../currencies'
import AmountInput from './AmountInput'
import DateField from './DateField'

// Form for creating or editing one salary entry.
export default function SalaryEntryForm({ initial, onSubmit, onCancel, busy }) {
  const [amount, setAmount] = useState(initial ? String(initial.amount) : '')
  const [currency, setCurrency] = useState(initial?.currency_code ?? 'KZT')
  const [date, setDate] = useState(initial?.effective_date?.slice(0, 10) ?? '')
  const [note, setNote] = useState(initial?.note ?? '')
  const [error, setError] = useState('')

  const submit = async (e) => {
    e.preventDefault()
    setError('')
    if (!amount || Number(amount) <= 0) {
      setError('Amount must be greater than 0')
      return
    }
    if (!date) {
      setError('Please enter a valid date')
      return
    }
    try {
      await onSubmit({
        amount: Number(amount),
        currency_code: currency,
        effective_date: date,
        note: note.trim() || null,
      })
      if (!initial) {
        setAmount('')
        setNote('')
      }
    } catch (err) {
      setError(err.message)
    }
  }

  return (
    <form className="entry-form" onSubmit={submit}>
      {error && <div className="form-error">{error}</div>}
      <div className="entry-form-row">
        <label>
          Amount
          <AmountInput value={amount} onChange={setAmount} required />
        </label>
        <label>
          Salary Currency
          <select value={currency} onChange={(e) => setCurrency(e.target.value)}>
            {CURRENCIES.map((c) => <option key={c} value={c}>{c}</option>)}
          </select>
        </label>
        <label>
          Effective from
          <DateField value={date} onChange={setDate} showToday />
        </label>
        <label className="entry-form-note">
          Note (optional)
          <input
            type="text" value={note} placeholder="e.g. promotion, new job"
            onChange={(e) => setNote(e.target.value)}
          />
        </label>
        <div className="entry-form-actions">
          <button className="btn btn-primary" disabled={busy}>
            {initial ? 'Save' : 'Add'}
          </button>
          {onCancel && (
            <button type="button" className="btn btn-ghost" onClick={onCancel}>
              Cancel
            </button>
          )}
        </div>
      </div>
    </form>
  )
}
