import { Routes, Route, Navigate, Link } from 'react-router-dom'
import { useAuth } from './AuthContext'
import LoginPage from './pages/LoginPage'
import RegisterPage from './pages/RegisterPage'
import SalaryTrackerPage from './pages/SalaryTrackerPage'

function ProtectedRoute({ children }) {
  const { user } = useAuth()
  if (user === undefined) return <div className="page-loading">Loading…</div>
  if (user === null) return <Navigate to="/login" replace />
  return children
}

function Nav() {
  const { user, logout } = useAuth()
  if (!user) return null
  return (
    <nav className="nav">
      <div className="nav-inner">
        <span className="nav-brand">Dynamics</span>
        <Link to="/">Salary Tracker</Link>
        <span className="nav-disabled" title="Coming in v4">Product Metrics</span>
        <span className="nav-spacer" />
        <span className="nav-user">{user.email}</span>
        <button className="nav-link-btn" onClick={logout}>Sign out</button>
      </div>
    </nav>
  )
}

function Footer() {
  const { user } = useAuth()
  if (!user) return null
  return (
    <footer className="footer">
      <p>
        Exchange rates: European Central Bank (via Frankfurter) · National Bank of Kazakhstan.
      </p>
      <p>Every value is converted at the rate effective on that day — never today's rate.</p>
    </footer>
  )
}

export default function App() {
  return (
    <>
      <Nav />
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <SalaryTrackerPage />
            </ProtectedRoute>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <Footer />
    </>
  )
}
