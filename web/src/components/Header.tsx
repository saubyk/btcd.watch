export function Header({ onHome }: { onHome: () => void }) {
  return (
    <header className="bp-header">
      <div className="bp-header-inner">
        <button className="bp-logo-btn" onClick={onHome} aria-label="Home">
          <span className="bp-logo-mark">₿</span>
          <span className="bp-wordmark">
            btcdwatch<span className="bp-wordmark-tld">.com</span>
          </span>
        </button>
        <span className="bp-tagline">Bitcoin transaction tracker</span>
      </div>
    </header>
  )
}
