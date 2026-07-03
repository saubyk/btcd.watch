import { useEffect, useRef, useState, type Dispatch } from 'react'

import { api } from '../api/client'
import { appConfig } from '../appConfig'
import type { Action } from '../state'

/**
 * REST-polling watch on a pending tx: counts down, refetches, and
 * dispatches tx-updated (which flips the view when the tx confirms). The
 * live-update milestone swaps the internals for a WebSocket subscription;
 * the countdown UI stays.
 *
 * Returns seconds until the next check.
 */
export function useWatchTx(
  txid: string | undefined,
  active: boolean,
  dispatch: Dispatch<Action>,
): number {
  const [secsLeft, setSecsLeft] = useState(appConfig.watchPollSeconds)
  const checking = useRef(false)

  useEffect(() => {
    if (!active || !txid) return

    setSecsLeft(appConfig.watchPollSeconds)
    const timer = setInterval(() => {
      setSecsLeft((s) => {
        if (s > 1) return s - 1
        if (!checking.current) {
          checking.current = true
          api
            .tx(txid)
            .then((tx) => dispatch({ type: 'tx-updated', tx }))
            .catch(() => {
              // Node hiccup — keep watching; next poll retries.
            })
            .finally(() => {
              checking.current = false
            })
        }
        return appConfig.watchPollSeconds
      })
    }, 1000)

    return () => clearInterval(timer)
  }, [active, txid, dispatch])

  return secsLeft
}
