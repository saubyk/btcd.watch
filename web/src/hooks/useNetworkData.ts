import { useEffect, useRef, useState } from 'react'

import { api } from '../api/client'
import type {
  BlockFlash,
  FeeEstimate,
  MempoolUpdate,
  Stats,
} from '../api/types'
import { live } from '../api/ws'
import { appConfig } from '../appConfig'

export interface NetworkData {
  stats: Stats | null
  fees: FeeEstimate | null
  /** Latest live mempool push (fresher than stats.queue); null until the
   * first push, or always with liveMempool off. */
  mempool: MempoolUpdate | null
  /** Recently mined block, cleared after minedFlashSeconds. */
  minedFlash: BlockFlash | null
  /** True while the backend (and its node) are reachable. */
  connected: boolean
}

/**
 * Landing-page data. Stats arrive as WebSocket pushes (on connect, per
 * block, and periodic ticks); the live mempool layer pushes queue +
 * arrivals on tx-accepted (throttled) and a flash per block. Fees are
 * refetched when the block height moves. A slow REST poll remains as a
 * fallback while the socket is down.
 */
export function useNetworkData(): NetworkData {
  const [data, setData] = useState<NetworkData>({
    stats: null,
    fees: null,
    mempool: null,
    minedFlash: null,
    connected: false,
  })
  const lastHeight = useRef(0)

  // Fees: fetched on mount and per new block (keyed off the stats push
  // height).
  const refreshFees = async () => {
    try {
      const fees = await api.fees()
      setData((prev) => ({ ...prev, fees }))
    } catch {
      // Keep the previous rates while the node is unreachable.
    }
  }

  useEffect(() => {
    const applyStats = (stats: Stats) => {
      setData((prev) => ({ ...prev, stats, connected: true }))
      if (stats.blockHeight !== lastHeight.current) {
        lastHeight.current = stats.blockHeight
        void refreshFees()
      }
    }

    const offStats = live.onStats(applyStats)
    const offConn = live.onConnection((open) => {
      if (!open) setData((prev) => ({ ...prev, connected: false }))
    })

    // Initial load + REST fallback while the socket is down.
    const load = async () => {
      if (live.isOpen) return
      try {
        applyStats(await api.stats())
      } catch {
        setData((prev) => ({ ...prev, connected: false }))
      }
    }
    void load()
    const timer = setInterval(load, appConfig.statsRefreshSeconds * 1000)

    return () => {
      offStats()
      offConn()
      clearInterval(timer)
    }
  }, [])

  // Live mempool layer: queue/arrivals pushes plus the block-mined flash.
  useEffect(() => {
    if (!appConfig.liveMempool) return

    const offMempool = live.onMempool((mempool) => {
      setData((prev) => ({
        ...prev,
        mempool,
        // The dark stats bar's mempool tile follows the same feed; keep
        // the stats reference stable when the count hasn't moved.
        stats:
          prev.stats &&
          prev.stats.mempool.txCount !== mempool.queue.txCount
            ? {
                ...prev.stats,
                mempool: {
                  ...prev.stats.mempool,
                  txCount: mempool.queue.txCount,
                },
              }
            : prev.stats,
      }))
    })

    let flashTimer: ReturnType<typeof setTimeout> | undefined
    const offBlock = live.onBlock((minedFlash) => {
      setData((prev) => ({ ...prev, minedFlash }))
      clearTimeout(flashTimer)
      flashTimer = setTimeout(() => {
        setData((prev) => ({ ...prev, minedFlash: null }))
      }, appConfig.minedFlashSeconds * 1000)
    })

    // A dropped socket must not leave a frozen live payload shadowing the
    // REST-fallback stats.queue.
    const offConn = live.onConnection((open) => {
      if (!open) {
        setData((prev) => ({ ...prev, mempool: null, minedFlash: null }))
      }
    })

    return () => {
      offMempool()
      offBlock()
      offConn()
      clearTimeout(flashTimer)
    }
  }, [])

  return data
}
