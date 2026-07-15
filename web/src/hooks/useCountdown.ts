import { useEffect, useRef, useState } from 'react'

/** Matches the backend's ETA floor (stats.go): the pill never reads
 * zero or negative, just "any moment now". */
const FLOOR_SECONDS = 5

/**
 * Ticks a server-provided ETA down locally between pushes (round-7
 * live countdown). The anchor re-stamps whenever `resetKey` changes —
 * pass the stats object, so every push (including the immediate
 * on-block push) resets the clock and the countdown "restarts" on each
 * mined block for free. Disabled → returns the ETA unchanged.
 */
export function useCountdown(
  etaSeconds: number,
  resetKey: unknown,
  enabled: boolean,
): number {
  const [now, setNow] = useState(() => Date.now())
  const anchor = useRef(Date.now())

  useEffect(() => {
    anchor.current = Date.now()
    setNow(anchor.current)
  }, [resetKey])

  useEffect(() => {
    if (!enabled) return
    const timer = setInterval(() => setNow(Date.now()), 15_000)
    return () => clearInterval(timer)
  }, [enabled])

  if (!enabled) return etaSeconds
  const elapsed = Math.floor((now - anchor.current) / 1000)
  return Math.max(FLOOR_SECONDS, etaSeconds - elapsed)
}
