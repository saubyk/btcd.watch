// API payload types mirroring the Go structs (see docs/ARCHITECTURE.md §4).
// All amounts are satoshis.

export interface TxBlock {
  height: number
  hash: string
  time: number
}

export interface TxPending {
  txsAhead: number
  vbytesAhead: number
  etaBlocks: number
  etaSeconds: number
  /** Share of mempool vbytes paying a higher rate — position on the queue bar. */
  queueVbytesFraction: number
}

/** Script-type classification; `in` is empty for coinbases. */
export interface TxType {
  code: string
  in: string
  out: string
}

export interface Tx {
  txid: string
  status: 'pending' | 'confirmed'
  amountSats: number
  fiatUsd: number | null
  from: string[]
  to: string[]
  isCoinbase: boolean
  confirmations: number
  block: TxBlock | null
  feeSats: number | null
  feeRateSatPerVb: number | null
  vsize: number
  firstSeen: number
  /** Null when neither side classifies (non-standard scripts). */
  type: TxType | null
  /** BIP-125 replaceability signaled. */
  rbf: boolean
  pending: TxPending | null
}

export interface AddressActivity {
  txid: string
  direction: 'received' | 'sent' | 'self'
  amountSats: number
  status: 'pending' | 'confirmed'
  confirmations: number
  time: number
}

export interface AddressSummary {
  address: string
  /** Script-type code (P2WPKH, P2TR, …); empty hides the type chips. */
  type: string
  balanceSats: number
  receivedSats: number
  sentSats: number
  fiatUsd: number | null
  txCount: number
  approximate: boolean
  activity: AddressActivity[]
  offset: number
  limit: number
  hasMore: boolean
}

export interface BlockTx {
  txid: string
  amountSats: number
  feeRateSatPerVb: number | null
  isCoinbase: boolean
}

export interface Block {
  height: number
  hash: string
  time: number
  confirmations: number
  txCount: number
  avgFeeSatPerVb: number
  sizeBytes: number
  nextHeight: number | null
  txs: BlockTx[]
  offset: number
  limit: number
  hasMore: boolean
}

export type SearchResult =
  | { kind: 'tx'; tx: Tx }
  | { kind: 'address'; address: AddressSummary }
  | { kind: 'block'; block: Block }
  | { kind: 'notfound'; query: string }
  | { kind: 'invalid'; query: string }

export interface FeeTier {
  id: 'slow' | 'standard' | 'urgent'
  satPerVb: number
  etaBlocks: number
  label: string
}

export interface FeeEstimate {
  tiers: FeeTier[]
  source: 'mempool' | 'floor'
}

export interface QueueBand {
  minSatPerVb: number
  /** 0 on the open-ended front band. */
  maxSatPerVb: number
  vbytes: number
}

export interface Queue {
  txCount: number
  totalVbytes: number
  /** Rolling recent-peak depth — the capacity track (≥ totalVbytes). */
  peakVbytes: number
  /** Front of the line (highest fee) first. */
  bands: QueueBand[]
  /** Next-block cutoff position along the vbytes-proportional bar (0..1]. */
  cutoffFraction: number
  /** Lowest feerate still inside the cutoff. */
  nextBlockRate: number
}

/** One recently accepted transaction in the landing feed. AmountSats is
 * the gross output total (no prevouts in the notification). */
export interface Arrival {
  txid: string
  amountSats: number
  feeRateSatPerVb: number
  vsize: number
  time: number
}

export interface MempoolUpdate {
  queue: Queue
  arrivals: Arrival[]
}

export interface BlockFlash {
  height: number
  txCount: number
}

export interface Stats {
  network: string
  blockHeight: number
  mempool: { txCount: number; bytes: number }
  queue: Queue | null
  nextBlockEtaSeconds: number
  avgBlockIntervalSeconds: number
  halving: { blocksRemaining: number; etaSeconds: number }
  price: { usd: number; source: string; updatedAt: number } | null
}
