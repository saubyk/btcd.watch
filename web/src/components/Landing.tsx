import { useRef, useState, type FormEvent } from 'react'

import type { Stats } from '../api/types'
import { appConfig } from '../appConfig'
import { useCountdown } from '../hooks/useCountdown'
import { useMotionMode } from '../hooks/useMotion'
import {
  formatCompact,
  formatEta,
  formatEtaShort,
  formatFiat,
  formatNumber,
} from '../lib/format'
import { SearchIcon } from './Icons'
import { TweenedCount } from './TweenedCount'

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
  if (stats.syncing) {
    return (
      <div className="bp-live-pill">
        <span className="bp-live-dot bp-live-dot--off bp-pulse-slow" />
        Node is syncing the blockchain — at block{' '}
        {formatNumber(stats.blockHeight)} so far
      </div>
    )
  }
  return <LivePill stats={stats} />
}

/** The live branch of the status pill: the ETA counts down locally
 * between pushes and the dot pulses (round-7 heartbeat). */
function LivePill({ stats }: { stats: Stats }) {
  const motionOn = useMotionMode() !== 'off'
  const eta = useCountdown(stats.nextBlockEtaSeconds, stats, motionOn)
  return (
    <div className="bp-live-pill">
      <span className={`bp-live-dot${motionOn ? ' bp-pulse-slow' : ''}`} />
      Network live · next block in {formatEtaShort(eta)}
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

export function StatsBar({
  stats,
  onSearch,
}: {
  stats: Stats | null
  onSearch: (q: string) => void
}) {
  if (!stats) return null
  return <StatsBarInner stats={stats} onSearch={onSearch} />
}

function StatsBarInner({
  stats,
  onSearch,
}: {
  stats: Stats
  onSearch: (q: string) => void
}) {
  const motionOn = useMotionMode() !== 'off'
  // The height pops only on a live change, never on the first paint.
  const firstHeight = useRef(stats.blockHeight)
  const ticked = motionOn && stats.blockHeight !== firstHeight.current
  return (
    <section className="bp-stats-section">
      <div className="bp-stats-bar">
        <div>
          <div className="bp-stat-label">Mempool size</div>
          <div className="bp-stat-value bp-stat-value--mono">
            <TweenedCount value={stats.mempool.txCount} format={formatCompact} />{' '}
            <span className="bp-stat-unit">tx</span>
          </div>
        </div>
        {/* Round 7: the height tile is the door into the chain. */}
        <button
          className="bp-stat-tile-btn"
          onClick={() => onSearch(String(stats.blockHeight))}
          title="Open the latest block and browse the chain"
        >
          <div className="bp-stat-label">
            Block height <span className="bp-stat-hint">· view →</span>
          </div>
          <div className="bp-stat-value bp-stat-value--mono">
            <span
              key={`h${stats.blockHeight}`}
              className={ticked ? 'bp-tickup' : undefined}
            >
              {formatNumber(stats.blockHeight)}
            </span>
          </div>
        </button>
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
