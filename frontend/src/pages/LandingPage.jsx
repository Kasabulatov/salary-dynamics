import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { publicCompute, computeRetry } from '../api'
import { track } from '../analytics'
import { useAuth } from '../AuthContext'
import { api } from '../api'
import { buildChartPoints, targetMapFrom } from '../chartData'
import SalaryChart from '../components/SalaryChart'
import CompareChart from '../components/CompareChart'
// Precomputed demo responses, bundled so the landing paints instantly even
// while the free-tier backend is cold-starting (30-60s). Regenerated monthly
// by .github/workflows/update-landing-demo.yml; the live queries below
// silently swap in fresh data when the backend answers.
import demoSalaryStatic from '../data/demo-salary.json'
import demoCompareStatic from '../data/demo-compare.json'

// Fictional example data for the live demo (KZT salary with two raises —
// long enough to show event bands and the inflation line).
const DEMO_FROM = '2022-03-01'

// Demo end date = first of the current month, NOT "today". A daily-changing
// end date makes every day's first visitor trigger an ~18s on-demand fetch
// for the new day's rates; a month-stable range stays served from cached
// data (~0.9s) for essentially every visitor.
function demoTo() {
  const d = new Date()
  return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, '0')}-01`
}
const DEMO_ENTRIES = [
  { amount: 480000, currency_code: 'KZT', effective_date: '2022-03-01', note: 'first job' },
  { amount: 650000, currency_code: 'KZT', effective_date: '2023-09-01', note: 'promotion' },
  { amount: 900000, currency_code: 'KZT', effective_date: '2025-04-01', note: 'senior role' },
]

const COMPARE_DEMO = {
  current: { amount: 800000, currency: 'KZT', city: 'Almaty' },
  offer: { amount: 3500, currency: 'EUR', city: 'Lisbon' },
  displayCurrency: 'USD',
}

export default function LandingPage() {
  const { user } = useAuth()
  const demoEnd = demoTo()
  // Background revalidation only — the page renders from the bundled JSON
  // right away, and these queries double as a warm-up ping for the backend
  // (by the time a visitor clicks Try/Compare it is usually awake).
  const demoQ = useQuery({
    queryKey: ['landing-demo', demoEnd],
    queryFn: () => publicCompute({
      entries: DEMO_ENTRIES, displayCurrency: 'USD', from: DEMO_FROM, to: demoEnd,
    }),
    staleTime: Infinity,
    ...computeRetry,
    refetchOnWindowFocus: (query) => query.state.status === 'error',
  })

  const compareDemoQ = useQuery({
    queryKey: ['landing-compare-demo'],
    queryFn: () => api('/api/public/compare', { method: 'POST', body: COMPARE_DEMO }),
    staleTime: Infinity,
    ...computeRetry,
    refetchOnWindowFocus: (query) => query.state.status === 'error',
  })

  // Live data wins; bundled snapshot otherwise. Same range and same rates, so
  // the swap is invisible — and a cold/failed backend never blanks the demo.
  const demo = demoQ.data ?? demoSalaryStatic
  const compareDemo = compareDemoQ.data ?? demoCompareStatic
  const points = buildChartPoints(demo.series.points, demo.entries, targetMapFrom(demo.inflation?.points))
  // End the axis where the rendered data ends (the snapshot may trail the
  // live range by a month right after a month rollover).
  const chartEnd = demo.series.points.at(-1)?.date ?? demoEnd

  return (
    <main>
      <header className="hero">
        <h1>Your salary. In real terms.</h1>
        <p className="hero-sub">
          Track what your pay is really worth, day by day — and put any job
          offer next to it, honestly.
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
      </header>

      {/* Two products, equal billing (Apple dual-tile pattern). */}
      <section className="section product-tiles-section">
        <div className="section-inner">
          <div className="product-tiles">
            <Link to={user ? '/' : '/try'} className="product-tile">
              <h3>Salary Dynamics</h3>
              <p>Your salary's true value, every single day — with inflation as the honest benchmark.</p>
              <span className="product-tile-cta">Open →</span>
            </Link>
            <Link to="/compare" className="product-tile">
              <h3>Offer Comparison</h3>
              <p>Current salary vs the offer — by exchange rate, and by what the money really buys.</p>
              <span className="product-tile-cta">Compare →</span>
            </Link>
          </div>
        </div>
      </section>

      <section className="section section-alt">
        <div className="section-inner">
          <h2 className="section-title">This is a live demo.</h2>
          <p className="section-sub">
            An example salary in Kazakhstani tenge, computed with real historical
            exchange rates and real published inflation — the same engine you'll use.
          </p>
          <div className="chart-panel">
            <SalaryChart
              points={points}
              from={new Date(DEMO_FROM)}
              to={new Date(chartEnd)}
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
          <h2 className="section-title">Weighing a job offer?</h2>
          <p className="section-sub">
            The <strong>Offer Comparison</strong> puts your current salary and an
            offer side by side — in any of 33 currencies, across cities — and
            answers honestly: better by exchange rate, and better in <em>real
            purchasing power</em> (cost-of-living adjusted). No account needed,
            and you can download the result as an image to share.
          </p>
          <div className="chart-panel landing-compare-demo">
            <p className="compare-verdict">{compareDemo.verdict}</p>
            <p className="landing-demo-caption">
              Live example: 800 000 KZT in Almaty vs a 3 500 EUR offer in Lisbon — real market rates.
            </p>
            <CompareChart result={compareDemo} />
          </div>
          <div className="hero-ctas">
            <Link to="/compare" className="btn btn-primary btn-large">Compare an offer</Link>
          </div>
        </div>
      </section>

      <section className="section">
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
