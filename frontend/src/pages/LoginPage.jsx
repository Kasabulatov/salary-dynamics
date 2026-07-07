import { useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { useAuth } from '../AuthContext'
import GoogleButton from '../components/GoogleButton'

const AUTH_ERRORS = {
  state: 'That sign-in attempt expired. Please try again.',
  declined: 'Sign-in was cancelled.',
  unverified: 'Your Google email isn’t verified, so we can’t sign you in with it.',
  exchange: 'Google sign-in failed. Please try again.',
  no_code: 'Google didn’t return a sign-in code. Please try again.',
  not_configured: 'Google sign-in isn’t available right now.',
  internal: 'Something went wrong. Please try again.',
}

export default function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const authError = params.get('auth_error')

  const onSubmit = async (e) => {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      await login(email, password)
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
        <h1>Sign in to Dynamics.</h1>
        {authError && <div className="form-error">{AUTH_ERRORS[authError] || 'Sign-in failed. Please try again.'}</div>}
        {error && <div className="form-error">{error}</div>}
        <GoogleButton label="Sign in with Google" />
        <label>
          Email
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoFocus />
        </label>
        <label>
          Password
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </label>
        <button className="btn btn-primary" disabled={busy}>
          {busy ? 'Signing in…' : 'Sign in'}
        </button>
        <p className="auth-alt">
          Don’t have an account? <Link to="/register">Create yours now.</Link>
        </p>
      </form>
    </div>
  )
}
