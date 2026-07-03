import type { AddressSummary, SearchResult, Tx } from './api/types'
import { appConfig } from './appConfig'

export type View =
  | 'landing'
  | 'loading'
  | 'notfound'
  | 'error'
  | 'confirmed'
  | 'pending'
  | 'address'

export interface AppState {
  view: View
  query: string
  tx: Tx | null
  address: AddressSummary | null
  detail: 'beginner' | 'detailed'
  justConfirmed: boolean
  watching: boolean
  errorMessage: string | null
}

export const initialState: AppState = {
  view: 'landing',
  query: '',
  tx: null,
  address: null,
  detail: appConfig.defaultDetail,
  justConfirmed: false,
  watching: false,
  errorMessage: null,
}

export type Action =
  | { type: 'search-start'; query: string }
  | { type: 'search-result'; result: SearchResult }
  | { type: 'search-error'; message: string }
  | { type: 'tx-updated'; tx: Tx }
  | { type: 'address-more'; page: AddressSummary }
  | { type: 'set-detail'; detail: 'beginner' | 'detailed' }
  | { type: 'watch-start' }
  | { type: 'watch-stop' }
  | { type: 'reset' }

export function reducer(state: AppState, action: Action): AppState {
  switch (action.type) {
    case 'search-start':
      return {
        ...state,
        view: 'loading',
        query: action.query,
        justConfirmed: false,
        watching: false,
        errorMessage: null,
      }

    case 'search-result': {
      const r = action.result
      switch (r.kind) {
        case 'tx':
          return {
            ...state,
            view: r.tx.status === 'confirmed' ? 'confirmed' : 'pending',
            tx: r.tx,
          }
        case 'address':
          return { ...state, view: 'address', address: r.address }
        default:
          return { ...state, view: 'notfound' }
      }
    }

    case 'search-error':
      return { ...state, view: 'error', errorMessage: action.message }

    // Watch polling: a pending tx either refreshed its queue position or
    // just landed in a block.
    case 'tx-updated': {
      if (action.tx.status === 'confirmed') {
        return {
          ...state,
          tx: action.tx,
          view: 'confirmed',
          justConfirmed: true,
          watching: false,
        }
      }
      return { ...state, tx: action.tx }
    }

    // Pagination: append the next activity page, adopt its cursor.
    case 'address-more': {
      if (!state.address) return state
      return {
        ...state,
        address: {
          ...action.page,
          activity: [...state.address.activity, ...action.page.activity],
        },
      }
    }

    case 'set-detail':
      return { ...state, detail: action.detail }
    case 'watch-start':
      return { ...state, watching: true }
    case 'watch-stop':
      return { ...state, watching: false }

    case 'reset':
      return {
        ...initialState,
        detail: state.detail,
      }
  }
}
