import { describe, expect, it } from 'vitest'

import {
  backLabel,
  kindOfResult,
  nextEntry,
  queryFromSearch,
  readEntry,
  shouldPush,
  urlFor,
  type NavEntry,
} from './nav'

const entry = (over: Partial<NavEntry> = {}): NavEntry => ({
  q: null,
  kind: null,
  prev: null,
  scrollY: 0,
  ...over,
})

describe('queryFromSearch', () => {
  it('reads and trims q', () => {
    expect(queryFromSearch('?q=%20abc%20')).toBe('abc')
  })
  it('treats a missing or blank q as the landing page', () => {
    expect(queryFromSearch('')).toBeNull()
    expect(queryFromSearch('?q=')).toBeNull()
    expect(queryFromSearch('?q=%20')).toBeNull()
  })
})

describe('readEntry', () => {
  it('keeps the state this app wrote for the same query (reload)', () => {
    const state = { q: 'abc', kind: 'tx', prev: 'address', scrollY: 420 }
    expect(readEntry(state, '?q=abc')).toEqual(state)
  })

  it('starts fresh, with nothing behind it, on a cold load', () => {
    expect(readEntry(null, '?q=abc')).toEqual(
      entry({ q: 'abc' }),
    )
  })

  it('discards state for a different query (URL edited by hand)', () => {
    const state = { q: 'old', kind: 'tx', prev: 'block', scrollY: 9 }
    expect(readEntry(state, '?q=new')).toEqual(entry({ q: 'new' }))
  })

  it('drops unknown kinds and bad scroll values from foreign state', () => {
    const state = { q: 'abc', kind: 'wallet', prev: 7, scrollY: -3 }
    expect(readEntry(state, '?q=abc')).toEqual(entry({ q: 'abc' }))
  })

  it('matches the landing page by a null q', () => {
    const state = { q: null, kind: 'home', prev: 'tx', scrollY: 0 }
    expect(readEntry(state, '')).toEqual(state)
  })
})

describe('shouldPush', () => {
  it('pushes when the query changes, including to and from home', () => {
    expect(shouldPush(entry({ q: 'a' }), 'b')).toBe(true)
    expect(shouldPush(entry({ q: null }), 'a')).toBe(true)
    expect(shouldPush(entry({ q: 'a' }), null)).toBe(true)
  })
  it('does not stack a duplicate of the current query', () => {
    expect(shouldPush(entry({ q: 'a' }), 'a')).toBe(false)
    expect(shouldPush(entry({ q: null }), null)).toBe(false)
  })
})

describe('nextEntry', () => {
  it('remembers what the outgoing entry showed', () => {
    const next = nextEntry(entry({ q: 'tx1', kind: 'tx', scrollY: 50 }), 'addr')
    expect(next).toEqual(entry({ q: 'addr', prev: 'tx' }))
  })
  it('treats the landing page as home before its kind is recorded', () => {
    expect(nextEntry(entry({ q: null }), 'tx1').prev).toBe('home')
  })
  it('has no back target when the outgoing result never resolved', () => {
    expect(nextEntry(entry({ q: 'loading' }), 'tx1').prev).toBeNull()
  })
})

describe('backLabel', () => {
  it('names the previous result', () => {
    expect(backLabel(entry({ prev: 'tx' }))).toBe('Back to transaction')
    expect(backLabel(entry({ prev: 'address' }))).toBe('Back to address')
    expect(backLabel(entry({ prev: 'block' }))).toBe('Back to block')
  })
  it('offers only a new search when home or nothing in-app is behind', () => {
    expect(backLabel(entry({ prev: 'home' }))).toBeNull()
    expect(backLabel(entry({ prev: null }))).toBeNull()
  })
})

describe('urlFor', () => {
  it('encodes the query, and uses the bare path for home', () => {
    expect(urlFor('a b', '/')).toBe('?q=a%20b')
    expect(urlFor(null, '/app/')).toBe('/app/')
  })
})

describe('kindOfResult', () => {
  it('maps found results and drops not-found/invalid', () => {
    expect(kindOfResult({ kind: 'notfound', query: 'x' })).toBeNull()
    expect(kindOfResult({ kind: 'invalid', query: 'x' })).toBeNull()
    expect(
      kindOfResult({ kind: 'block', block: {} as never }),
    ).toBe('block')
  })
})
