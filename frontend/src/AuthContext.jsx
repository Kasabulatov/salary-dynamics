import { createContext, useContext, useEffect, useState } from 'react'
import { api } from './api'

const AuthContext = createContext(null)

// Remembers (client-side only) that this browser has had a session, so the
// cold-start fallback below never flashes the landing at a logged-in user.
const HAD_SESSION = 'sd_had_session'

export function AuthProvider({ children }) {
  // undefined = still checking the session; null = logged out.
  const [user, setUser] = useState(undefined)

  useEffect(() => {
    // On a free-tier cold start /api/me can hang for 30-60s, and the whole
    // app sits behind the user===undefined gate. A browser that never had a
    // session is a first-time or guest visitor: assume logged-out after
    // 2.5s so the landing renders instantly; the real answer still applies
    // when it arrives. Browsers with a previous session keep the gate —
    // their tracker needs the backend anyway.
    let timer
    if (localStorage.getItem(HAD_SESSION) !== '1') {
      timer = setTimeout(() => setUser((u) => (u === undefined ? null : u)), 2500)
    }
    api('/api/me')
      .then((u) => {
        localStorage.setItem(HAD_SESSION, '1')
        setUser(u)
      })
      .catch(() => {
        localStorage.removeItem(HAD_SESSION)
        setUser(null)
      })
    return () => clearTimeout(timer)
  }, [])

  const login = async (email, password) => {
    const u = await api('/api/login', { method: 'POST', body: { email, password } })
    localStorage.setItem(HAD_SESSION, '1')
    setUser(u)
    return u
  }

  const register = async (email, password) => {
    await api('/api/register', { method: 'POST', body: { email, password } })
    return login(email, password)
  }

  const logout = async () => {
    await api('/api/logout', { method: 'POST' })
    localStorage.removeItem(HAD_SESSION)
    setUser(null)
  }

  return (
    <AuthContext.Provider value={{ user, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  return useContext(AuthContext)
}
