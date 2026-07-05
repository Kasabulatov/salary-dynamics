// Guest-mode storage: entries live ONLY in sessionStorage — they survive
// page-to-page navigation but are wiped when the tab closes. Nothing is
// ever written to the server for guests.

const KEY = 'guest_salary_entries'

export function getGuestEntries() {
  try {
    return JSON.parse(sessionStorage.getItem(KEY)) ?? []
  } catch {
    return []
  }
}

function save(entries) {
  sessionStorage.setItem(KEY, JSON.stringify(entries))
  return entries
}

export function addGuestEntry({ amount, currency_code, effective_date, note }) {
  const entries = getGuestEntries()
  const id = entries.length ? Math.max(...entries.map((e) => e.id)) + 1 : 1
  return save([...entries, { id, amount, currency_code, effective_date, note: note ?? null }])
}

export function updateGuestEntry(id, patch) {
  return save(getGuestEntries().map((e) => (e.id === id ? { ...e, ...patch } : e)))
}

export function deleteGuestEntry(id) {
  return save(getGuestEntries().filter((e) => e.id !== id))
}

export function clearGuestEntries() {
  sessionStorage.removeItem(KEY)
}
