import { useSyncExternalStore, type ReactNode } from 'react'

import type { Tx } from '../api/types'
import { formatBtc, formatFiat, truncateMiddle } from '../lib/format'
import { backLabel, currentEntry, subscribeEntry } from '../lib/nav'
import { AddressLink } from './AddressLink'
import { TypeChips } from './TypeChips'

/**
 * "← Back to <previous result>" when the user drilled in from another
 * result (same as the browser's Back), plus "New search" to the landing
 * page. A cold-loaded deep link has nothing in-app behind it, so only
 * "New search" shows.
 */
export function BackButton({ onClick }: { onClick: () => void }) {
  const entry = useSyncExternalStore(subscribeEntry, currentEntry)
  const back = backLabel(entry)
  if (!back) {
    return (
      <button className="bp-back-btn" onClick={onClick}>
        ← New search
      </button>
    )
  }
  return (
    <div className="bp-back-row">
      <button className="bp-back-btn" onClick={() => history.back()}>
        ← {back}
      </button>
      <button className="bp-back-btn bp-back-btn--secondary" onClick={onClick}>
        New search
      </button>
    </div>
  )
}

/** Big BTC amount + fiat + from→to line + script-type chips. Addresses
 * link into the Address view. */
export function AmountHeader({
  tx,
  onSearch,
}: {
  tx: Tx
  onSearch: (q: string) => void
}) {
  return (
    <>
      <div className="bp-amount-row">
        <span className="bp-amount">
          {formatBtc(tx.amountSats)} <span className="bp-amount-unit">BTC</span>
        </span>
        {tx.fiatUsd != null && (
          <span className="bp-amount-fiat">≈ {formatFiat(tx.fiatUsd)}</span>
        )}
      </div>
      <div className="bp-fromto">
        {tx.isCoinbase ? (
          <span>newly minted</span>
        ) : (
          <AddrListLink addrs={tx.from} onSearch={onSearch} />
        )}
        <span className="bp-fromto-arrow">→</span>
        <AddrListLink addrs={tx.to} onSearch={onSearch} />
      </div>
      {(tx.type || (tx.rbf && tx.status === 'pending')) && (
        <div className="bp-type-chips">
          {tx.type && <TypeChips code={tx.type.code} suffix=" transaction" />}
          {tx.rbf && tx.status === 'pending' && (
            <span
              className="bp-type-chip bp-type-chip--rbf"
              title="The sender can rebroadcast this transaction with a higher fee to speed it up"
            >
              RBF — fee can be bumped
            </span>
          )}
        </div>
      )}
    </>
  )
}

/** First address as a truncated link, "+N" plain for the rest. */
function AddrListLink({
  addrs,
  onSearch,
}: {
  addrs: string[]
  onSearch: (q: string) => void
}) {
  if (addrs.length === 0) return <span>—</span>
  return (
    <span>
      <AddressLink
        address={addrs[0]!}
        display={truncateMiddle(addrs[0]!)}
        onSearch={onSearch}
      />
      {addrs.length > 1 && ` +${addrs.length - 1}`}
    </span>
  )
}

export function StatTile({
  label,
  children,
  mono = true,
  onClick,
  title,
}: {
  label: string
  children: ReactNode
  mono?: boolean
  onClick?: () => void
  title?: string
}) {
  if (onClick) {
    return (
      <button className="bp-tile-item bp-tile-item--link" onClick={onClick} title={title}>
        <div className="bp-tile-label">{label}</div>
        <div
          className={`bp-tile-value bp-tile-link-value${
            mono ? ' bp-tile-value--mono' : ''
          }`}
        >
          {children} →
        </div>
      </button>
    )
  }
  return (
    <div className="bp-tile-item">
      <div className="bp-tile-label">{label}</div>
      <div className={`bp-tile-value${mono ? ' bp-tile-value--mono' : ''}`}>
        {children}
      </div>
    </div>
  )
}
