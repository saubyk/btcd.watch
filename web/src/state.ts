import type { AddressSummary, Block, SearchResult, Tx } from './api/types'
import { appConfig } from './appConfig'

export type View =
  | 'landing'
  | 'loading'
  | 'notfound'
  | 'error'
  | 'confirmed'
  | 'pending'
  | 'address'
  | 'block'

export interface AppState {
  view: View
  query: string
  tx: Tx | null
  address: AddressSummary | null
  block: Block | null
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
  block: null,
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
  | {
      type: 'tx-queue'
      txsAhead: number
      etaSeconds: number
      queueVbytesFraction: number | null
    }
  | { type: 'address-more'; page: AddressSummary }
  | { type: 'block-more'; page: Block }
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
        case 'block':
          return { ...state, view: 'block', block: r.block }
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

    // Live queue-position refresh for the pending view.
    case 'tx-queue': {
      if (!state.tx?.pending) return state
      return {
        ...state,
        tx: {
          ...state.tx,
          pending: {
            ...state.tx.pending,
            txsAhead: action.txsAhead,
            etaSeconds: action.etaSeconds,
            queueVbytesFraction:
              action.queueVbytesFraction ??
              state.tx.pending.queueVbytesFraction,
          },
        },
      }
    }

    // Pagination: append the next activity page, adopt its cursor. A
    // stale in-flight page for a previously viewed address is dropped.
    case 'address-more': {
      if (!state.address || action.page.address !== state.address.address) {
        return state
      }
      return {
        ...state,
        address: {
          ...action.page,
          activity: [...state.address.activity, ...action.page.activity],
        },
      }
    }

    // Pagination: append the next page of block transactions. A stale
    // in-flight page for a previously viewed block is dropped.
    case 'block-more': {
      if (!state.block || action.page.hash !== state.block.hash) {
        return state
      }
      return {
        ...state,
        block: {
          ...action.page,
          txs: [...state.block.txs, ...action.page.txs],
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
