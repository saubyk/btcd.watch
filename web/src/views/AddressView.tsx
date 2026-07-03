import { CopyIcon } from '../components/Icons'
import { BackButton } from '../components/ResultParts'
import { useCopy } from '../components/Toast'

/**
 * Minimal address card until the address milestone lands the full
 * balance/activity endpoint. Shows the recognized address so search
 * routing already works end to end.
 */
export function AddressView({
  address,
  onHome,
}: {
  address: string
  onHome: () => void
}) {
  const copy = useCopy()

  return (
    <main className="bp-view bp-result">
      <BackButton onClick={onHome} />

      <div className="bp-card">
        <div className="bp-card-body">
          <div className="bp-address-label">◆ Wallet address</div>
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 10 }}>
            <div className="bp-address-value" style={{ flex: 1, minWidth: 0 }}>
              {address}
            </div>
            <button className="bp-copy-btn" onClick={() => copy(address)}>
              <CopyIcon />
              Copy
            </button>
          </div>

          <div className="bp-explain" style={{ marginTop: 20 }}>
            <div className="bp-explain-title">
              Balance &amp; activity are coming 🏗️
            </div>
            <p>
              This is a valid address on your node's network. The full
              address view — balance, totals, and recent activity — arrives
              in the next milestone.
            </p>
          </div>
        </div>
      </div>
    </main>
  )
}
