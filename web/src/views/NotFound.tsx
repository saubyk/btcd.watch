import { addressHint } from '../appConfig'
import { SearchSlashIcon } from '../components/Icons'

export function NotFound({
  network,
  errorMessage,
  onHome,
}: {
  network: string | null
  /** Set when the failure was an API error rather than a bad query. */
  errorMessage?: string | null
  onHome: () => void
}) {
  const hint = addressHint(network ?? 'mainnet')

  return (
    <main className="bp-view bp-notfound">
      <div className="bp-notfound-icon">
        <SearchSlashIcon />
      </div>

      {errorMessage ? (
        <>
          <h2>Something went wrong</h2>
          <p className="bp-notfound-copy">
            We couldn't reach the node to answer that ({errorMessage}). Give
            it a moment and try again.
          </p>
        </>
      ) : (
        <>
          <h2>We couldn't find that</h2>
          <p className="bp-notfound-copy">
            That doesn't look like a Bitcoin transaction ID or address.
            Mistyped it? Here's what each one looks like:
          </p>
          <div className="bp-ref-card">
            <div className="bp-ref-row">
              <div className="bp-ref-title">Transaction ID</div>
              <div className="bp-ref-sample">
                64 letters &amp; numbers — e.g. f4e2a7c9…b2f6a4e
              </div>
            </div>
            <div className="bp-ref-row">
              <div className="bp-ref-title">Wallet address</div>
              <div className="bp-ref-sample">
                starts with {hint.prefix} — e.g. {hint.sample}
              </div>
            </div>
          </div>
        </>
      )}

      <button className="bp-btn-cta" onClick={onHome}>
        ← Try another search
      </button>
    </main>
  )
}
