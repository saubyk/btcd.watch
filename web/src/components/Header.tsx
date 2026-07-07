import type { FeeEstimate } from '../api/types'
import { ChevronDownIcon } from './Icons'

/**
 * Sticky header: logo plus the fee ticker — the Standard rate at a glance,
 * one tap from the full fee helper on every view.
 */
export function Header({
  fees,
  onHome,
  onOpenFees,
}: {
  fees: FeeEstimate | null
  onHome: () => void
  onOpenFees: () => void
}) {
  const standard = fees?.tiers.find((t) => t.id === 'standard')

  return (
    <header className="bp-header">
      <div className="bp-header-inner">
        <button className="bp-logo-btn" onClick={onHome} aria-label="Home">
          <span className="bp-logo-mark">₿</span>
          <span className="bp-wordmark">
            btcdwatch<span className="bp-wordmark-tld">.com</span>
          </span>
        </button>
        <button
          className="bp-fee-ticker"
          onClick={onOpenFees}
          title="What fee should I pay?"
        >
          <span className="bp-live-dot" />
          Fees:{' '}
          <span className="bp-fee-ticker-rate">
            {standard ? `${Math.round(standard.satPerVb)} sat/vB` : '—'}
          </span>
          <ChevronDownIcon />
        </button>
      </div>
    </header>
  )
}
