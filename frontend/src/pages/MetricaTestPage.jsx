import { useEffect } from 'react'

const COUNTER_ID = import.meta.env.VITE_METRICA_COUNTER_ID

// Hidden page (/metrica-test): the ONLY place the Yandex Metrica tag loads.
// It exists to generate real counter data for developing/testing the v4
// analytics module without tracking actual users — the privacy promise on
// the landing page stays true.
export default function MetricaTestPage() {
  useEffect(() => {
    if (!COUNTER_ID) return
    const noindex = document.createElement('meta')
    noindex.name = 'robots'
    noindex.content = 'noindex'
    document.head.appendChild(noindex)

    /* Standard Yandex Metrica snippet, scoped to this page only. */
    // eslint-disable-next-line
    ;(function (m, e, t, r, i, k, a) {
      m[i] = m[i] || function () { (m[i].a = m[i].a || []).push(arguments) }
      m[i].l = 1 * new Date()
      k = e.createElement(t); a = e.getElementsByTagName(t)[0]
      k.async = 1; k.src = r; a.parentNode.insertBefore(k, a)
    })(window, document, 'script', 'https://mc.yandex.ru/metrika/tag.js', 'ym')
    window.ym(Number(COUNTER_ID), 'init', {
      clickmap: false, trackLinks: false, accurateTrackBounce: true,
    })
    window.ym(Number(COUNTER_ID), 'hit', '/metrica-test')

    return () => { document.head.removeChild(noindex) }
  }, [])

  return (
    <main className="section">
      <div className="section-inner narrow">
        <h1 className="section-title">Metrica test page.</h1>
        <p className="section-sub">
          {COUNTER_ID
            ? `Counter ${COUNTER_ID} records a visit each time this page loads. ` +
              'Reload a few times, wait a couple of minutes, and the numbers appear ' +
              'in Metrica — and then in the Product Metrics module.'
            : 'No counter configured: set VITE_METRICA_COUNTER_ID in .env and restart the frontend.'}
        </p>
        <p className="section-sub muted">
          This is the only page that loads the Metrica tag — regular visitors are never tracked.
        </p>
      </div>
    </main>
  )
}
