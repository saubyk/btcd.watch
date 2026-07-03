import { useCallback, useReducer } from 'react'

import { Footer } from './components/Footer'
import { Header } from './components/Header'
import { ToastProvider } from './components/Toast'
import { useNetworkData } from './hooks/useNetworkData'
import { useSearch } from './hooks/useSearch'
import { initialState, reducer } from './state'
import { AddressView } from './views/AddressView'
import { ConfirmedTx } from './views/ConfirmedTx'
import { Landing } from './views/Landing'
import { LoadingView } from './views/LoadingView'
import { NotFound } from './views/NotFound'
import { PendingTx } from './views/PendingTx'

export function App() {
  const [state, dispatch] = useReducer(reducer, initialState)
  const data = useNetworkData()
  const search = useSearch(dispatch)

  const goHome = useCallback(() => {
    dispatch({ type: 'reset' })
    window.scrollTo({ top: 0 })
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
            onHome={goHome}
          />
        ) : null
      case 'pending':
        return state.tx ? (
          <PendingTx
            tx={state.tx}
            fees={data.fees}
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
      default:
        return <Landing data={data} onSearch={search} />
    }
  })()

  return (
    <ToastProvider>
      <div className="bp-app">
        <Header onHome={goHome} />
        {view}
        <Footer network={network} />
      </div>
    </ToastProvider>
  )
}
