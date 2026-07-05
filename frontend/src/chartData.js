// Merge a daily series with entry metadata (dots + tooltip) and the optional
// inflation target. Shared by the tracker, guest mode, and the landing demo.
export function buildChartPoints(seriesPoints, entries, targetByDate) {
  const sorted = [...entries].sort((a, b) => a.effective_date.localeCompare(b.effective_date))
  const entryByDate = new Map(sorted.map((e) => [e.effective_date.slice(0, 10), e]))
  let idx = 0
  let active = null
  return seriesPoints.map((p) => {
    while (idx < sorted.length && sorted[idx].effective_date.slice(0, 10) <= p.date) {
      active = sorted[idx]
      idx++
    }
    const isEntry = entryByDate.has(p.date)
    return {
      ts: new Date(p.date + 'T00:00:00Z').getTime(),
      value: p.value,
      target: targetByDate?.get(p.date) ?? null,
      entry: isEntry ? entryByDate.get(p.date) : active,
      isEntry,
    }
  })
}

export function targetMapFrom(inflationPoints) {
  if (!inflationPoints?.length) return null
  return new Map(inflationPoints.map((p) => [p.date, p.value]))
}
