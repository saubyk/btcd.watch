import { useSyncExternalStore } from 'react'

/**
 * Subscribe to a CSS media query. Returns whether it currently matches and
 * re-renders on change — the framework-native equivalent of the round-5
 * prototype's `isNarrow` resize listener, without the manual bookkeeping.
 */
export function useMediaQuery(query: string): boolean {
  return useSyncExternalStore(
    (onChange) => {
      const mql = window.matchMedia(query)
      mql.addEventListener('change', onChange)
      return () => mql.removeEventListener('change', onChange)
    },
    () => window.matchMedia(query).matches,
    () => false,
  )
}
