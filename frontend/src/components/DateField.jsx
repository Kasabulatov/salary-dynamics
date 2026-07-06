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

// Auto-insert dashes while typing digits (mobile numeric keyboards have no
// '-'): always YYYY-MM-DD — 20240115 -> 2024-01-15. Dates typed WITH
// separators (e.g. 15.03.2024 on a desktop keyboard) still parse as before.
function autoFormat(raw) {
  // Dots/slashes = the user is typing the European format themselves; leave
  // it for parseTyped. Dashes are OUR separators — always re-group as ISO
  // (otherwise our own inserted dash would block the next one).
  if (/[./]/.test(raw)) return raw
  const d = raw.replace(/[^\d]/g, '').slice(0, 8)
  if (!d) return ''
  if (d.length <= 4) return d
  if (d.length <= 6) return `${d.slice(0, 4)}-${d.slice(4)}`
  return `${d.slice(0, 4)}-${d.slice(4, 6)}-${d.slice(6)}`
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
        placeholder="YYYY-MM-DD"
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
