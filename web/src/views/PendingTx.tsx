import type { Dispatch } from 'react'

import type { FeeEstimate, Stats, Tx } from '../api/types'
import { ClockIcon } from '../components/Icons'
import { QueueBar } from '../components/QueueBar'
import { AmountHeader, BackButton, StatTile } from '../components/ResultParts'
import { useWatchTx } from '../hooks/useWatchTx'
import { formatEta, formatNumber } from '../lib/format'
import type { Action } from '../state'

export function PendingTx({
  tx,
  fees,
  stats,
  watching,
  dispatch,
  onHome,
}: {
  tx: Tx
  fees: FeeEstimate | null
  stats: Stats | null
  watching: boolean
  dispatch: Dispatch<Action>
  onHome: () => void
}) {
  const watch = useWatchTx(tx.txid, watching, dispatch)
  const rate = tx.feeRateSatPerVb ?? 0
  const urgent = fees?.tiers.find((t) => t.id === 'urgent')
  const queue = stats?.queue ?? null

  return (
    <main className="bp-view bp-result">
      <BackButton onClick={onHome} />

      <div className="bp-card bp-card--pending">
        <div className="bp-card-head bp-card-head--pending">
          <span className="bp-status-badge bp-status-badge--pending bp-pulse">
            <ClockIcon />
          </span>
          <div>
            <div className="bp-status-title bp-status-title--pending">
              Waiting to confirm
            </div>
            <div className="bp-status-sub bp-status-sub--pending">
              In the queue — not yet in a block.
            </div>
          </div>
        </div>

        <div className="bp-card-body">
          <AmountHeader tx={tx} />

          {tx.pending && (
            <div className="bp-wait-panel">
              <div
                className={`bp-wait-head${queue ? ' bp-wait-head--queue' : ''}`}
              >
                <span className="bp-wait-label">Your place in line</span>
                <span className="bp-wait-eta">
                  {formatEta(tx.pending.etaSeconds)}
                </span>
              </div>
              {queue && (
                <QueueBar
                  queue={queue}
                  compact
                  youFraction={tx.pending.queueVbytesFraction}
                />
              )}
              <div
                className={`bp-wait-foot${queue ? ' bp-wait-foot--queue' : ''}`}
              >
                {/* Until the first stats push there is no bar for the
                    directional caption to point at. */}
                <span>{queue ? '← Front of line' : 'In mempool'}</span>
                <span>
                  {tx.pending.txsAhead === 0
                    ? 'next in line'
                    : `~${formatNumber(
                        tx.pending.txsAhead,
                      )} transactions ahead of you`}
                </span>
              </div>
            </div>
          )}

          {watching ? (
            <div className="bp-watch-panel">
              <span className="bp-watch-dot bp-pulse" />
              <div className="bp-watch-text">
                <div className="bp-watch-title">
                  Watching for confirmation…
                </div>
                <div className="bp-watch-sub">
                  {watch.live
                    ? "Live — connected to your btcd node; this page flips the moment it lands in a block."
                    : `Auto-checking your btcd node — we'll alert you the moment it lands. Next check in ${watch.secsLeft}s.`}
                </div>
              </div>
              <button
                className="bp-watch-stop"
                onClick={() => dispatch({ type: 'watch-stop' })}
              >
                Stop
              </button>
            </div>
          ) : (
            <button
              className="bp-watch-btn"
              onClick={() => dispatch({ type: 'watch-start' })}
            >
              🔔 Watch this transaction
              <span className="bp-watch-btn-sub">
                — alert me when it confirms
              </span>
            </button>
          )}

          <div className="bp-tiles" style={{ marginTop: 0 }}>
            <StatTile label="This tx paid">
              {Math.round(rate * 10) / 10}{' '}
              <span className="bp-tile-unit">sat/vB</span>
            </StatTile>
            <StatTile label="Confirmations">0</StatTile>
          </div>

          <div className="bp-explain bp-explain--amber">
            <div className="bp-explain-title">
              Why is it taking a while? ⏳
            </div>
            <p>
              {urgent && rate < urgent.satPerVb ? (
                <>
                  This transaction paid{' '}
                  <strong>{Math.round(rate * 10) / 10} sat/vB</strong> — below
                  the <strong>Urgent</strong> rate of{' '}
                  {Math.round(urgent.satPerVb)}. Miners pick the
                  highest-paying transactions first, so yours waits until the
                  network quiets down or enough higher-fee transactions
                  clear. It will still confirm; it just isn't first in line.
                </>
              ) : (
                <>
                  This transaction pays a competitive fee, so it's near the
                  front of the queue. It just needs the next block to be
                  mined — on average that happens{' '}
                  {tx.pending
                    ? formatEta(
                        Math.max(
                          tx.pending.etaSeconds / Math.max(tx.pending.etaBlocks, 1),
                          1,
                        ),
                      ).replace('~', 'every ~')
                    : 'every few minutes'}
                  .
                </>
              )}
            </p>
          </div>
        </div>
      </div>
    </main>
  )
}
