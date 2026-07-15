import {
  LiveStatusPill,
  NodeCta,
  SearchBar,
  StatsBar,
} from '../components/Landing'
import { MempoolQueue } from '../components/MempoolQueue'
import { appConfig } from '../appConfig'
import type { NetworkData } from '../hooks/useNetworkData'

export function Landing({
  data,
  onSearch,
}: {
  data: NetworkData
  onSearch: (q: string) => void
}) {
  // While the node is unreachable or still syncing, lookups can't be
  // answered (the API gates them too) — the pill explains the state and
  // the search/stats/mempool features stay hidden.
  const ready = data.stats != null && !data.stats.syncing

  return (
    <main className="bp-view">
      <section className="bp-hero">
        <LiveStatusPill stats={data.stats} connected={data.connected} />
        <h1>
          Is my Bitcoin
          <br />
          transaction confirmed?
        </h1>
        <p className="bp-hero-sub">
          Paste a transaction ID or wallet address. We'll tell you what's
          happening — in plain English.
        </p>
        {ready && <SearchBar onSearch={onSearch} />}
      </section>

      {/* Round 4: high-level stats first, then the detailed queue. */}
      {ready && appConfig.showStats && (
        <StatsBar stats={data.stats} onSearch={onSearch} />
      )}
      {ready && appConfig.showMempool && (
        <MempoolQueue
          stats={data.stats}
          mempool={data.mempool}
          minedFlash={data.minedFlash}
          onSearch={onSearch}
        />
      )}
      <NodeCta />
    </main>
  )
}
