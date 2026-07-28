import { useEffect, useState } from 'react'

// A warm backend answers a public compute in ~1s; anything much longer means
// the free-tier instance is cold-starting (30-60s). After `ms` of continuous
// fetching, switch the UI to honest wake-up copy instead of a generic spinner.
export function useWakingUp(isFetching, ms = 2500) {
  const [waking, setWaking] = useState(false)
  useEffect(() => {
    if (!isFetching) {
      setWaking(false)
      return undefined
    }
    const t = setTimeout(() => setWaking(true), ms)
    return () => clearTimeout(t)
  }, [isFetching, ms])
  return waking
}

export const WAKEUP_MESSAGE =
  'Our free server is waking up — this can take up to 30 seconds. ' +
  'The result will appear automatically.'
