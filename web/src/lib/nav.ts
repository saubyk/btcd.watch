import type { SearchResult } from '../api/types'

/**
 * Browser-history integration. The app has no router: each result the
 * user navigates to is one history entry at `?q=<query>` (the landing
 * page is the entry with no `q`), and the entry's state remembers what
 * it showed so the in-page back button can name where it goes.
 *
 * The decisions are pure functions (unit-tested); the few lines that
 * touch `window.history` are thin wrappers below them.
 */

/** What a history entry shows. */
export type PageKind = 'home' | 'tx' | 'address' | 'block'

export interface NavEntry {
  /** The query this entry shows; null is the landing page. */
  q: string | null
  /** What the entry showed, once its result arrived. */
  kind: PageKind | null
  /**
   * What the in-app entry before this one showed; null when there is
   * none (a cold load or a link from another site), so Back would leave.
   */
  prev: PageKind | null
  /** Scroll offset saved when the user navigated away from the entry. */
  scrollY: number
}

const KINDS: readonly PageKind[] = ['home', 'tx', 'address', 'block']

const isKind = (v: unknown): v is PageKind =>
  typeof v === 'string' && (KINDS as readonly string[]).includes(v)

/** The `q` a location search string carries, or null when absent/blank. */
export function queryFromSearch(search: string): string | null {
  const q = new URLSearchParams(search).get('q')?.trim()
  return q ? q : null
}

/**
 * The entry for the current location. A state this app wrote for the same
 * query is kept (a reload preserves it); anything else — no state, a
 * foreign shape, or a URL edited by hand — is a fresh entry with nothing
 * in-app behind it.
 */
export function readEntry(state: unknown, search: string): NavEntry {
  const q = queryFromSearch(search)
  if (state && typeof state === 'object') {
    const s = state as Record<string, unknown>
    const sq = typeof s.q === 'string' ? s.q : s.q === null ? null : undefined
    if (sq === q) {
      return {
        q,
        kind: isKind(s.kind) ? s.kind : null,
        prev: isKind(s.prev) ? s.prev : null,
        scrollY:
          typeof s.scrollY === 'number' && s.scrollY > 0 ? s.scrollY : 0,
      }
    }
  }
  return { q, kind: null, prev: null, scrollY: 0 }
}

/**
 * Navigating to `q` adds an entry unless the current one already shows
 * it, so re-running the same search doesn't stack duplicates.
 */
export function shouldPush(cur: NavEntry, q: string | null): boolean {
  return cur.q !== q
}

/** The entry pushed when navigating from `cur` to `q`. */
export function nextEntry(cur: NavEntry, q: string | null): NavEntry {
  return {
    q,
    kind: null,
    prev: cur.kind ?? (cur.q === null ? 'home' : null),
    scrollY: 0,
  }
}

/** The URL for an entry, relative to the app's path. */
export function urlFor(q: string | null, pathname: string): string {
  return q === null ? pathname : `?q=${encodeURIComponent(q)}`
}

/** What a search result shows; null for not-found/invalid. */
export function kindOfResult(result: SearchResult): PageKind | null {
  switch (result.kind) {
    case 'tx':
    case 'address':
    case 'block':
      return result.kind
    default:
      return null
  }
}

/**
 * The back button's label for an entry, or null when the in-page
 * button should only offer a new search (nothing in-app behind it, or
 * only the landing page — which "New search" already reaches).
 */
export function backLabel(entry: NavEntry): string | null {
  switch (entry.prev) {
    case 'tx':
      return 'Back to transaction'
    case 'address':
      return 'Back to address'
    case 'block':
      return 'Back to block'
    default:
      return null
  }
}

// --- window.history wrappers -------------------------------------------

let snapshot: NavEntry | null = null
const listeners = new Set<() => void>()

function write(entry: NavEntry, mode: 'push' | 'replace') {
  const url = urlFor(entry.q, location.pathname)
  if (mode === 'push') history.pushState(entry, '', url)
  else history.replaceState(entry, '', url)
  setSnapshot(entry)
}

function setSnapshot(entry: NavEntry) {
  snapshot = entry
  listeners.forEach((l) => l())
}

/** The current entry (stable between changes, for useSyncExternalStore). */
export function currentEntry(): NavEntry {
  snapshot ??= readEntry(history.state, location.search)
  return snapshot
}

export function subscribeEntry(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

/**
 * Normalizes the entry the app was loaded on and takes over scroll
 * restoration: results load after the first render, so the browser's
 * own restoration would land on an empty page.
 */
export function initHistory(): NavEntry {
  history.scrollRestoration = 'manual'
  const entry = readEntry(history.state, location.search)
  write(entry, 'replace')
  return entry
}

/**
 * Navigates to `q`: saves the outgoing entry's scroll offset, then pushes
 * the new entry. A no-op (returns false) when `q` is already current.
 */
export function pushQuery(q: string | null): boolean {
  const cur = currentEntry()
  if (!shouldPush(cur, q)) return false
  history.replaceState({ ...cur, scrollY: window.scrollY }, '')
  write(nextEntry(cur, q), 'push')
  return true
}

/** Records what the current entry shows once its result has arrived. */
export function markKind(kind: PageKind | null) {
  const cur = currentEntry()
  if (cur.kind === kind) return
  write({ ...cur, kind }, 'replace')
}

/** Re-reads the entry after the browser moved (popstate). */
export function syncFromLocation(): NavEntry {
  const entry = readEntry(history.state, location.search)
  setSnapshot(entry)
  return entry
}
