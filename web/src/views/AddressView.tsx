import { useState, type Dispatch } from 'react'

import { api } from '../api/client'
import type { AddressActivity, AddressSummary } from '../api/types'
import { CopyIcon } from '../components/Icons'
import { BackButton } from '../components/ResultParts'
import { useCopy } from '../components/Toast'
import {
  formatBtc,
  formatFiat,
  formatNumber,
  formatRelative,
} from '../lib/format'
import type { Action } from '../state'

export function AddressView({
  summary,
  dispatch,
  onHome,
}: {
  summary: AddressSummary
  dispatch: Dispatch<Action>
  onHome: () => void
}) {
  const copy = useCopy()
  const [loadingMore, setLoadingMore] = useState(false)

  const loadMore = async () => {
    setLoadingMore(true)
    try {
      const page = await api.address(
        summary.address,
        summary.offset + summary.activity.length,
        summary.limit,
      )
      dispatch({ type: 'address-more', page })
    } catch {
      // Leave the list as-is; the button stays for a retry.
    } finally {
      setLoadingMore(false)
    }
  }

  return (
    <main className="bp-view bp-result">
      <BackButton onClick={onHome} />

      <div className="bp-card">
        <div className="bp-card-body">
          <div className="bp-address-label">◆ Wallet address</div>
          <div className="bp-address-row">
            <div className="bp-address-value">{summary.address}</div>
            <button
              className="bp-copy-btn"
              onClick={() => copy(summary.address)}
            >
              <CopyIcon />
              Copy
            </button>
          </div>

          <div className="bp-amount-row">
            <span className="bp-amount bp-amount--lg">
              {formatBtc(summary.balanceSats)}{' '}
              <span className="bp-amount-unit">BTC</span>
            </span>
            {summary.fiatUsd != null && (
              <span className="bp-amount-fiat bp-amount-fiat--lg">
                ≈ {formatFiat(summary.fiatUsd)}
              </span>
            )}
          </div>
          <div className="bp-balance-caption">
            Current balance{summary.approximate && ' (approximate)'}
          </div>

          <div className="bp-addr-tiles">
            <div className="bp-addr-tile bp-addr-tile--received">
              <div className="bp-addr-tile-label">Total received</div>
              <div className="bp-addr-tile-value">
                {formatBtc(summary.receivedSats)} BTC
              </div>
            </div>
            <div className="bp-addr-tile bp-addr-tile--sent">
              <div className="bp-addr-tile-label">Total sent</div>
              <div className="bp-addr-tile-value">
                {formatBtc(summary.sentSats)} BTC
              </div>
            </div>
            <div className="bp-addr-tile">
              <div className="bp-addr-tile-label">Transactions</div>
              <div className="bp-addr-tile-value">
                {formatNumber(summary.txCount)}
                {summary.approximate && '+'}
              </div>
            </div>
          </div>
        </div>

        {summary.activity.length > 0 && (
          <div className="bp-activity">
            <div className="bp-activity-head">Recent activity</div>
            {summary.activity.map((a) => (
              <ActivityRow key={a.txid + a.direction} activity={a} />
            ))}
            {summary.hasMore && (
              <div className="bp-activity-more">
                <button
                  className="bp-chip"
                  onClick={loadMore}
                  disabled={loadingMore}
                >
                  {loadingMore ? 'Loading…' : 'Load more'}
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </main>
  )
}

const ROW_META = {
  received: { icon: '↓', label: 'Received', sign: '+' },
  sent: { icon: '↑', label: 'Sent', sign: '−' },
  self: { icon: '⟳', label: 'Self transfer', sign: '−' },
} as const

function ActivityRow({ activity }: { activity: AddressActivity }) {
  const meta = ROW_META[activity.direction]
  const pending = activity.status === 'pending'

  const when = pending
    ? activity.time > 0
      ? formatRelative(activity.time)
      : 'Just now'
    : formatRelative(activity.time)
  const conf = pending
    ? 'in mempool'
    : `${formatNumber(activity.confirmations)} ${
        activity.confirmations === 1 ? 'confirmation' : 'confirmations'
      }`

  return (
    <div className="bp-activity-row">
      <span className={`bp-activity-icon bp-activity-icon--${activity.direction}`}>
        {meta.icon}
      </span>
      <div className="bp-activity-main">
        <div className="bp-activity-label">{meta.label}</div>
        <div className="bp-activity-when">
          {when} · {conf}
        </div>
      </div>
      <div className="bp-activity-right">
        <div
          className={`bp-activity-amount bp-activity-amount--${activity.direction}`}
        >
          {meta.sign}
          {formatBtc(activity.amountSats)} BTC
        </div>
        <span
          className={`bp-activity-badge ${
            pending ? 'bp-activity-badge--pending' : 'bp-activity-badge--confirmed'
          }`}
        >
          {pending ? 'Pending' : 'Confirmed'}
        </span>
      </div>
    </div>
  )
}
