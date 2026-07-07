import { useEffect, useRef, useState } from 'react'

import type { FeeEstimate, FeeTier } from '../api/types'
import { formatNumber } from '../lib/format'

const SIZE_PRESETS = [
  { label: 'Simple', vbytes: 140 },
  { label: 'With change', vbytes: 220 },
  { label: 'Consolidation', vbytes: 400 },
]

const TIER_META: Record<string, { emoji: string; name: string; badge: string }> = {
  slow: { emoji: '🐢', name: 'Slow', badge: 'Cheapest' },
  standard: { emoji: '🚶', name: 'Standard', badge: 'RECOMMENDED' },
  urgent: { emoji: '⚡', name: 'Urgent', badge: 'Next block' },
}

/**
 * The fee helper as a right-hand slide-over, opened from the header
 * ticker. Standard dialog semantics: backdrop/✕/Escape close it, and Tab
 * cycles inside the panel while it is open.
 */
export function FeeSlideOver({
  fees,
  open,
  onClose,
}: {
  fees: FeeEstimate | null
  open: boolean
  onClose: () => void
}) {
  const [vbytes, setVbytes] = useState(140)
  const panel = useRef<HTMLDivElement>(null)

  // Initial focus only on open — not on later re-renders (App re-renders
  // on every stats push while the panel is up).
  useEffect(() => {
    if (open) panel.current?.querySelector('button')?.focus()
  }, [open])

  useEffect(() => {
    if (!open) return

    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
        return
      }
      if (e.key !== 'Tab' || !panel.current) return
      const focusables = panel.current.querySelectorAll<HTMLElement>(
        'button, [href], input, [tabindex]:not([tabindex="-1"])',
      )
      if (focusables.length === 0) return
      const first = focusables[0]!
      const last = focusables[focusables.length - 1]!
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null

  return (
    <>
      <div className="bp-fee-overlay" onClick={onClose} />
      <div
        className="bp-fee-panel"
        role="dialog"
        aria-modal="true"
        aria-label="What fee should I pay?"
        ref={panel}
      >
        <div className="bp-fee-panel-head">
          <div className="bp-fee-panel-title">What fee should I pay?</div>
          <button className="bp-fee-close" onClick={onClose} aria-label="Close">
            ✕
          </button>
        </div>
        <div className="bp-fee-panel-live">
          <span className="bp-live-dot bp-live-dot--sm" />
          {fees?.source === 'mempool'
            ? 'Live rates · updated just now'
            : 'Quiet mempool — minimum rates'}
        </div>

        <div className="bp-fee-panel-size-label">
          How big is your payment?{' '}
          <span>— changes the total cost</span>
        </div>
        <div className="bp-seg">
          {SIZE_PRESETS.map((p) => (
            <button
              key={p.vbytes}
              className={vbytes === p.vbytes ? 'bp-seg--on' : ''}
              onClick={() => setVbytes(p.vbytes)}
            >
              {p.label}
            </button>
          ))}
        </div>

        <div className="bp-fee-tiers">
          {fees?.tiers.map((tier) => (
            <TierRow key={tier.id} tier={tier} vbytes={vbytes} />
          ))}
        </div>

        <p className="bp-fee-footnote">
          <strong>sat/vB</strong> means satoshis per virtual byte — the price
          you pay per unit of transaction size. Rates rise when the network
          is busy.
        </p>
      </div>
    </>
  )
}

function TierRow({ tier, vbytes }: { tier: FeeTier; vbytes: number }) {
  const meta = TIER_META[tier.id] ?? { emoji: '', name: tier.id, badge: '' }
  const recommended = tier.id === 'standard'

  return (
    <div
      className={`bp-fee-tier${recommended ? ' bp-fee-tier--recommended' : ''}`}
    >
      <span className="bp-fee-tier-emoji">{meta.emoji}</span>
      <div className="bp-fee-tier-main">
        <div className="bp-fee-tier-name">
          {meta.name}{' '}
          <span
            className={`bp-fee-badge${recommended ? ' bp-fee-badge--orange' : ''}`}
          >
            {meta.badge}
          </span>
        </div>
        <div className="bp-fee-tier-eta">Confirms in {tier.label}</div>
      </div>
      <div className="bp-fee-tier-right">
        <div
          className={`bp-fee-tier-rate${
            recommended ? ' bp-fee-tier-rate--orange' : ''
          }`}
        >
          {Math.round(tier.satPerVb)}{' '}
          <span className="bp-fee-tier-unit">sat/vB</span>
        </div>
        <div className="bp-fee-tier-cost">
          ~{formatNumber(Math.round(tier.satPerVb * vbytes))} sats
        </div>
      </div>
    </div>
  )
}
