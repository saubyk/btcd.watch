import { useRef, useState, type Dispatch } from 'react'

import { api } from '../api/client'
import type { Block, BlockTx, Stats } from '../api/types'
import { appConfig } from '../appConfig'
import { CopyIcon } from '../components/Icons'
import { BackButton, StatTile } from '../components/ResultParts'
import { useCopy } from '../components/Toast'
import { useLoadMore } from '../hooks/useLoadMore'
import { useMotionMode } from '../hooks/useMotion'
import { useNow } from '../hooks/useNow'
import {
  formatBtc,
  formatMinedAgo,
  formatNumber,
  formatRelative,
  formatTimestamp,
  truncateMiddle,
} from '../lib/format'
import type { Action } from '../state'

/** "· the newest block" → "settling in" → "permanent". Round 8 dropped
 * "— just mined": the mined line right below it counts up. */
function depthLabel(depth: number): string {
  if (depth === 0) return '· the newest block'
  if (depth < 6) {
    return `· ${depth} ${depth === 1 ? 'block' : 'blocks'} deep — settling in`
  }
  return `· ${formatNumber(depth)} blocks deep — permanent`
}

export function BlockView({
  block,
  stats,
  dispatch,
  onSearch,
  onHome,
}: {
  block: Block
  stats: Stats | null
  dispatch: Dispatch<Action>
  onSearch: (q: string) => void
  onHome: () => void
}) {
  const copy = useCopy()
  const now = useNow(appConfig.minedAgoRefreshSeconds * 1000)

  // Live depth: prefer the pushed tip so the view flips from "newest"
  // to "1 block deep" (and grows a next-button) the moment a block is
  // mined, without refetching. Confirmations are the fallback.
  const depth = stats
    ? Math.max(0, stats.blockHeight - block.height)
    : Math.max(0, block.confirmations - 1)
  const atTip = depth === 0

  // Round 7: prev/next swap the block in place — a direct fetch into
  // the reducer, skipping the search flow's min-400ms loading view. A
  // failed fetch falls back to the search path for the real error view.
  const navSeq = useRef(0)
  const [navigating, setNavigating] = useState(false)
  const navigate = async (height: number) => {
    const mySeq = ++navSeq.current
    setNavigating(true)
    try {
      const next = await api.block(height)
      if (navSeq.current !== mySeq) return
      dispatch({ type: 'search-result', result: { kind: 'block', block: next } })
      history.replaceState(null, '', `?q=${height}`)
    } catch {
      if (navSeq.current === mySeq) onSearch(String(height))
      return
    } finally {
      if (navSeq.current === mySeq) setNavigating(false)
    }
  }

  // Paginate by hash (unambiguous even for stale blocks); the merged list
  // always starts at offset 0, so its length is the next offset.
  const { loadMore, loading: loadingMore } = useLoadMore(async () => {
    const page = await api.block(block.hash, block.txs.length, block.limit)
    dispatch({ type: 'block-more', page })
  })

  const remaining = block.txCount - block.txs.length

  return (
    <main className="bp-view bp-result">
      <BackButton onClick={onHome} />

      <div className="bp-card">
        <div className="bp-card-body">
          <div className="bp-address-label">▦ Block</div>
          <div className="bp-amount-row">
            <span className="bp-amount bp-amount--lg">
              {formatNumber(block.height)}
            </span>
            <span className="bp-amount-fiat">{depthLabel(depth)}</span>
          </div>
          <div className="bp-balance-caption">
            {/* At the tip the age is the headline number, so it uses the
                minute-resolution copy the hero pill shares. */}
            Mined{' '}
            {atTip ? formatMinedAgo(block.time, now) : formatRelative(block.time)}{' '}
            · {formatTimestamp(block.time)}
          </div>

          <div className="bp-address-row bp-block-hash-row">
            <div className="bp-address-value bp-block-hash">{block.hash}</div>
            <button className="bp-copy-btn" onClick={() => copy(block.hash)}>
              <CopyIcon />
              Copy
            </button>
          </div>

          <div className="bp-tiles" style={{ margin: '20px 0 0' }}>
            <StatTile label="Transactions">
              {formatNumber(block.txCount)}
            </StatTile>
            <StatTile label="Average fee">
              {Math.round(block.avgFeeSatPerVb)}{' '}
              <span className="bp-tile-unit">sat/vB</span>
            </StatTile>
            <StatTile label="Size">
              {(block.sizeBytes / 1e6).toFixed(2)}{' '}
              <span className="bp-tile-unit">MB</span>
            </StatTile>
          </div>

          <div className="bp-block-nav">
            {block.height > 0 ? (
              <button
                className="bp-block-nav-btn"
                onClick={() => navigate(block.height - 1)}
                disabled={navigating}
              >
                ← Block {formatNumber(block.height - 1)}
              </button>
            ) : (
              <span />
            )}
            {atTip
              ? <TipPill time={block.time} now={now} />
              : (
                  <button
                    className="bp-block-nav-btn"
                    onClick={() =>
                      navigate(block.nextHeight ?? block.height + 1)
                    }
                    disabled={navigating}
                  >
                    Block {formatNumber(block.nextHeight ?? block.height + 1)}{' '}
                    →
                  </button>
                )}
          </div>
        </div>

        <div className="bp-activity">
          <div className="bp-activity-head">Transactions in this block</div>
          {block.txs.map((tx) => (
            <BlockTxRow key={tx.txid} tx={tx} onSearch={onSearch} />
          ))}
          {block.hasMore && (
            <button
              className="bp-blocktx-more"
              onClick={loadMore}
              disabled={loadingMore}
            >
              {loadingMore
                ? 'Loading…'
                : `…and ${formatNumber(remaining)} more ${
                    remaining === 1 ? 'transaction' : 'transactions'
                  } — show more`}
            </button>
          )}
        </div>
      </div>
    </main>
  )
}

/** At the tip the next-button's place holds a live pill sharing the
 * hero pill's copy: how long ago this block was mined. */
function TipPill({ time, now }: { time: number; now: number }) {
  const motionOn = useMotionMode() !== 'off'
  return (
    <span className="bp-block-tip-pill">
      <span
        className={`bp-block-tip-dot${motionOn ? ' bp-pulse-slow' : ''}`}
      />
      Newest block — mined {formatMinedAgo(time, now)}
    </span>
  )
}

function BlockTxRow({
  tx,
  onSearch,
}: {
  tx: BlockTx
  onSearch: (q: string) => void
}) {
  return (
    <button
      className="bp-blocktx-row"
      onClick={() => onSearch(tx.txid)}
      title="View this transaction"
    >
      <div className="bp-blocktx-main">
        <div className="bp-blocktx-id-row">
          <span className="bp-blocktx-id">{truncateMiddle(tx.txid, 8, 7)}</span>
          {tx.isCoinbase && <span className="bp-cb-badge">miner reward</span>}
        </div>
        <div className="bp-blocktx-meta">
          {tx.isCoinbase
            ? 'New coins + all fees from this block'
            : `Paid ${
                tx.feeRateSatPerVb == null
                  ? '?'
                  : Math.round(tx.feeRateSatPerVb * 10) / 10
              } sat/vB`}
        </div>
      </div>
      <span className="bp-blocktx-amount">{formatBtc(tx.amountSats)} BTC</span>
    </button>
  )
}
