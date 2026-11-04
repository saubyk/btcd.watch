import { useCallback, useRef, type Dispatch } from 'react'

import { api } from '../api/client'
import { kindOfResult, markKind, pushQuery } from '../lib/nav'
import type { Action } from '../state'

const MIN_LOADING_MS = 400

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

export interface SearchOptions {
  /**
   * The history entry already shows this query (cold load, Back/Forward):
   * don't push one, skip the minimum skeleton time, and restore this
   * scroll offset once the result renders.
   */
  restore?: { scrollY: number }
}

/**
 * Runs a search with a minimum loading duration (avoids a flash of the
 * skeleton on fast regtest responses). A new search pushes a history
 * entry at `?q=…` so Back returns to the previous result; stale
 * responses from superseded searches are dropped.
 */
export function useSearch(dispatch: Dispatch<Action>) {
  const seq = useRef(0)

  return useCallback(
    async (raw: string, opts: SearchOptions = {}) => {
      const query = raw.trim()
      if (!query) return

      const mySeq = ++seq.current
      dispatch({ type: 'search-start', query })
      // Keep the URL shareable; the SPA fallback renders /?q=… cold.
      if (!opts.restore) pushQuery(query)
      window.scrollTo({ top: 0 })

      const minMs = opts.restore ? 0 : MIN_LOADING_MS
      const started = Date.now()
      try {
        const result = await api.search(query)
        await sleep(Math.max(0, minMs - (Date.now() - started)))
        if (seq.current !== mySeq) return
        dispatch({ type: 'search-result', result })
        markKind(kindOfResult(result))
        if (opts.restore?.scrollY) {
          const y = opts.restore.scrollY
          // After React commits the result view.
          requestAnimationFrame(() => window.scrollTo({ top: y }))
        }
      } catch (err) {
        await sleep(Math.max(0, minMs - (Date.now() - started)))
        if (seq.current !== mySeq) return
        dispatch({
          type: 'search-error',
          message:
            err instanceof Error ? err.message : 'something went wrong',
        })
      }
    },
    [dispatch],
  )
}
