import { useEffect, useState } from 'react'

import { api } from '../api/client'
import type { Examples, FeeEstimate, Stats } from '../api/types'
import { appConfig } from '../appConfig'

export interface NetworkData {
  stats: Stats | null
  fees: FeeEstimate | null
  examples: Examples | null
  /** False until the first stats fetch settles, then true on success. */
  connected: boolean
}

/**
 * Landing-page data: stats, fee tiers, and example chips, refreshed on an
 * interval. (The live-update milestone replaces polling with WebSocket
 * push.)
 */
export function useNetworkData(): NetworkData {
  const [data, setData] = useState<NetworkData>({
    stats: null,
    fees: null,
    examples: null,
    connected: false,
  })

  useEffect(() => {
    let cancelled = false

    const load = async () => {
      const [stats, fees, examples] = await Promise.allSettled([
        api.stats(),
        api.fees(),
        api.examples(),
      ])
      if (cancelled) return
      setData((prev) => ({
        stats: stats.status === 'fulfilled' ? stats.value : prev.stats,
        fees: fees.status === 'fulfilled' ? fees.value : prev.fees,
        examples:
          examples.status === 'fulfilled' ? examples.value : prev.examples,
        connected: stats.status === 'fulfilled',
      }))
    }

    load()
    const timer = setInterval(load, appConfig.statsRefreshSeconds * 1000)
    return () => {
      cancelled = true
      clearInterval(timer)
    }
  }, [])

  return data
}
