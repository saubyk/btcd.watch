import { useState, type FormEvent } from 'react'

import type { Stats } from '../api/types'
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
