import { createContext, useContext, useEffect, useState } from 'react'
import { api } from './api'

const AuthContext = createContext(null)

export function AuthProvider({ children }) {
  // undefined = still checking the session; null = logged out.
  const [user, setUser] = useState(undefined)

  useEffect(() => {
    api('/api/me')
      .then(setUser)
      .catch(() => setUser(null))
  }, [])

  const login = async (email, password) => {
    const u = await api('/api/login', { method: 'POST', body: { email, password } })
    setUser(u)
    return u
  }

  const register = async (email, password) => {
    await api('/api/register', { method: 'POST', body: { email, password } })
    return login(email, password)
  }

  const logout = async () => {
    await api('/api/logout', { method: 'POST' })
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
