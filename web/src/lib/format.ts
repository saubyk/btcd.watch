// Display formatters. Kept pure so they are trivially unit-testable.

const SATS_PER_BTC = 100_000_000

/** 4250000 sats → "0.0425"; trims below 8 decimals but keeps at least 4. */
export function formatBtc(sats: number): string {
  const btc = sats / SATS_PER_BTC
  let out = btc.toFixed(8)
  while (out.length > 0 && out.endsWith('0') && !out.endsWith('.0000')) {
    const candidate = out.slice(0, -1)
    if (candidate.split('.')[1]!.length < 4) break
    out = candidate
  }
  return out
}

/** 4165.2 → "$4,165"; sub-$100 values keep cents. */
export function formatFiat(usd: number): string {
  return usd.toLocaleString('en-US', {
    style: 'currency',
    currency: 'USD',
    maximumFractionDigits: usd >= 100 ? 0 : 2,
    minimumFractionDigits: usd >= 100 ? 0 : 2,
  })
}

export function formatNumber(n: number): string {
  return n.toLocaleString('en-US')
}

/** 14231 → "14.2k"; small numbers pass through. */
export function formatCompact(n: number): string {
  if (n < 1000) return String(n)
  if (n < 1_000_000) return `${(n / 1000).toFixed(1).replace(/\.0$/, '')}k`
  return `${(n / 1_000_000).toFixed(1).replace(/\.0$/, '')}M`
}

/** "bc1qar0srrr7xfkvy..." → "bc1qar…5mdq" (head 6, tail 4). */
export function truncateMiddle(s: string, head = 6, tail = 4): string {
  if (s.length <= head + tail + 1) return s
  return `${s.slice(0, head)}…${s.slice(-tail)}`
}

/** Unix seconds → "3 days ago" / "Just now". */
export function formatRelative(unixSecs: number, now = Date.now()): string {
  const diff = Math.max(0, Math.floor(now / 1000) - unixSecs)
  if (diff < 60) return 'Just now'
  if (diff < 3600) {
    const m = Math.floor(diff / 60)
    return `${m} ${m === 1 ? 'minute' : 'minutes'} ago`
  }
  if (diff < 86400) {
    const h = Math.floor(diff / 3600)
    return `${h} ${h === 1 ? 'hour' : 'hours'} ago`
  }
  const d = Math.floor(diff / 86400)
  return `${d} ${d === 1 ? 'day' : 'days'} ago`
}

/** Seconds → "~45 minutes" / "~2 hours" / "~622 days". */
export function formatEta(seconds: number): string {
  if (seconds < 90) return '~1 minute'
  if (seconds < 90 * 60) {
    return `~${Math.round(seconds / 60)} minutes`
  }
  if (seconds < 48 * 3600) {
    const h = Math.round(seconds / 3600)
    return `~${h} ${h === 1 ? 'hour' : 'hours'}`
  }
  return `~${Math.round(seconds / 86400)} days`
}

/** Short variant for the live pill: "~8 min". */
export function formatEtaShort(seconds: number): string {
  if (seconds < 90) return '~1 min'
  if (seconds < 90 * 60) return `~${Math.round(seconds / 60)} min`
  return `~${Math.round(seconds / 3600)} hr`
}

/** Unix seconds → "Jun 18, 2026 · 2:14 PM". */
export function formatTimestamp(unixSecs: number): string {
  const d = new Date(unixSecs * 1000)
  const date = d.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
  const time = d.toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: '2-digit',
  })
  return `${date} · ${time}`
}
