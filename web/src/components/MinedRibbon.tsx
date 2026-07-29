import { useRef } from 'react'

import type { RecentBlock } from '../api/types'
import { appConfig } from '../appConfig'
import { useNow } from '../hooks/useNow'
import { formatBlockAge, formatNumber } from '../lib/format'

/** Tiles shown at rest — the ribbon adds one more that squeezes out. */
const LIVE_TILES = 6
const LIVE_TILES_NARROW = 3

/**
 * Round-8 "blocks already mined" ribbon: where the queue goes. Newest is
 * on the LEFT, matching the queue bar above it — a block leaves the
 * front of the line and lands directly under where it left.
 *
 * The tile count is `live + 1`: the extra one is the oldest block on its
 * way out, an in-flow tile squeezing itself to nothing (never an
 * absolutely-positioned ghost, so it can't overlap a live tile and the
 * row closes the gap behind it). It only appears once a block has been
 * mined in this session, so the first paint shows exactly `live` tiles.
 */
export function MinedRibbon({
  blocks,
  /** Height of the block being announced right now, or null between
   * blocks. Drives the whole animation. */
  flyingHeight,
  narrow,
  motionOn,
  onSearch,
}: {
  blocks: RecentBlock[]
  flyingHeight: number | null
  narrow: boolean
  motionOn: boolean
  onSearch: (q: string) => void
}) {
  const now = useNow(appConfig.minedAgoRefreshSeconds * 1000)
  const live = narrow ? LIVE_TILES_NARROW : LIVE_TILES

  // The departing tile holds the end of its squeeze animation at rest,
  // so it may only join the row once a block has actually been mined
  // here — otherwise the first paint would play a tile out for nothing.
  const sawBlock = useRef(false)
  if (flyingHeight !== null) sawBlock.current = true
  const departing = motionOn && sawBlock.current && blocks.length > live

  const tiles = blocks.slice(0, live + (departing ? 1 : 0))
  if (tiles.length === 0) return null

  return (
    <div className="bp-ribbon">
      <div className="bp-ribbon-head">
        <span className="bp-ribbon-title">Blocks already mined</span>
        <span className="bp-ribbon-hint">tap one to open it</span>
      </div>

      <div className="bp-ribbon-row">
        {tiles.map((block, i) => (
          <button
            key={block.height}
            className={tileClass({
              newest: i === 0,
              // Pops in exactly as the flying block lands.
              popping: i === 0 && block.height === flyingHeight,
              leaving: departing && i === live,
            })}
            onClick={() => onSearch(String(block.height))}
            title={`Open block ${formatNumber(block.height)}`}
          >
            <span className="bp-ribbon-height">
              {narrow
                ? `…${String(block.height).slice(-3)}`
                : formatNumber(block.height)}
            </span>
            <span className="bp-ribbon-age">
              {formatBlockAge(block.time, now)}
            </span>
            <span className="bp-ribbon-tx">
              {formatNumber(block.txCount)} tx
            </span>
          </button>
        ))}
      </div>

      <div className="bp-ribbon-captions">
        <span>↑ just left the front of the line</span>
        {/* Resolves the reversed reading direction — keep it. */}
        <span>older →</span>
      </div>

      {/* The block itself, arcing out of the front of the queue bar and
          landing on the new tile. Suppressed on phones: the drop is too
          tall and the wrapped legend makes it read badly. */}
      {flyingHeight !== null && !narrow && (
        <div key={`drop-${flyingHeight}`} className="bp-ribbon-drop">
          {String(flyingHeight).slice(-3)}
        </div>
      )}
    </div>
  )
}

function tileClass(state: {
  newest: boolean
  popping: boolean
  leaving: boolean
}): string {
  const classes = ['bp-ribbon-tile']
  if (state.newest) classes.push('bp-ribbon-tile--newest')
  if (state.popping) classes.push('bp-ribbon-tile--pop')
  if (state.leaving) classes.push('bp-ribbon-tile--leaving')
  return classes.join(' ')
}
