import type { Dispatch } from 'react'

import { api } from '../api/client'
import type { Block, BlockTx } from '../api/types'
import { CopyIcon } from '../components/Icons'
import { BackButton, StatTile } from '../components/ResultParts'
import { useCopy } from '../components/Toast'
import { useLoadMore } from '../hooks/useLoadMore'
import {
  formatBtc,
  formatNumber,
  formatRelative,
  formatTimestamp,
  truncateMiddle,
} from '../lib/format'
import type { Action } from '../state'

export function BlockView({
  block,
  dispatch,
  onSearch,
  onHome,
}: {
  block: Block
  dispatch: Dispatch<Action>
  onSearch: (q: string) => void
  onHome: () => void
}) {
  const copy = useCopy()

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
            <span className="bp-amount-fiat">
              · {formatNumber(block.confirmations)}{' '}
              {block.confirmations === 1 ? 'block' : 'blocks'} deep — permanent
            </span>
          </div>
          <div className="bp-balance-caption">
            Mined {formatRelative(block.time)} · {formatTimestamp(block.time)}
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
                onClick={() => onSearch(String(block.height - 1))}
              >
                ← Block {formatNumber(block.height - 1)}
              </button>
            ) : (
              <span />
            )}
            {block.nextHeight != null && (
              <button
                className="bp-block-nav-btn"
                onClick={() => onSearch(String(block.nextHeight))}
              >
                Block {formatNumber(block.nextHeight)} →
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
