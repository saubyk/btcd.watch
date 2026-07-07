import { useEffect, useRef, useState } from 'react'

import { api } from '../api/client'
import type { FeeEstimate, Stats } from '../api/types'
import { live } from '../api/ws'
import { appConfig } from '../appConfig'

export interface NetworkData {
  stats: Stats | null
  fees: FeeEstimate | null
  /** True while the backend (and its node) are reachable. */
  connected: boolean
}

/**
 * Landing-page data. Stats arrive as WebSocket pushes (on connect, per
 * block, and periodic ticks); fees are refetched when the block height
 * moves. A slow REST poll remains as a fallback while the socket is down.
 */
export function useNetworkData(): NetworkData {
  const [data, setData] = useState<NetworkData>({
    stats: null,
    fees: null,
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

  return data
}
