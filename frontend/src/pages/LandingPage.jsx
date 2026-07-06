import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { publicCompute } from '../api'
import { track } from '../analytics'
import { useAuth } from '../AuthContext'
import { buildChartPoints, targetMapFrom } from '../chartData'
import SalaryChart from '../components/SalaryChart'

// Fictional example data for the live demo (KZT salary with two raises —
// long enough to show event bands and the inflation line).
const DEMO_FROM = '2022-03-01'
const DEMO_ENTRIES = [
  { amount: 480000, currency_code: 'KZT', effective_date: '2022-03-01', note: 'first job' },
  { amount: 650000, currency_code: 'KZT', effective_date: '2023-09-01', note: 'promotion' },
  { amount: 900000, currency_code: 'KZT', effective_date: '2025-04-01', note: 'senior role' },
]

export default function LandingPage() {
  const { user } = useAuth()
  const today = new Date().toISOString().slice(0, 10)
  const demoQ = useQuery({
    queryKey: ['landing-demo'],
    queryFn: () => publicCompute({
      entries: DEMO_ENTRIES, displayCurrency: 'USD', from: DEMO_FROM, to: today,
    }),
    staleTime: Infinity,
  })

  const demo = demoQ.data
  const points = demo
    ? buildChartPoints(demo.series.points, demo.entries, targetMapFrom(demo.inflation?.points))
    : []

  return (
    <main>
      <header className="hero">
        <h1>Your salary. In real terms.</h1>
        <p className="hero-sub">
          See what your pay is really worth — converted at the exchange rate of{' '}
          <em>every single day</em>, measured against inflation.
        </p>
        <div className="hero-ctas">
          {user ? (
            <Link to="/" className="btn btn-primary btn-large">Open your Salary Tracker</Link>
          ) : (
            <>
              <Link to="/try" className="btn btn-primary btn-large"
                onClick={() => track('cta_try_clicked', { from: 'hero' })}>
                Try it — no account needed
              </Link>
              <Link to="/register" className="btn btn-ghost btn-large"
                onClick={() => track('cta_register_clicked', { from: 'hero' })}>
                Create account
              </Link>
            </>
          )}
        </div>
        <p className="hero-secondary-cta">
          Comparing a job offer in another currency?{' '}
          <Link to="/compare">Try the Offer Comparison →</Link>
        </p>
      </header>

      <section className="section section-alt">
        <div className="section-inner">
          <h2 className="section-title">This is a live demo.</h2>
          <p className="section-sub">
            An example salary in Kazakhstani tenge, computed with real historical
            exchange rates and real published inflation — the same engine you'll use.
          </p>
          <div className="chart-panel">
            {demoQ.isLoading && <div className="chart-empty">Loading the demo with real market data…</div>}
            {demoQ.isError && <div className="chart-empty">Demo unavailable right now — the product still works.</div>}
            {demo && (
              <>
                <SalaryChart
                  points={points}
                  from={new Date(DEMO_FROM)}
                  to={new Date()}
                  displayCurrency="USD"
                  events={demo.events}
                  inflationMeta={demo.inflation}
                />
                <div className="chart-legend">
                  <span><span className="legend-dot" /> salary change</span>
                  <span><span className="legend-band red" /> sharp drop</span>
                  <span><span className="legend-band green" /> sharp rise</span>
                  <span><span className="legend-target" /> inflation target</span>
                </div>
              </>
            )}
          </div>
        </div>
      </section>

      <section className="section">
        <div className="section-inner">
          <h2 className="section-title">Three honest answers.</h2>
          <p className="section-sub">The questions every raise conversation should start with.</p>
          <div className="feature-grid">
            <div className="feature-card">
              <h3>What is it worth today?</h3>
              <p>
                Your salary history, converted between 33 currencies using official
                daily rates — from the European Central Bank and the National Bank
                of Kazakhstan. The rate of <em>that day</em>, never today's.
              </p>
            </div>
            <div className="feature-card">
              <h3>Did the market move it?</h3>
              <p>
                Sharp exchange-rate moves (&gt;2% in a day, &gt;4% in a week) are
                highlighted directly on your chart — red when your value dropped,
                green when it grew — with the exact dates and amounts.
              </p>
            </div>
            <div className="feature-card">
              <h3>Is it keeping up with inflation?</h3>
              <p>
                A target line shows what your starting salary would need to become
                to keep its purchasing power, using officially published annual
                inflation (World Bank data) for your salary's country.
              </p>
            </div>
          </div>
        </div>
      </section>

      <section className="section section-alt">
        <div className="section-inner narrow">
          <h2 className="section-title">Your data stays yours.</h2>
          <p className="section-sub">
            Guest entries never leave your browser tab. Registered accounts store
            only your email and salary entries — never shared, never sold. We collect
            anonymous usage statistics (Yandex Metrica, no session recording) to
            understand how the product is used; your salary data is never part of it.
          </p>
          <div className="hero-ctas">
            <Link to="/try" className="btn btn-primary btn-large"
              onClick={() => track('cta_try_clicked', { from: 'bottom' })}>
              Try it now
            </Link>
          </div>
        </div>
      </section>
    </main>
  )
}
