import type { Queue } from '../api/types'

/**
 * The mempool "line": one horizontal bar split into fee-band segments,
 * front of the line (highest fee) on the left. Width is proportional to
 * the vbytes waiting in each band. Markers overhang the bar, so callers
 * must leave ~32px clearance above (and below, when `youFraction` is set)
 * for the pill labels.
 */
export function QueueBar({
  queue,
  compact = false,
  youFraction,
}: {
  queue: Queue
  compact?: boolean
  youFraction?: number
}) {
  const total = queue.totalVbytes

  return (
    <div className="bp-queue">
      <div className={`bp-queue-bar${compact ? ' bp-queue-bar--compact' : ''}`}>
        {total === 0 ? (
          <div className="bp-queue-seg bp-queue-seg--4" style={{ width: '100%' }} />
        ) : (
          queue.bands.map((band, i) => (
            <div
              key={i}
              className={`bp-queue-seg bp-queue-seg--${i}`}
              style={{ width: `${(band.vbytes / total) * 100}%` }}
            />
          ))
        )}
      </div>
      <Marker fraction={queue.cutoffFraction} kind="cutoff" label="next-block cutoff" />
      {youFraction != null && (
        <Marker fraction={youFraction} kind="you" label="you are here" />
      )}
    </div>
  )
}

function Marker({
  fraction,
  kind,
  label,
}: {
  fraction: number
  kind: 'cutoff' | 'you'
  label: string
}) {
  const pct = Math.min(Math.max(fraction, 0), 1) * 100
  // The pill label is centered on the line, but its center is clamped a
  // half-label-width from either edge so it never overflows the card,
  // whatever the bar width.
  const half = kind === 'cutoff' ? '52px' : '42px'
  const labelLeft = `min(max(${pct}%, ${half}), calc(100% - ${half}))`

  return (
    <>
      <div className={`bp-queue-mark bp-queue-mark--${kind}`} style={{ left: `${pct}%` }} />
      <div
        className={`bp-queue-mark-label bp-queue-mark-label--${kind}`}
        style={{ left: labelLeft }}
      >
        {label}
      </div>
    </>
  )
}
