// Design-level options from the handoff, fixed as build-time constants.
export const appConfig = {
  showMempool: true,
  /** Live mempool pushes (queue + arrivals feed); off = static queue from
   * the 10s stats pushes only (useful for tests/screenshots). */
  liveMempool: true,
  showStats: true,
  /** Seconds the "block mined" flash stays visible. */
  minedFlashSeconds: 6,
  defaultDetail: 'beginner' as 'beginner' | 'detailed',
  /** Seconds between REST watch polls on a pending tx. */
  watchPollSeconds: 15,
  /** Seconds between stats refreshes on the landing page. */
  statsRefreshSeconds: 30,
  btcdUrl: 'https://github.com/btcsuite/btcd',
  /** This app's own public repo (round 6 open-source CTA + footer). */
  repoUrl: 'https://github.com/saubyk/btcd.watch',
  issuesUrl: 'https://github.com/saubyk/btcd.watch/issues',
}

/** Example address prefix per network, for the not-found reference card. */
export function addressHint(network: string): { prefix: string; sample: string } {
  switch (network) {
    case 'regtest':
      return { prefix: 'bcrt1', sample: 'bcrt1qar0sr…wf5mdq' }
    case 'testnet3':
    case 'signet':
      return { prefix: 'tb1', sample: 'tb1qar0sr…wf5mdq' }
    case 'simnet':
      return { prefix: 'sb1', sample: 'sb1qar0sr…wf5mdq' }
    default:
      return { prefix: 'bc1, 1, or 3', sample: 'bc1qar0sr…wf5mdq' }
  }
}
