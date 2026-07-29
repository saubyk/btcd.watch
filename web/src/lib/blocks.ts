import type { RecentBlock } from '../api/types'

/** Ribbon depth kept on the client — matches the server's window. */
export const RECENT_BLOCKS_MAX = 10

/**
 * Merges two views of the recently mined blocks into one list, newest
 * first. `preferred` wins where both describe the same height.
 *
 * The ribbon has two sources: the stats payload (authoritative, but as
 * old as the last live-cache refresh) and the per-block push (instant,
 * one block at a time). Merging keeps the new tile arriving in time for
 * its animation while letting the server correct the window — including
 * after a reorg, where the replacement block simply overwrites the
 * height and the orphan falls off the end.
 */
export function mergeRecentBlocks(
  preferred: readonly RecentBlock[],
  fallback: readonly RecentBlock[],
  max = RECENT_BLOCKS_MAX,
): RecentBlock[] {
  const byHeight = new Map<number, RecentBlock>()
  for (const block of fallback) byHeight.set(block.height, block)
  for (const block of preferred) byHeight.set(block.height, block)
  return [...byHeight.values()]
    .sort((a, b) => b.height - a.height)
    .slice(0, max)
}
