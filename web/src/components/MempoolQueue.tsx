import { useEffect, useState } from 'react'

import type {
  Arrival,
  BlockFlash,
  MempoolUpdate,
  Queue,
  QueueBand,
  Stats,
} from '../api/types'
import {
  formatAgeShort,
  formatBtc,
  formatEtaShort,
  formatNumber,
} from '../lib/format'
import { Marker, QueueSegments } from './QueueBar'

/** The feed shows at most this many arrivals. */
const FEED_ROWS = 6

/** Design clamps (round-3 handoff): the bar never shrinks below 28% of
 * the capacity track, and the cutoff marker never sits past 60% of the
 * colored bar — aesthetics over literal proportion at the extremes. */
const MIN_BAR_FRACTION = 0.28
const MAX_CUTOFF_FRACTION = 0.6

/** "15+ sat/vB" for the open-ended front band, "10–15" after. */
function bandLabel(band: QueueBand): string {
  return band.maxSatPerVb === 0
    ? `${band.minSatPerVb}+ sat/vB`
    : `${band.minSatPerVb}–${band.maxSatPerVb}`
}

function bandIndex(queue: Queue, rate: number): number {
  for (let i = 0; i < queue.bands.length - 1; i++) {
    if (rate >= queue.bands[i]!.minSatPerVb) return i
  }
  return queue.bands.length - 1
}

/** Blocks a tx at this rate typically waits: vbytes queued in front of
 * its band over the per-block capacity the server encoded in
 * cutoffFraction (cutoff 1 = everything fits in the next block). */
function bandEtaBlocks(queue: Queue, rate: number): number {
  if (queue.cutoffFraction >= 1) return 1
  const band = bandIndex(queue, rate)
  let aheadVbytes = 0
  for (let i = 0; i < band; i++) aheadVbytes += queue.bands[i]!.vbytes
  const blockVbytes = queue.totalVbytes * queue.cutoffFraction
  return Math.floor(aheadVbytes / blockVbytes) + 1
}

/** "≈52 min" / "≈2 hrs" — formatEtaShort's shape with the ≈ the design
 * caption uses. */
function approx(seconds: number): string {
  return formatEtaShort(seconds).replace('~', '≈')
}

/**
 * Landing section: the live mempool as a queue on a capacity track. The
 * bar grows and shrinks with real traffic (live pushes), a banner flashes
 * when a block is mined, and the newest unconfirmed transactions stream
 * into the "Just joined the line" feed.
 */
