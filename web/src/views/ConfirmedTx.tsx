import type { Tx } from '../api/types'
import { ConfirmationProgress } from '../components/ConfirmationProgress'
import { CheckIcon, CopyIcon } from '../components/Icons'
import { AmountHeader, BackButton, StatTile } from '../components/ResultParts'
import { useCopy } from '../components/Toast'
import {
  formatNumber,
  formatRelative,
  formatTimestamp,
  truncateMiddle,
} from '../lib/format'

export function ConfirmedTx({
  tx,
  detail,
  justConfirmed,
  onSetDetail,
  onHome,
}: {
  tx: Tx
  detail: 'beginner' | 'detailed'
  justConfirmed: boolean
  onSetDetail: (d: 'beginner' | 'detailed') => void
  onHome: () => void
}) {
  const copy = useCopy()

  return (
    <main className="bp-view bp-result">
      <BackButton onClick={onHome} />

      {justConfirmed && tx.block && (
        <div className="bp-celebrate">
          <span className="bp-celebrate-emoji">🎉</span>
          <div>
            <div className="bp-celebrate-title">
              It just confirmed while you watched!
            </div>
            <div className="bp-celebrate-sub">
              Landed in block {formatNumber(tx.block.height)} a moment ago —
              your payment is now on the blockchain.
            </div>
          </div>
        </div>
      )}

      <div className="bp-card bp-card--confirmed">
        <div className="bp-card-head bp-card-head--confirmed">
          <span className="bp-status-badge bp-status-badge--confirmed">
            <CheckIcon />
          </span>
          <div>
            <div className="bp-status-title bp-status-title--confirmed">
              Confirmed
            </div>
            <div className="bp-status-sub bp-status-sub--confirmed">
              This payment is complete and permanent.
            </div>
          </div>
        </div>

        <div className="bp-card-body">
          <AmountHeader tx={tx} />

          <div className="bp-tiles">
            <StatTile label="Confirmations">
              {formatNumber(tx.confirmations)}{' '}
              {tx.confirmations >= 6 && (
                <span className="bp-safe-pill">very safe</span>
              )}
            </StatTile>
            {tx.block && (
              <StatTile label="In block">
                {formatNumber(tx.block.height)}
              </StatTile>
            )}
            {tx.block && (
              <StatTile label="When" mono={false}>
                {formatRelative(tx.block.time)}
              </StatTile>
            )}
          </div>

          <ConfirmationProgress confirmations={tx.confirmations} />

          <div className="bp-toggle" role="tablist">
            <button
              className={detail === 'beginner' ? 'bp-toggle--on' : ''}
              onClick={() => onSetDetail('beginner')}
            >
              Simple
            </button>
            <button
              className={detail === 'detailed' ? 'bp-toggle--on' : ''}
              onClick={() => onSetDetail('detailed')}
            >
              Detailed
            </button>
          </div>

          {detail === 'beginner' ? (
            <div className="bp-explain">
              <div className="bp-explain-title">What does this mean? 💡</div>
              <p>
                Your transaction was included in the Bitcoin ledger{' '}
                <strong>
                  {formatNumber(tx.confirmations)}{' '}
                  {tx.confirmations === 1 ? 'block' : 'blocks'} ago
                </strong>
                . Each new block stacked on top makes it harder to ever
                reverse. Anything above 6 confirmations is considered final —
                so this money has truly, permanently moved.
              </p>
            </div>
          ) : (
            <DetailTable tx={tx} copy={copy} />
          )}
        </div>
      </div>
    </main>
  )
}

function DetailTable({ tx, copy }: { tx: Tx; copy: (t: string) => void }) {
  return (
    <div className="bp-detail-table">
      <DetailRow label="Transaction ID" copyValue={tx.txid} onCopy={copy}>
        {truncateMiddle(tx.txid, 8, 7)}
      </DetailRow>
      {tx.block && (
        <DetailRow label="Block hash" copyValue={tx.block.hash} onCopy={copy}>
          {truncateMiddle(tx.block.hash, 8, 4)}
        </DetailRow>
      )}
      <DetailRow label="Fee paid">
        {tx.feeSats == null
          ? 'none — newly minted'
          : `${formatNumber(tx.feeSats)} sats · ${round1(
              tx.feeRateSatPerVb,
            )} sat/vB`}
      </DetailRow>
      <DetailRow label="Size">{formatNumber(tx.vsize)} vB</DetailRow>
      {tx.block && (
        <DetailRow label="Timestamp" sans>
          {formatTimestamp(tx.block.time)}
        </DetailRow>
      )}
    </div>
  )
}

function round1(n: number | null): string {
  return n == null ? '?' : String(Math.round(n * 10) / 10)
}

function DetailRow({
  label,
  children,
  copyValue,
  onCopy,
  sans = false,
}: {
  label: string
  children: React.ReactNode
  copyValue?: string
  onCopy?: (t: string) => void
  sans?: boolean
}) {
  return (
    <div className="bp-detail-row">
      <span className="bp-detail-key">{label}</span>
      <span className="bp-detail-value">
        <span
          className={`bp-detail-value-text${
            sans ? ' bp-detail-value-text--sans' : ''
          }`}
        >
          {children}
        </span>
        {copyValue && onCopy && (
          <button
            className="bp-copy-icon-btn"
            title="Copy"
            onClick={() => onCopy(copyValue)}
          >
            <CopyIcon />
          </button>
        )}
      </span>
    </div>
  )
}
