import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'

const COUNTER_ID = Number(import.meta.env.VITE_METRICA_COUNTER_ID) || 0

// Site-wide Yandex Metrica tag: anonymous usage statistics for the site
// owner (viewed directly at metrika.yandex.com — there is no in-app
// analytics UI). Session replay (Webvisor) and click maps stay OFF; salary
// data never reaches Metrica. Renders nothing.
export default function MetricaTag() {
  const location = useLocation()

  useEffect(() => {
    if (!COUNTER_ID || window.ym) return
    /* Standard Metrica loader snippet. */
    ;(function (m, e, t, r, i, k, a) {
      m[i] = m[i] || function () { (m[i].a = m[i].a || []).push(arguments) }
      m[i].l = 1 * new Date()
      k = e.createElement(t); a = e.getElementsByTagName(t)[0]
      k.async = 1; k.src = r; a.parentNode.insertBefore(k, a)
    })(window, document, 'script', 'https://mc.yandex.ru/metrika/tag.js', 'ym')
    window.ym(COUNTER_ID, 'init', {
      clickmap: false,
      trackLinks: false,
      webvisor: false,
      accurateTrackBounce: true,
    })
  }, [])

  // SPA navigation: report each route change as a pageview.
  useEffect(() => {
    if (COUNTER_ID && window.ym) {
      window.ym(COUNTER_ID, 'hit', location.pathname)
    }
  }, [location.pathname])

  return null
}