export function MempoolQueue({
  stats,
  mempool,
  minedFlash,
  onSearch,
}: {
  stats: Stats | null
  mempool: MempoolUpdate | null
  minedFlash: BlockFlash | null
  onSearch: (q: string) => void
}) {
  // Live pushes are fresher than the 10s stats queue; use whichever is
  // newest available.
  const queue = mempool?.queue ?? stats?.queue
  if (!stats || !queue) return null

  const threshold = Math.max(1, Math.ceil(queue.nextBlockRate))
  const interval = stats.avgBlockIntervalSeconds

  // Bar width on the capacity track, and the moving cutoff within it.
  const barFraction = Math.min(
    1,
    Math.max(
      MIN_BAR_FRACTION,
      queue.peakVbytes > 0 ? queue.totalVbytes / queue.peakVbytes : 0,
    ),
  )
  const cutoffFraction =
    barFraction * Math.min(queue.cutoffFraction, MAX_CUTOFF_FRACTION)

  // Whole-line clear time: cutoffFraction is per-block capacity over
  // total depth, so its inverse is blocks-to-drain (no duplicated
  // block-size constant).
  const clearBlocks = Math.max(
    1,
    Math.round((1 / queue.cutoffFraction) * 10) / 10,
  )

  return (
    <section className="bp-mempool-section">
      <div className="bp-mempool-card">
        <div className="bp-mempool-head">
          <h2>The line right now</h2>
          <span className="bp-mempool-live">
            <span className="bp-live-dot bp-live-dot--sm bp-pulse-slow" />
            Live ·{' '}
            <span className="bp-mempool-count">
              {formatNumber(queue.txCount)}
            </span>
            &nbsp;waiting
          </span>
        </div>
        <p className="bp-mempool-sub">
          Every unconfirmed transaction, queued by fee. Miners take from the
          front — the bar grows and shrinks with real traffic.
        </p>

        {minedFlash && (
          <div className="bp-mined-flash">
            <span className="bp-mined-flash-emoji">⛏️</span>
            <span className="bp-mined-flash-text">
              Block {formatNumber(minedFlash.height)} just mined —{' '}
              {formatNumber(minedFlash.txCount)}{' '}
              {minedFlash.txCount === 1 ? 'transaction' : 'transactions'} left
              the front of the line
            </span>
          </div>
        )}

        <div className="bp-queue bp-queue--live">
          <div className="bp-queue-track">
            <div
              className="bp-queue-bar bp-queue-bar--live"
              style={{ width: `${barFraction * 100}%` }}
            >
              <QueueSegments queue={queue} />
            </div>
            <Marker
              fraction={cutoffFraction}
              kind="cutoff"
              label="next-block cutoff"
            />
          </div>
        </div>

        <div className="bp-queue-captions">
          <span>← Front of line (paid more)</span>
          <span>
            Whole line clears in{' '}
            <strong>
              ~{clearBlocks} {clearBlocks === 1 ? 'block' : 'blocks'} (
              {approx(clearBlocks * interval)})
            </strong>{' '}
            · dashed track = recent peak
          </span>
        </div>

        <div className="bp-queue-legend">
          {queue.bands.map((band, i) => (
            <span key={band.minSatPerVb} className="bp-legend-item">
              <span className={`bp-legend-swatch bp-queue-seg--${i}`} />
              {bandLabel(band)}
            </span>
          ))}
        </div>

        <ArrivalsFeed
          queue={queue}
          arrivals={mempool?.arrivals ?? []}
          interval={interval}
          onSearch={onSearch}
        />

        <div className="bp-takeaway">
          Pay <strong>{threshold}+ sat/vB</strong> and you'll likely make the
          next block ({formatEtaShort(stats.nextBlockEtaSeconds)}). Track a
          pending transaction and we'll show your place in this line.
        </div>
      </div>
    </section>
  )
}

function ArrivalsFeed({
  queue,
  arrivals,
  interval,
  onSearch,
}: {
  queue: Queue
  arrivals: Arrival[]
  interval: number
  onSearch: (q: string) => void
}) {
  // Ages tick without new pushes.
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (arrivals.length === 0) return
    const timer = setInterval(() => setNow(Date.now()), 10_000)
    return () => clearInterval(timer)
  }, [arrivals.length])

  if (arrivals.length === 0) return null

  return (
    <div className="bp-feed">
      <div className="bp-feed-head">
        <span className="bp-feed-title">Just joined the line</span>
        <span className="bp-feed-hint">tap one to inspect it</span>
      </div>
      {arrivals.slice(0, FEED_ROWS).map((a) => (
        <button
          key={a.txid}
          className="bp-feed-row"
          onClick={() => onSearch(a.txid)}
          title="Track this unconfirmed transaction"
        >
          <span
            className={`bp-feed-dot bp-feed-dot--${bandIndex(queue, a.feeRateSatPerVb)}`}
          />
          <span className="bp-feed-txid">{a.txid}</span>
          <span className="bp-feed-age">{formatAgeShort(a.time, now)}</span>
          <span className="bp-feed-rate">
            {Math.round(a.feeRateSatPerVb * 10) / 10} sat/vB
          </span>
          <span className="bp-feed-eta">
            {etaChip(queue, a.feeRateSatPerVb, interval)}
          </span>
          <span className="bp-feed-amount">{formatBtc(a.amountSats)} BTC</span>
        </button>
      ))}
    </div>
  )
}

/** ETA chip from the queue's own depth and the measured block interval. */
function etaChip(queue: Queue, rate: number, interval: number): string {
  const blocks = bandEtaBlocks(queue, rate)
  if (blocks === 1) return 'next block'
  return formatEtaShort(blocks * interval)
}
