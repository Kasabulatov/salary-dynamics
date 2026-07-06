const BASE = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

function getCookie(name) {
  const match = document.cookie.match(new RegExp('(?:^|; )' + name + '=([^;]*)'))
  return match ? decodeURIComponent(match[1]) : ''
}

// api wraps fetch: sends the auth cookie, attaches the CSRF header on writes,
// and normalizes the backend's {"error": "..."} shape into thrown Errors.
export async function api(path, { method = 'GET', body } = {}) {
  const headers = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (method !== 'GET') headers['X-CSRF-Token'] = getCookie('csrf')

  const res = await fetch(BASE + path, {
    method,
    headers,
    credentials: 'include',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const err = new Error(data.error || `Request failed (${res.status})`)
    err.status = res.status
    throw err
  }
  return data
}

// Retry policy for the public compute/compare queries. A momentary 429
// (shared-NAT burst, or heavy interaction) should self-heal — httprate uses
// a rolling window, so waiting a few seconds frees capacity. Client input
// errors (400/404) never retry; 429 and 5xx retry up to 3 times.
export const computeRetry = {
  retry: (failureCount, error) => {
    const s = error?.status
    if (s && s >= 400 && s < 500 && s !== 429) return false
    return failureCount < 3
  },
  retryDelay: (attempt, error) => (error?.status === 429 ? 4000 : 800 * (attempt + 1)),
}

// rateLimited reports whether an error is a 429 (for friendlier UI copy).
export const rateLimited = (error) => error?.status === 429

// publicCompute: stateless computation for guest mode and the landing demo.
// Entries travel in the request; the server stores nothing.
export function publicCompute({ entries, displayCurrency, from, to }) {
  return api('/api/public/compute', {
    method: 'POST',
    body: {
      entries,
      display_currency: displayCurrency,
      from,
      to,
      include_inflation: true,
      include_events: true,
    },
  })
}
