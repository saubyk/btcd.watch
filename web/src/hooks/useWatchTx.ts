import { useEffect, useRef, useState, type Dispatch } from 'react'

import { api } from '../api/client'
import { live } from '../api/ws'
import { appConfig } from '../appConfig'
import type { Action } from '../state'

export interface WatchState {
  /** True when updates arrive as WebSocket pushes. */
  live: boolean
  /** Seconds until the next check while on the polling fallback. */
  secsLeft: number
}

/**
 * Watches a pending tx. Primary path: a WebSocket subscription — the
 * server pushes queue-position updates and flips the view the moment a
 * block includes the tx. While the socket is down, a REST poll fallback
 * takes over (with the visible countdown).
 */
export function useWatchTx(
  txid: string | undefined,
  active: boolean,
  dispatch: Dispatch<Action>,
): WatchState {
  const [connected, setConnected] = useState(live.isOpen)
  const [secsLeft, setSecsLeft] = useState(appConfig.watchPollSeconds)
  const checking = useRef(false)

  // Confirmation handling is identical for both paths: fetch the full
  // payload once so the confirmed view renders complete data.
  const confirm = () => {
    if (!txid || checking.current) return
    checking.current = true
    api
      .tx(txid)
      .then((tx) => dispatch({ type: 'tx-updated', tx }))
      .catch(() => {})
      .finally(() => {
        checking.current = false
      })
  }

  // WebSocket subscription.
  useEffect(() => {
    if (!active || !txid) return

    const offConn = live.onConnection(setConnected)
    const offWatch = live.watch(txid, (update) => {
      if (update.status === 'confirmed') {
        confirm()
      } else if (update.txsAhead != null && update.etaSeconds != null) {
        dispatch({
          type: 'tx-queue',
          txsAhead: update.txsAhead,
          etaSeconds: update.etaSeconds,
          queueVbytesFraction: update.queueVbytesFraction,
        })
      }
    })
    setConnected(live.isOpen)

    return () => {
      offConn()
      offWatch()
    }
    // confirm is stable per txid via refs.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, txid, dispatch])

  // REST polling fallback while the socket is down.
  useEffect(() => {
    if (!active || !txid || connected) return

    setSecsLeft(appConfig.watchPollSeconds)
    const timer = setInterval(() => {
      setSecsLeft((s) => {
        if (s > 1) return s - 1
        confirm()
        return appConfig.watchPollSeconds
      })
    }, 1000)

    return () => clearInterval(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active, txid, connected, dispatch])

  return { live: connected, secsLeft }
}
