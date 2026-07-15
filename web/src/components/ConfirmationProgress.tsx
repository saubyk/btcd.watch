import { useMotionMode } from '../hooks/useMotion'
import { formatNumber } from '../lib/format'

const TARGET = 6

/**
 * The six-segment confirmation bar. Below six confirmations it counts
 * "N of 6"; at or beyond it reads fully settled. When the tx confirmed
 * while being watched, the segments pop in staggered (round-7).
 */
export function ConfirmationProgress({
  confirmations,
  justConfirmed = false,
}: {
  confirmations: number
  justConfirmed?: boolean
}) {
  const motionOn = useMotionMode() !== 'off'
  const pop = justConfirmed && motionOn
  const filled = Math.min(confirmations, TARGET)
  const settled = confirmations >= TARGET

  const count = settled
    ? `${confirmations >= 1000 ? formatNumber(confirmations) : '6+'} confirmations`
    : `${confirmations} of ${TARGET}`

  const caption = settled
    ? 'Fully settled — far beyond the 6-confirmation safety mark, so this payment is permanent.'
    : confirmations === 0
      ? 'Waiting for its first block. Once mined, each additional block makes it safer.'
      : `Almost there — ${TARGET - confirmations} more ${
          TARGET - confirmations === 1 ? 'block' : 'blocks'
        } until it is fully settled.`

  return (
    <div className="bp-progress">
      <div className="bp-progress-head">
        <span className="bp-progress-label">Confirmation progress</span>
        <span className="bp-progress-count">{count}</span>
      </div>
      <div className="bp-progress-track">
        {Array.from({ length: TARGET }, (_, i) => (
          <div
            key={i}
            className={`bp-progress-seg ${
              i < filled ? 'bp-progress-seg--filled' : 'bp-progress-seg--empty'
            }${pop ? ' bp-progress-seg--pop' : ''}`}
            style={pop ? { animationDelay: `${i * 90}ms` } : undefined}
          />
        ))}
      </div>
      <div className="bp-progress-caption">{caption}</div>
    </div>
  )
}
