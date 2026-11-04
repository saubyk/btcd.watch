import type { SpendState, Tx, TxIO } from '../api/types'
import { formatBtc, formatNumber } from '../lib/format'
import { AddressLink } from './AddressLink'

/**
 * The Detailed tab's inputs/outputs breakdown: full tappable addresses,
 * per-row amounts, recipient/change role chips, spent/unspent chips on
 * outputs, and the plain-English fee footer.
 */
export function TxIOCard({
  tx,
  onSearch,
}: {
  tx: Tx
  onSearch: (q: string) => void
}) {
  return (
    <div className="bp-io-card">
      <div className="bp-io-head">
        Inputs — where the money came from{' '}
        <span className="bp-io-count">
          ({tx.isCoinbase ? 0 : tx.inputs.length})
        </span>
      </div>
      {tx.isCoinbase ? (
        <div className="bp-io-row">
          <span className="bp-io-plain">
            newly minted coins — this is the block's coinbase
          </span>
        </div>
      ) : (
        tx.inputs.map((io, i) => (
          <IORow key={i} io={io} onSearch={onSearch} />
        ))
      )}

      <div className="bp-io-head">
        Outputs — where it went{' '}
        <span className="bp-io-count">({tx.outputs.length})</span>
      </div>
      {tx.outputs.map((io, i) => (
        <IORow key={i} io={io} role onSearch={onSearch} />
      ))}

      {tx.feeSats != null && (
        <div className="bp-io-foot">
          Inputs − outputs = <strong>{formatBtc(tx.feeSats)} BTC</strong> (
          {formatNumber(tx.feeSats)} sats) — the fee, kept by the miner. Tap
          any address to view it.
        </div>
      )}
    </div>
  )
}

function IORow({
  io,
  role = false,
  onSearch,
}: {
  io: TxIO
  role?: boolean
  onSearch: (q: string) => void
}) {
  return (
    <div className="bp-io-row">
      <div className="bp-io-main">
        <AddressLink address={io.address} breakAll onSearch={onSearch} />
        {role && (
          <span
            className={`bp-io-role${io.change ? ' bp-io-role--change' : ''}`}
          >
            {io.change ? 'change — back to sender' : 'recipient'}
          </span>
        )}
        {io.spend && <SpendChip spend={io.spend} />}
      </div>
      <span className="bp-io-amount">{formatBtc(io.amountSats)} BTC</span>
    </div>
  )
}

const SPEND_COPY: Record<SpendState, { label: string; title: string }> = {
  unspent: {
    label: 'unspent',
    title: 'Still sitting at this address — not sent on yet.',
  },
  spent: {
    label: 'spent',
    title:
      'Already sent on in a later transaction — tap the address to follow it.',
  },
  unspendable: {
    label: 'data only — can’t be spent',
    title: 'An OP_RETURN output carries data, not money, and can never be spent.',
  },
}

function SpendChip({ spend }: { spend: SpendState }) {
  const copy = SPEND_COPY[spend]
  return (
    <span className={`bp-io-spend bp-io-spend--${spend}`} title={copy.title}>
      {copy.label}
    </span>
  )
}
