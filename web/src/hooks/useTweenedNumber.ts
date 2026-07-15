import { useEffect, useRef, useState } from 'react'

/** Tween cadence and easing from the round-7 prototype: ~35% of the
 * remaining delta every 90ms, snapping once within 40. */
const STEP_MS = 90
const STEP_FRACTION = 0.35
const SNAP_WITHIN = 40

/**
 * Eases the displayed value toward the real one so live counts drift
 * instead of jumping (round-7 "tweened counts"). The first value is
 * adopted as-is — mount the caller once real data exists so page load
 * doesn't count up from zero. Disabled → passthrough.
 */
export function useTweenedNumber(value: number, enabled: boolean): number {
  const [disp, setDisp] = useState(value)
  const dispRef = useRef(value)

  useEffect(() => {
    if (!enabled || Math.abs(value - dispRef.current) < SNAP_WITHIN) {
      dispRef.current = value
      setDisp(value)
      return
    }
    const timer = setInterval(() => {
      const delta = value - dispRef.current
      if (Math.abs(delta) < SNAP_WITHIN) {
        dispRef.current = value
        clearInterval(timer)
      } else {
        dispRef.current += delta * STEP_FRACTION
      }
      setDisp(dispRef.current)
    }, STEP_MS)
    return () => clearInterval(timer)
  }, [value, enabled])

  return Math.round(disp)
}
