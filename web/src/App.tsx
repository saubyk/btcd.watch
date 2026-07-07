import { useCallback, useEffect, useReducer, useState } from 'react'

import { FeeSlideOver } from './components/FeeSlideOver'
import { Footer } from './components/Footer'
import { Header } from './components/Header'
import { ToastProvider } from './components/Toast'
import { useNetworkData } from './hooks/useNetworkData'
import { useSearch } from './hooks/useSearch'
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
    history.replaceState(null, '', location.pathname)
    window.scrollTo({ top: 0 })
  }, [])

  // Deep links: /?q=<txid|address|height> searches on cold load.
  useEffect(() => {
    const q = new URLSearchParams(location.search).get('q')
    if (q) void search(q)
    // search is stable (useCallback over dispatch).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const network = data.stats?.network ?? null

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
            fees={data.fees}
            stats={data.stats}
            watching={state.watching}
            dispatch={dispatch}
            onHome={goHome}
          />
        ) : null
      case 'address':
        return state.address ? (
          <AddressView
            summary={state.address}
            dispatch={dispatch}
            onHome={goHome}
          />
        ) : null
      case 'block':
        return state.block ? (
          <BlockView
            block={state.block}
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
          fees={data.fees}
          onHome={goHome}
          onOpenFees={() => setFeeOpen(true)}
        />
        {view}
        <Footer network={network} />
        <FeeSlideOver
          fees={data.fees}
          open={feeOpen}
          onClose={() => setFeeOpen(false)}
        />
      </div>
    </ToastProvider>
  )
}
