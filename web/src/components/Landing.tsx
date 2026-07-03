import { useState, type FormEvent } from 'react'

import type { Examples, FeeEstimate, Stats } from '../api/types'
import { appConfig } from '../appConfig'
import {
  formatCompact,
  formatEta,
  formatEtaShort,
  formatFiat,
  formatNumber,
} from '../lib/format'
import { SearchIcon } from './Icons'

/* ===== Live status pill ===== */

export function LiveStatusPill({
  stats,
  connected,
}: {
  stats: Stats | null
  connected: boolean
}) {
  if (!connected || !stats) {
    return (
      <div className="bp-live-pill">
        <span className="bp-live-dot bp-live-dot--off" />
        Connecting to your node…
      </div>
    )
  }
  return (
    <div className="bp-live-pill">
      <span className="bp-live-dot" />
      Network live · next block in {formatEtaShort(stats.nextBlockEtaSeconds)}
    </div>
  )
}

/* ===== Search bar ===== */

export function SearchBar({ onSearch }: { onSearch: (q: string) => void }) {
  const [value, setValue] = useState('')

  const submit = (e: FormEvent) => {
    e.preventDefault()
    onSearch(value)
  }

  return (
    <form className="bp-searchbar" onSubmit={submit}>
      <div className="bp-search-field">
        <SearchIcon />
        <input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder="Transaction ID or address…"
          aria-label="Transaction ID or address"
          spellCheck={false}
        />
      </div>
      <button type="submit" className="bp-btn-primary">
        Track it
      </button>
    </form>
  )
}

/* ===== Example chips (real data; null hides a chip) ===== */

export function ExampleChips({
  examples,
  onSearch,
}: {
  examples: Examples | null
  onSearch: (q: string) => void
}) {
  const chips = [
    { label: '✓ Confirmed', query: examples?.confirmedTxid },
    { label: '⏳ Pending', query: examples?.pendingTxid },
    { label: '◆ Wallet address', query: examples?.address },
  ].filter((c): c is { label: string; query: string } => !!c.query)

  if (chips.length === 0) return null

  return (
    <div className="bp-chips">
      <span className="bp-chips-label">Try an example:</span>
      {chips.map((c) => (
        <button key={c.label} className="bp-chip" onClick={() => onSearch(c.query)}>
          {c.label}
        </button>
      ))}
    </div>
  )
}

/* ===== Fee estimator ===== */

const SIZE_PRESETS = [
  { label: 'Simple payment', sub: '1 in · 2 out', vbytes: 140 },
  { label: 'With change', sub: '2 in · 2 out', vbytes: 220 },
  { label: 'Consolidation', sub: '5 in · 1 out', vbytes: 400 },
]

const TIER_META: Record<string, { name: string; badge: string }> = {
  slow: { name: '🐢 Slow', badge: 'Cheapest' },
  standard: { name: '🚶 Standard', badge: 'Most popular' },
  urgent: { name: '⚡ Urgent', badge: 'Next block' },
}

