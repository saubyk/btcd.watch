import type { Stats } from './types'

// TxUpdate is the compact per-transaction push from the server.
export interface TxUpdate {
  status: 'pending' | 'confirmed'
  confirmations: number
  blockHeight: number | null
  txsAhead: number | null
  etaSeconds: number | null
  queueVbytesFraction: number | null
}

type ServerMessage =
  | { type: 'stats'; data: Stats }
  | { type: 'tx'; txid: string; data: TxUpdate }

const INITIAL_BACKOFF_MS = 1000
const MAX_BACKOFF_MS = 30_000

/**
 * Singleton WebSocket to /api/ws. Reconnects with exponential backoff and
 * replays every active watch subscription on reopen, so a dropped
 * connection never silently loses a watch.
 */
class LiveSocket {
  private ws: WebSocket | null = null
  private backoff = INITIAL_BACKOFF_MS
  private reconnectTimer: ReturnType<typeof setTimeout> | undefined
  private open = false

  private statsSubs = new Set<(s: Stats) => void>()
  private connSubs = new Set<(open: boolean) => void>()
  private watches = new Map<string, Set<(u: TxUpdate) => void>>()

  get isOpen(): boolean {
    return this.open
  }

  /** Subscribe to live stats pushes. Returns an unsubscribe function. */
  onStats(cb: (s: Stats) => void): () => void {
    this.statsSubs.add(cb)
    this.ensure()
    return () => this.statsSubs.delete(cb)
  }

  /** Subscribe to connection state changes. */
  onConnection(cb: (open: boolean) => void): () => void {
    this.connSubs.add(cb)
    this.ensure()
    return () => this.connSubs.delete(cb)
  }

  /** Watch a txid for live updates. Returns an unwatch function. */
  watch(txid: string, cb: (u: TxUpdate) => void): () => void {
    let subs = this.watches.get(txid)
    if (!subs) {
      subs = new Set()
      this.watches.set(txid, subs)
      this.send({ type: 'watch', txid })
    }
    subs.add(cb)
    this.ensure()

    return () => {
      const set = this.watches.get(txid)
      if (!set) return
      set.delete(cb)
      if (set.size === 0) {
        this.watches.delete(txid)
        this.send({ type: 'unwatch', txid })
      }
    }
  }

  private ensure() {
    if (this.ws || this.reconnectTimer) return
    this.connect()
  }

  private connect() {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    const ws = new WebSocket(`${proto}://${location.host}/api/ws`)
    this.ws = ws

    ws.onopen = () => {
      this.open = true
      this.backoff = INITIAL_BACKOFF_MS
      this.connSubs.forEach((cb) => cb(true))
      // Replay active watches lost with the previous connection.
      for (const txid of this.watches.keys()) {
        ws.send(JSON.stringify({ type: 'watch', txid }))
      }
    }

    ws.onmessage = (ev) => {
      let msg: ServerMessage
      try {
        msg = JSON.parse(ev.data)
      } catch {
        return
      }
      if (msg.type === 'stats') {
        this.statsSubs.forEach((cb) => cb(msg.data))
      } else if (msg.type === 'tx') {
        this.watches.get(msg.txid)?.forEach((cb) => cb(msg.data))
      }
    }

    ws.onclose = () => {
      this.ws = null
      if (this.open) {
        this.open = false
        this.connSubs.forEach((cb) => cb(false))
      }
      this.reconnectTimer = setTimeout(() => {
        this.reconnectTimer = undefined
        this.connect()
      }, this.backoff)
      this.backoff = Math.min(this.backoff * 2, MAX_BACKOFF_MS)
    }

    ws.onerror = () => ws.close()
  }

  private send(payload: { type: 'watch' | 'unwatch'; txid: string }) {
    if (this.open && this.ws) {
      this.ws.send(JSON.stringify(payload))
    }
    // Not open: watch() state is replayed in onopen; unwatch for a dead
    // connection is a no-op server-side anyway.
  }
}

export const live = new LiveSocket()
