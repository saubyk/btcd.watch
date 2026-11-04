import { useCallback, useEffect, useReducer, useState } from 'react'

import { FeeSlideOver } from './components/FeeSlideOver'
import { Footer } from './components/Footer'
import { OpenSourceCta } from './components/OpenSourceCta'
import { Header } from './components/Header'
import { ToastProvider } from './components/Toast'
import { useNetworkData } from './hooks/useNetworkData'
import { useSearch } from './hooks/useSearch'
import { initHistory, markKind, pushQuery, syncFromLocation } from './lib/nav'
import { initialState, reducer } from './state'
import { AddressView } from './views/AddressView'
import { BlockView } from './views/BlockView'
import { ConfirmedTx } from './views/ConfirmedTx'
import { Landing } from './views/Landing'
import { LoadingView } from './views/LoadingView'
import { NotFound } from './views/NotFound'
import { PendingTx } from './views/PendingTx'

export function App() {
  const [state, dispatch] = useReducer(reducer, initialState)
  const [feeOpen, setFeeOpen] = useState(false)
  const data = useNetworkData()
  const search = useSearch(dispatch)

  const goHome = useCallback(() => {
    dispatch({ type: 'reset' })
    pushQuery(null)
    markKind('home')
    window.scrollTo({ top: 0 })
  }, [])

  // Deep links: /?q=<txid|address|height> searches on cold load, on the
  // entry the page was loaded on (no push). Back/Forward re-show the
  // entry the browser moved to, again without pushing.
  useEffect(() => {
    const entry = initHistory()
    if (entry.q) void search(entry.q, { restore: entry })
    else markKind('home')

    const onPop = () => {
      const e = syncFromLocation()
      if (e.q) {
        void search(e.q, { restore: e })
      } else {
        dispatch({ type: 'reset' })
        markKind('home')
        requestAnimationFrame(() => window.scrollTo({ top: e.scrollY }))
      }
    }
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
    // search is stable (useCallback over dispatch).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const network = data.stats?.network ?? null
  // During initial block download the mempool is empty, so fee numbers
  // are just the configured floors — hide the ticker rather than mislead.
  const syncing = data.stats?.syncing ?? false
  const fees = syncing ? null : data.fees

  const view = (() => {
    switch (state.view) {
      case 'loading':
        return <LoadingView />
      case 'notfound':
        return <NotFound network={network} onHome={goHome} />
      case 'error':
        return (
          <NotFound
            network={network}
            errorMessage={state.errorMessage}
            onHome={goHome}
          />
        )
      case 'confirmed':
        return state.tx ? (
          <ConfirmedTx
            tx={state.tx}
            detail={state.detail}
            justConfirmed={state.justConfirmed}
            onSetDetail={(detail) => dispatch({ type: 'set-detail', detail })}
            onSearch={search}
            onHome={goHome}
          />
        ) : null
      case 'pending':
        return state.tx ? (
          <PendingTx
            tx={state.tx}
            fees={fees}
            stats={data.stats}
            watching={state.watching}
            dispatch={dispatch}
            onSearch={search}
            onHome={goHome}
          />
        ) : null
      case 'address':
        return state.address ? (
          <AddressView
            summary={state.address}
            dispatch={dispatch}
            onSearch={search}
            onHome={goHome}
          />
        ) : null
      case 'block':
        return state.block ? (
          <BlockView
            block={state.block}
            stats={data.stats}
            dispatch={dispatch}
            onSearch={search}
            onHome={goHome}
          />
        ) : null
      default:
        return <Landing data={data} onSearch={search} />
    }
  })()

  return (
    <ToastProvider>
      <div className="bp-app">
        <Header
          fees={fees}
          onHome={goHome}
          onOpenFees={() => setFeeOpen(true)}
        />
        {view}
        <OpenSourceCta />
        <Footer network={network} />
        <FeeSlideOver
          fees={fees}
          open={feeOpen}
          onClose={() => setFeeOpen(false)}
        />
      </div>
    </ToastProvider>
  )
}
