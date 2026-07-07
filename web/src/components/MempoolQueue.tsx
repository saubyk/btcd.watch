import type { QueueBand, Stats } from '../api/types'
import { formatEtaShort, formatNumber } from '../lib/format'
import { QueueBar } from './QueueBar'

/** "15+ sat/vB" for the open-ended front band, "10–15" after. */
function bandLabel(band: QueueBand): string {
  return band.maxSatPerVb === 0
    ? `${band.minSatPerVb}+ sat/vB`
    : `${band.minSatPerVb}–${band.maxSatPerVb}`
}

/**
 * Landing section: the live mempool visualized as a queue, refreshed with
 * every stats push.
 */
export function MempoolQueue({ stats }: { stats: Stats | null }) {
  const queue = stats?.queue
  if (!stats || !queue) return null

  const threshold = Math.max(1, Math.ceil(queue.nextBlockRate))

  return (
    <section className="bp-mempool-section">
      <div className="bp-mempool-card">
        <div className="bp-mempool-head">
          <h2>The line right now</h2>
          <span className="bp-mempool-live">
            <span className="bp-live-dot bp-live-dot--sm bp-pulse-slow" />
            Live · {formatNumber(queue.txCount)} waiting
          </span>
        </div>
        <p className="bp-mempool-sub">
          Every unconfirmed transaction, queued by fee. Miners take from the
          front.
        </p>

        <QueueBar queue={queue} />

        <div className="bp-queue-captions">
          <span>← Front of line (paid more)</span>
          <span>Back of line (paid less) →</span>
        </div>

        <div className="bp-queue-legend">
          {queue.bands.map((band, i) => (
            <span key={band.minSatPerVb} className="bp-legend-item">
              <span className={`bp-legend-swatch bp-queue-seg--${i}`} />
              {bandLabel(band)}
            </span>
          ))}
        </div>

        <div className="bp-takeaway">
          Pay <strong>{threshold}+ sat/vB</strong> and you'll likely make the
          next block ({formatEtaShort(stats.nextBlockEtaSeconds)}). Track a
          pending transaction and we'll show your place in this line.
        </div>
      </div>
    </section>
  )
}