export function FeeEstimator({ fees }: { fees: FeeEstimate | null }) {
  const [vbytes, setVbytes] = useState(140)
  if (!fees) return null

  return (
    <section className="bp-section">
      <div className="bp-section-head">
        <div>
          <h2>What fee should I pay?</h2>
          <p>
            Higher fees confirm faster. Pick the speed that fits — prices
            shown are for a typical payment.
          </p>
        </div>
        <span className="bp-updated">
          {fees.source === 'mempool'
            ? 'Live from your node'
            : 'Quiet mempool — minimum rates'}
        </span>
      </div>

      <div>
        <div className="bp-size-label">
          How big is your payment?{' '}
          <span>— this changes the total cost</span>
        </div>
        <div className="bp-size-row">
          {SIZE_PRESETS.map((p) => (
            <button
              key={p.vbytes}
              className={`bp-size-btn${
                vbytes === p.vbytes ? ' bp-size-btn--active' : ''
              }`}
              onClick={() => setVbytes(p.vbytes)}
            >
              <div className="bp-size-btn-title">{p.label}</div>
              <div className="bp-size-btn-sub">
                {p.sub} · {p.vbytes} vB
              </div>
            </button>
          ))}
        </div>
      </div>

      <div className="bp-fee-grid">
        {fees.tiers.map((tier) => {
          const meta = TIER_META[tier.id] ?? { name: tier.id, badge: '' }
          const recommended = tier.id === 'standard'
          return (
            <div
              key={tier.id}
              className={`bp-fee-card${
                recommended ? ' bp-fee-card--recommended' : ''
              }`}
            >
              {recommended && <span className="bp-fee-ribbon">RECOMMENDED</span>}
              <div className="bp-fee-card-head">
                <span className="bp-fee-card-name">{meta.name}</span>
                <span
                  className={`bp-fee-badge${
                    recommended ? ' bp-fee-badge--orange' : ''
                  }`}
                >
                  {meta.badge}
                </span>
              </div>
              <div className="bp-fee-rate">
                <span
                  className={`bp-fee-rate-num${
                    recommended ? ' bp-fee-rate-num--orange' : ''
                  }`}
                >
                  {Math.round(tier.satPerVb)}
                </span>
                <span className="bp-fee-rate-unit">sat/vB</span>
              </div>
              <div className="bp-fee-divider" />
              <div className="bp-fee-row">
                <span className="bp-fee-row-label">Confirms in</span>
                <span className="bp-fee-row-value">{tier.label}</span>
              </div>
              <div className="bp-fee-row">
                <span className="bp-fee-row-label">Typical cost</span>
                <span className="bp-fee-row-value bp-mono">
                  ~{formatNumber(Math.round(tier.satPerVb * vbytes))} sats
                </span>
              </div>
            </div>
          )
        })}
      </div>
      <p className="bp-fee-footnote">
        <strong>sat/vB</strong> means satoshis per virtual byte — the price
        you pay per unit of transaction size. Rates rise when the network is
        busy.
      </p>
    </section>
  )
}

/* ===== Dark stats bar ===== */

export function StatsBar({ stats }: { stats: Stats | null }) {
  if (!stats) return null
  return (
    <section className="bp-stats-section">
      <div className="bp-stats-bar">
        <div>
          <div className="bp-stat-label">Mempool size</div>
          <div className="bp-stat-value bp-stat-value--mono">
            {formatCompact(stats.mempool.txCount)}{' '}
            <span className="bp-stat-unit">tx</span>
          </div>
        </div>
        <div>
          <div className="bp-stat-label">Block height</div>
          <div className="bp-stat-value bp-stat-value--mono">
            {formatNumber(stats.blockHeight)}
          </div>
        </div>
        <div>
          <div className="bp-stat-label">Next halving</div>
          <div className="bp-stat-value">
            in {formatEta(stats.halving.etaSeconds).replace('~', '~')}
          </div>
        </div>
        <div>
          <div className="bp-stat-label">1 BTC ≈</div>
          <div className="bp-stat-value bp-stat-value--mono bp-stat-value--orange">
            {stats.price ? formatFiat(stats.price.usd) : '—'}
          </div>
        </div>
      </div>
    </section>
  )
}

/* ===== Node CTA ===== */

const BENEFITS = [
  {
    emoji: '🔒',
    title: 'Private by default',
    sub: 'Your lookups never leave your machine.',
  },
  {
    emoji: '⚖️',
    title: 'No middleman',
    sub: 'You see the truth straight from the network.',
  },
  {
    emoji: '🌐',
    title: 'Strengthens Bitcoin',
    sub: 'Every node makes the network more resilient.',
  },
]

export function NodeCta() {
  return (
    <section className="bp-cta-section">
      <div className="bp-cta-card">
        <div>
          <div className="bp-cta-pill">POWERED BY YOUR OWN NODE</div>
          <h2>Don't trust. Verify.</h2>
          <p>
            Every answer here comes from a <strong>btcd</strong> node —
            open-source Bitcoin software anyone can run. Run your own and you
            check the blockchain yourself, instead of trusting someone else's
            word.
          </p>
          <a
            className="bp-cta-btn"
            href={appConfig.btcdUrl}
            target="_blank"
            rel="noreferrer"
          >
            Run your own btcd node →
          </a>
        </div>
        <div className="bp-benefits">
          {BENEFITS.map((b) => (
            <div key={b.title} className="bp-benefit">
              <span className="bp-benefit-emoji">{b.emoji}</span>
              <div>
                <div className="bp-benefit-title">{b.title}</div>
                <div className="bp-benefit-sub">{b.sub}</div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
