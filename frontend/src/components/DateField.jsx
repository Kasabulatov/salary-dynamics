import { useEffect, useState } from 'react'

// DateField: type the date directly (2024-06-01, 01.06.2024, 01/06/2024)
// or pick from the native calendar. `value` is always ISO YYYY-MM-DD.

function buildISO(y, mo, d) {
  const yy = Number(y), mm = Number(mo), dd = Number(d)
  const dt = new Date(yy, mm - 1, dd)
  if (dt.getFullYear() !== yy || dt.getMonth() !== mm - 1 || dt.getDate() !== dd) return null
  return `${yy}-${String(mm).padStart(2, '0')}-${String(dd).padStart(2, '0')}`
}

function parseTyped(s) {
  s = s.trim()
  let m = s.match(/^(\d{4})-(\d{1,2})-(\d{1,2})$/)
  if (m) return buildISO(m[1], m[2], m[3])
  m = s.match(/^(\d{1,2})[./](\d{1,2})[./](\d{4})$/) // 01.06.2024 / 01/06/2024
  if (m) return buildISO(m[3], m[2], m[1])
  return null
}

const todayISO = () => new Date().toISOString().slice(0, 10)

// Auto-insert separators while typing digits (mobile numeric keyboards have
// no '-' or '.'): 20240115 -> 2024-01-15, 15032024 -> 15.03.2024. Year-first
// is chosen when the first 4 digits are a plausible year AND digits 5-6 are a
// plausible month; otherwise day-first (resolves e.g. 19.03.2024 vs 1903-…).
function autoFormat(raw) {
  const digits = raw.replace(/[^\d]/g, '').slice(0, 8)
  if (!digits) return raw.trim() === '' ? '' : raw
  const yearFirst =
    /^(19|20)/.test(digits) &&
    (digits.length <= 4 || Number(digits.slice(4, 6).padEnd(2, '1')) <= 12) &&
    (digits.length < 6 || Number(digits.slice(4, 6)) >= 1)
  if (yearFirst) {
    if (digits.length <= 4) return digits
    if (digits.length <= 6) return `${digits.slice(0, 4)}-${digits.slice(4)}`
    return `${digits.slice(0, 4)}-${digits.slice(4, 6)}-${digits.slice(6)}`
  }
  if (digits.length <= 2) return digits
  if (digits.length <= 4) return `${digits.slice(0, 2)}.${digits.slice(2)}`
  return `${digits.slice(0, 2)}.${digits.slice(2, 4)}.${digits.slice(4)}`
}

export default function DateField({ value, onChange, showToday = false }) {
  const [text, setText] = useState(value || '')
  const [invalid, setInvalid] = useState(false)
  const [focused, setFocused] = useState(false)

  // Sync external value in ONLY while not typing — otherwise a valid prefix
  // (e.g. "2024-01-1") triggers onChange and clobbers the keystrokes.
  useEffect(() => {
    if (!focused) {
      setText(value || '')
      setInvalid(false)
    }
  }, [value, focused])

  const handleText = (e) => {
    const t = autoFormat(e.target.value)
    setText(t)
    if (t === '') {
      setInvalid(false)
      onChange('')
      return
    }
    const iso = parseTyped(t)
    if (iso) {
      setInvalid(false)
      onChange(iso)
    } else {
      setInvalid(true)
    }
  }

  return (
    <div className={`datefield ${invalid ? 'datefield-invalid' : ''}`}>
      <input
        type="text"
        inputMode="numeric"
        autoComplete="off"
        placeholder="2024-06-01 or 01.06.2024"
        value={text}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        onChange={handleText}
      />
      <input
        type="date"
        className="datefield-picker"
        aria-label="Open calendar"
        tabIndex={-1}
        value={parseTyped(text) || ''}
        onChange={(e) => onChange(e.target.value)}
      />
      {showToday && (
        <button
          type="button"
          className="btn btn-mini"
          onClick={() => onChange(todayISO())}
        >
          Today
        </button>
      )}
    </div>
  )
}
