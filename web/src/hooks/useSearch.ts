import { useCallback, useRef, type Dispatch } from 'react'

import { api } from '../api/client'
import type { Action } from '../state'

const MIN_LOADING_MS = 400

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

/**
 * Runs a search with a minimum loading duration (avoids a flash of the
 * skeleton on fast regtest responses). Stale responses from superseded
 * searches are dropped.
 */
export function useSearch(dispatch: Dispatch<Action>) {
  const seq = useRef(0)

  return useCallback(
    async (raw: string) => {
      const query = raw.trim()
      if (!query) return

      const mySeq = ++seq.current
      dispatch({ type: 'search-start', query })
      window.scrollTo({ top: 0 })

      const started = Date.now()
      try {
        const result = await api.search(query)
        await sleep(Math.max(0, MIN_LOADING_MS - (Date.now() - started)))
        if (seq.current !== mySeq) return
        dispatch({ type: 'search-result', result })
      } catch (err) {
        await sleep(Math.max(0, MIN_LOADING_MS - (Date.now() - started)))
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
