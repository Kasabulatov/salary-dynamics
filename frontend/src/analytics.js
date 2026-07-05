// Product analytics events (Yandex Metrica goals).
//
// HARD RULE: events describe BEHAVIOR, never money. Currency codes, flags,
// counts and presets are fine; salary amounts and note text must NEVER be
// sent — the landing page promises "your salary data is never part of it".
//
// Metrica attaches the current page URL to every goal automatically, and
// session reports give the full page path — so funnels like
// landing -> /try -> guest_entry_added -> register_completed work out of
// the box once the goals are defined in the counter settings.

const COUNTER = Number(import.meta.env.VITE_METRICA_COUNTER_ID) || 0

export function track(goal, params) {
  if (COUNTER && typeof window !== 'undefined' && window.ym) {
    window.ym(COUNTER, 'reachGoal', goal, params)
  }
}
