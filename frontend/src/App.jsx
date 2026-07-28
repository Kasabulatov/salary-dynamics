import { Routes, Route, Navigate, Link, useLocation } from 'react-router-dom'
import { useAuth } from './AuthContext'
import { track } from './analytics'
import LandingPage from './pages/LandingPage'
import LoginPage from './pages/LoginPage'
import RegisterPage from './pages/RegisterPage'
import SalaryTrackerPage from './pages/SalaryTrackerPage'
import GuestTrackerPage from './pages/GuestTrackerPage'
import OfferComparePage from './pages/OfferComparePage'
import MetricaTag from './components/MetricaTag'
import { useWakingUp, WAKEUP_MESSAGE } from './components/wakeup'

// Auth-gate placeholder. Only browsers with a previous session ever hold
// this longer than 2.5s (see AuthContext) — be honest about why.
function PageLoading() {
  const waking = useWakingUp(true)
  return <div className="page-loading">{waking ? WAKEUP_MESSAGE : 'Loading…'}</div>
}

function Home() {
  const { user } = useAuth()
  if (user === undefined) return <PageLoading />
  return user ? <SalaryTrackerPage /> : <LandingPage />
}

function GuestOnly({ children }) {
  const { user } = useAuth()
  if (user === undefined) return <PageLoading />
  if (user) return <Navigate to="/" replace />
  return children
}

function Nav() {
  const { user, logout } = useAuth()
  const location = useLocation()

  if (user) {
    return (
      <nav className="nav">
        <div className="nav-inner">
          <Link to="/welcome" className="nav-brand nav-brand-link">About</Link>
          <Link to="/">Salary Dynamics</Link>
          <Link to="/compare">Offer Compare</Link>
          <a href="https://forms.gle/A9kN2tPradKXNRkt5" target="_blank"
            rel="noopener noreferrer" onClick={() => track('feedback_click', {})}>
            Feedback
          </a>
          <span className="nav-spacer" />
          <span className="nav-user">{user.email}</span>
          <button className="nav-link-btn" onClick={logout}>Sign out</button>
        </div>
      </nav>
    )
  }

  // Public nav for landing / guest / auth pages.
  return (
    <nav className="nav">
      <div className="nav-inner">
        <Link to="/" className="nav-brand nav-brand-link">About</Link>
        <Link to="/try">Salary Dynamics</Link>
        <Link to="/compare">Offer Compare</Link>
        <a href="https://forms.gle/A9kN2tPradKXNRkt5" target="_blank"
            rel="noopener noreferrer" onClick={() => track('feedback_click', {})}>
            Feedback
          </a>
        {location.pathname === '/try' && <span className="nav-guest-chip">Guest mode</span>}
        <span className="nav-spacer" />
        <Link to="/login">Sign in</Link>
        <Link to="/register" className="nav-cta">Create account</Link>
      </div>
    </nav>
  )
}

function Footer() {
  return (
    <footer className="footer">
      <p>
        Exchange rates: European Central Bank (via Frankfurter) · National Bank of Kazakhstan.
        Inflation: World Bank.
      </p>
      <p>Every value is converted at the rate effective on that day — never today's rate.</p>
    </footer>
  )
}

export default function App() {
  return (
    <>
      <MetricaTag />
      <Nav />
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/welcome" element={<LandingPage />} />
        <Route path="/try" element={<GuestOnly><GuestTrackerPage /></GuestOnly>} />
        <Route path="/compare" element={<OfferComparePage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <Footer />
    </>
  )
}
