/**
 * Round-7 queue-bar motion: the three effects that teach the queue
 * mechanics. The drift loop says "the line advances toward the miners",
 * the particle stream says "new transactions join at the back", and the
 * detach/land pair says "a mined block removes the front chunk". All
 * pure CSS animations on keyed nodes — no timers to clean up.
 */

/** Inflow that reads as "full" (5 parallel particles): ~10 tx/s
 * sustained mainnet traffic. The square root keeps the quiet end
 * visible — 60 tx/min still shows 2–3 particles, not a flat line. */
const INFLOW_FULL_TX_PER_MIN = 600

/** Map the server's raw acceptance rate onto the design's 0..1 traffic
 * level. */
export function trafficLevel(inflowTxPerMin: number): number {
  if (inflowTxPerMin <= 0) return 0
  return Math.min(1, Math.sqrt(inflowTxPerMin / INFLOW_FULL_TX_PER_MIN))
}

/** Three vertical lanes so parallel particles don't overlap. */
const LANES = [-4, -13, 5]

/** Faint diagonal stripes drifting toward the front of the line, inside
 * the filled bar ('ambient' motion only). */
export function DriftOverlay() {
  return <div className="bp-queue-drift" />
}

/**
 * One burst of particles gliding into the tail of the bar. Keyed per
 * mempool tick so each push replays the burst; fill-mode `both` parks
 * finished particles at opacity 0 until the next tick re-keys them.
 */
export function JoinParticles({
  tick,
  traffic,
  barPct,
}: {
  tick: number
  traffic: number
  barPct: number
}) {
  const count = 1 + Math.round(traffic * 4)
  const duration = (1.5 - traffic * 0.5).toFixed(2)
  return (
    <>
      {Array.from({ length: count }, (_, i) => (
        <div
          key={`join-${tick}-${i}`}
          className="bp-queue-join"
          style={{
            left: `calc(${barPct.toFixed(1)}% + 4px)`,
            marginTop: LANES[i % LANES.length],
            animationDuration: `${duration}s`,
            animationDelay: `${(i * 0.13).toFixed(2)}s`,
          }}
        />
      ))}
    </>
  )
}

/**
 * The mined-block moment: the next-block chunk slides off the front of
 * the track and pops in as a block beside it. Keyed by height (and
 * `forwards`), so each block plays exactly once while the flash shows.
 */
export function DetachFx({
  height,
  cutoffPct,
}: {
  height: number
  cutoffPct: number
}) {
  return (
    <>
      <div
        key={`chunk-${height}`}
        className="bp-queue-detach"
        style={{ width: `${cutoffPct.toFixed(1)}%` }}
      />
      <div key={`land-${height}`} className="bp-queue-land" />
    </>
  )
}
