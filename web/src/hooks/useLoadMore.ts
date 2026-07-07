import { useState } from 'react'

/**
 * Shared "Load more" wiring: runs the fetch, exposes the loading flag, and
 * swallows errors so the button stays for a retry.
 */
export function useLoadMore(fetchNext: () => Promise<void>) {
  const [loading, setLoading] = useState(false)

  const loadMore = async () => {
    setLoading(true)
    try {
      await fetchNext()
    } catch {
      // Leave the list as-is; the control stays for a retry.
    } finally {
      setLoading(false)
    }
  }

  return { loadMore, loading }
}
