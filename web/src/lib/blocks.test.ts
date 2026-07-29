import { describe, expect, it } from 'vitest'

import type { RecentBlock } from '../api/types'
import { mergeRecentBlocks, RECENT_BLOCKS_MAX } from './blocks'

const block = (height: number, txCount = 1): RecentBlock => ({
  height,
  txCount,
  time: 1_700_000_000 + height * 600,
})

describe('mergeRecentBlocks', () => {
  it('orders newest first', () => {
    const merged = mergeRecentBlocks([block(9)], [block(7), block(8)])
    expect(merged.map((b) => b.height)).toEqual([9, 8, 7])
  })

  it('keeps a pushed block the server has not caught up to yet', () => {
    const pushed = block(101, 2940)
    const merged = mergeRecentBlocks([pushed], [block(100), block(99)])
    expect(merged[0]).toEqual(pushed)
  })

  it('lets the preferred side correct a height both describe', () => {
    const merged = mergeRecentBlocks([block(5, 42)], [block(5, 7)])
    expect(merged).toEqual([block(5, 42)])
  })

  it('drops the orphan when a reorg replaces a height', () => {
    const orphan = block(12, 1)
    const replacement = block(12, 900)
    const merged = mergeRecentBlocks([replacement], [orphan, block(11)])
    expect(merged.map((b) => b.txCount)).toEqual([900, 1])
  })

  it('caps the window', () => {
    const many = Array.from({ length: 25 }, (_, i) => block(i))
    const merged = mergeRecentBlocks([], many)
    expect(merged).toHaveLength(RECENT_BLOCKS_MAX)
    expect(merged[0]!.height).toBe(24)
  })
})
