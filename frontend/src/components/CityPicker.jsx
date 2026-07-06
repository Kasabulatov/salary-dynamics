import { useState } from 'react'

// Searchable city picker: values come ONLY from the known-cities list
// (typing filters, clicking selects). Selecting "No city" clears it.
export default function CityPicker({ value, onChange, cities }) {
  const [open, setOpen] = useState(false)
  const [filter, setFilter] = useState('')

  const shown = open ? filter : value
  const matches = (cities ?? []).filter((c) =>
    c.city.toLowerCase().includes(filter.toLowerCase()))

  const pick = (city) => {
    onChange(city)
    setOpen(false)
  }

  return (
    <div className="citypicker">
      <input
        type="text"
        value={shown}
        placeholder="Choose a city (optional)"
        onFocus={() => { setOpen(true); setFilter('') }}
        onChange={(e) => setFilter(e.target.value)}
        onBlur={() => setTimeout(() => setOpen(false), 150)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && matches.length) {
            e.preventDefault()
            pick(matches[0].city)
          }
          if (e.key === 'Escape') setOpen(false)
        }}
      />
      {open && (
        <ul className="citypicker-list">
          <li className="citypicker-none" onMouseDown={() => pick('')}>
            No city (exchange rate only)
          </li>
          {matches.map((c) => (
            <li key={`${c.city}-${c.country}`} onMouseDown={() => pick(c.city)}>
              {c.city} <span className="muted">{c.country}</span>
            </li>
          ))}
          {!matches.length && <li className="citypicker-none">No matches</li>}
        </ul>
      )}
    </div>
  )
}
