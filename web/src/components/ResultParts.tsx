import type { ReactNode } from 'react'

import type { Tx } from '../api/types'
import { formatBtc, formatFiat, truncateMiddle } from '../lib/format'

export function BackButton({ onClick }: { onClick: () => void }) {
  return (
    <button className="bp-back-btn" onClick={onClick}>
      ← New search
    </button>
  )
}

/** Big BTC amount + fiat + from→to line. */
export function AmountHeader({ tx }: { tx: Tx }) {
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
          <span>{formatAddrList(tx.from)}</span>
        )}
        <span className="bp-fromto-arrow">→</span>
        <span>{formatAddrList(tx.to)}</span>
      </div>
    </>
  )
}

function formatAddrList(addrs: string[]): string {
  if (addrs.length === 0) return '—'
  const first = truncateMiddle(addrs[0]!)
  return addrs.length > 1 ? `${first} +${addrs.length - 1}` : first
}

export function StatTile({
  label,
  children,
  mono = true,
}: {
  label: string
  children: ReactNode
  mono?: boolean
}) {
  return (
    <div className="bp-tile-item">
      <div className="bp-tile-label">{label}</div>
      <div className={`bp-tile-value${mono ? ' bp-tile-value--mono' : ''}`}>
        {children}
      </div>
    </div>
  )
}
