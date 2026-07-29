import { useEffect, useState } from 'react'

/**
 * A clock that re-renders on an interval, so relative times ("last block
 * mined 4 min ago", ribbon tile ages) keep counting up between server
 * pushes instead of freezing at whatever they read when the push landed.
 */
export function useNow(intervalMs: number): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), intervalMs)
    return () => clearInterval(timer)
  }, [intervalMs])
  return now
}
