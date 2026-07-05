import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../AuthContext'
import { api } from '../api'
import { getGuestEntries, clearGuestEntries } from '../guestStore'

export default function RegisterPage() {
  const { register } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const guestCount = getGuestEntries().length

  const onSubmit = async (e) => {
    e.preventDefault()
    setError('')
    if (password.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }
    if (password !== confirm) {
      setError('Passwords do not match')
      return
    }
    setBusy(true)
    try {
      await register(email, password)
      // Carry guest entries into the fresh account (register() logged us in,
      // so the CSRF cookie is present for these writes).
      const guestEntries = getGuestEntries()
      for (const { amount, currency_code, effective_date, note } of guestEntries) {
        await api('/api/salary', {
          method: 'POST',
          body: { amount, currency_code, effective_date, note },
        })
      }
      if (guestEntries.length) clearGuestEntries()
      navigate('/')
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="auth-page">
      <form className="auth-card" onSubmit={onSubmit}>
        <h1>Create your account.</h1>
        {guestCount > 0 && (
          <div className="guest-import-note">
            Your {guestCount} guest {guestCount === 1 ? 'entry' : 'entries'} will be
            saved to your new account automatically.
          </div>
        )}
        {error && <div className="form-error">{error}</div>}
        <label>
          Email
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoFocus />
        </label>
        <label>
          Password (min. 8 characters)
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </label>
        <label>
          Confirm password
          <input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} required />
        </label>
        <button className="btn btn-primary" disabled={busy}>
          {busy ? 'Creating…' : 'Register'}
        </button>
        <p className="auth-alt">
          Already have an account? <Link to="/login">Sign in.</Link>
        </p>
      </form>
    </div>
  )
}
