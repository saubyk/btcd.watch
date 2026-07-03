import {
  ExampleChips,
  FeeEstimator,
  LiveStatusPill,
  NodeCta,
  SearchBar,
  StatsBar,
} from '../components/Landing'
import { appConfig } from '../appConfig'
import type { NetworkData } from '../hooks/useNetworkData'

export function Landing({
  data,
  onSearch,
}: {
  data: NetworkData
  onSearch: (q: string) => void
}) {
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
        <SearchBar onSearch={onSearch} />
        <ExampleChips examples={data.examples} onSearch={onSearch} />
      </section>

      {appConfig.showFeeEstimator && <FeeEstimator fees={data.fees} />}
      {appConfig.showStats && <StatsBar stats={data.stats} />}
      <NodeCta />
    </main>
  )
}
