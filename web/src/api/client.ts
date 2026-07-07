import type {
  AddressSummary,
  Block,
  FeeEstimate,
  SearchResult,
  Stats,
  Tx,
} from './types'

// ApiError carries the backend's error envelope so views can distinguish
// a down node (503) from an unknown tx (404).
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message)
  }
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(path, { headers: { Accept: 'application/json' } })
  if (!res.ok) {
    let code = 'http_error'
    let message = `request failed (${res.status})`
    try {
      const body = await res.json()
      code = body?.error?.code ?? code
      message = body?.error?.message ?? message
    } catch {
      // Non-JSON error body; keep defaults.
    }
    throw new ApiError(res.status, code, message)
  }
  return res.json() as Promise<T>
}

export const api = {
  search: (q: string) =>
    get<SearchResult>(`/api/search?q=${encodeURIComponent(q)}`),
  tx: (txid: string) => get<Tx>(`/api/tx/${encodeURIComponent(txid)}`),
  address: (addr: string, offset = 0, limit = 25) =>
    get<AddressSummary>(
      `/api/address/${encodeURIComponent(addr)}?offset=${offset}&limit=${limit}`,
    ),
  block: (ref: string | number, offset = 0, limit = 25) =>
    get<Block>(
      `/api/block/${encodeURIComponent(ref)}?offset=${offset}&limit=${limit}`,
    ),
  fees: () => get<FeeEstimate>('/api/fees'),
  stats: () => get<Stats>('/api/stats'),
}
